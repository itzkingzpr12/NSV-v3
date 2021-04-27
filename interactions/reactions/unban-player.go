package reactions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gammazero/workerpool"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

type UnbanPlayerSuccess struct {
	Servers    []gcscmodels.Server
	PlayerName string
}

type UnbanPlayerError struct {
	Message    string
	Servers    []gcscmodels.Server
	PlayerName string
}

type UnbanSuccess struct {
	Server     gcscmodels.Server
	PlayerName string
}

type UnbanError struct {
	Server     gcscmodels.Server
	Message    string
	Error      string
	PlayerName string
}

// UnbanPlayer func
func (r *Reactions) UnbanPlayer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.UnbanReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.UnbanReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to unban player", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "unban reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to unban player", mra.ChannelID, Error{
			Message: "unban player message has expired",
			Err:     errors.New("please run the unban command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to unban player", mra.ChannelID, Error{
			Message: "no servers found to unban on",
			Err:     errors.New("please run the unban command again"),
		})
		return
	}

	guildFeed, gfErr := guildconfigservice.GetGuildFeed(ctx, r.GuildConfigService, mra.GuildID)
	if gfErr != nil {
		r.ErrorOutput(ctx, command.Name, mra.ChannelID, Error{
			Message: gfErr.Message,
			Err:     gfErr,
		})
		return
	}

	if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, r.Config.Bot.GuildService, "Servers"); vErr != nil {
		r.ErrorOutput(ctx, command.Name, mra.ChannelID, Error{
			Message: vErr.Message,
			Err:     vErr,
		})
		return
	}

	var serversToUnbanOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToUnbanOn = append(serversToUnbanOn, *aServer)
				break
			}
		}
	}

	if len(serversToUnbanOn) == 0 {
		r.ErrorOutput(ctx, "Failed to unban player", mra.ChannelID, Error{
			Message: "no servers found to unban on from guild config",
			Err:     errors.New("please run the unban command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan UnbanSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan UnbanError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleUnbanPlayerResponses(ctx, s, mra, command, len(serversToUnbanOn), successChannel, errorChannel)

	for _, stb := range serversToUnbanOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.UnbanPlayerRequest(ctx, aServer, cbr.PlayerName, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// UnbanPlayerRequest func
func (r *Reactions) UnbanPlayerRequest(ctx context.Context, server gcscmodels.Server, playerName string, banSuccess chan UnbanSuccess, banError chan UnbanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.UnbanPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName)
	if err != nil {
		banError <- UnbanError{
			Server:     server,
			Message:    err.Message(),
			Error:      err.Error(),
			PlayerName: playerName,
		}
		return
	}

	banSuccess <- UnbanSuccess{
		Server:     server,
		PlayerName: playerName,
	}
	return
}

// HandleUnbanPlayerResponses func
func (r *Reactions) HandleUnbanPlayerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, banSuccess chan UnbanSuccess, banError chan UnbanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []UnbanSuccess
	var errs []UnbanError

	var timer *time.Timer

Loop:
	for {
		if count == servers {
			break
		}

		// Auto-close for loop if timer has fired
		if timer != nil {
			select {
			case <-timer.C:
				break Loop
			default:
				break
			}
		}

		select {
		case success := <-banSuccess:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			successes = append(successes, success)
		case err := <-banError:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			errs = append(errs, err)
		default:
			break
		}
	}

	var playerName string
	var unbanPlayerSuccess UnbanPlayerSuccess
	var unbanErrorTypes map[string]UnbanPlayerError = make(map[string]UnbanPlayerError)

	for _, success := range successes {
		unbanPlayerSuccess.Servers = append(unbanPlayerSuccess.Servers, success.Server)
		unbanPlayerSuccess.PlayerName = success.PlayerName

		if playerName == "" {
			playerName = success.PlayerName
		}
	}

	for _, err := range errs {
		errMsg := err.Error
		switch errMsg {
		case "Can't lookup player name to ID.":
			errMsg = "Nitrado could not find player: " + err.PlayerName
		}

		if val, ok := unbanErrorTypes[errMsg]; ok {
			servers := append(val.Servers, err.Server)
			unbanErrorTypes[errMsg] = UnbanPlayerError{
				Message:    errMsg,
				Servers:    servers,
				PlayerName: err.PlayerName,
			}
		} else {
			unbanErrorTypes[errMsg] = UnbanPlayerError{
				Message: errMsg,
				Servers: []gcscmodels.Server{
					err.Server,
				},
				PlayerName: err.PlayerName,
			}
		}

		if playerName == "" {
			playerName = err.PlayerName
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(unbanPlayerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &unbanPlayerSuccess)
	}

	for _, err := range unbanErrorTypes {
		var anErr UnbanPlayerError = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Unbanned %s", playerName)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *UnbanPlayerError) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to unban player on servers"
	}

	for _, server := range bpe.Servers {
		fieldVal += fmt.Sprintf("[%d] - %s\n", server.NitradoID, server.Name)
	}

	if fieldVal == "" {
		fieldVal = "Unknown servers"
	} else if len(fieldVal) > 800 {
		fieldVal = fieldVal[:800]
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *UnbanPlayerSuccess) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "This may take up to 5 minutes to take effect."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s unbanned on %d server(s)", bps.PlayerName, len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
