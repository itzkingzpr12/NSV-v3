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

type UnwhitelistPlayerSuccess struct {
	Servers    []gcscmodels.Server
	PlayerName string
}

type UnwhitelistPlayerError struct {
	Message    string
	Servers    []gcscmodels.Server
	PlayerName string
}

type UnwhitelistSuccess struct {
	Server     gcscmodels.Server
	PlayerName string
}

type UnwhitelistError struct {
	Server     gcscmodels.Server
	Message    string
	Error      string
	PlayerName string
}

// UnwhitelistPlayer func
func (r *Reactions) UnwhitelistPlayer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.UnwhitelistReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.UnwhitelistReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to unwhitelist player", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "unwhitelist reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to unwhitelist player", mra.ChannelID, Error{
			Message: "unwhitelist player message has expired",
			Err:     errors.New("please run the unwhitelist command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to unwhitelist player", mra.ChannelID, Error{
			Message: "no servers found to unwhitelist on",
			Err:     errors.New("please run the unwhitelist command again"),
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

	var serversToUnwhitelistOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToUnwhitelistOn = append(serversToUnwhitelistOn, *aServer)
				break
			}
		}
	}

	if len(serversToUnwhitelistOn) == 0 {
		r.ErrorOutput(ctx, "Failed to unwhitelist player", mra.ChannelID, Error{
			Message: "no servers found to unwhitelist on from guild config",
			Err:     errors.New("please run the unwhitelist command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan UnwhitelistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan UnwhitelistError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleUnwhitelistPlayerResponses(ctx, s, mra, command, len(serversToUnwhitelistOn), successChannel, errorChannel)

	for _, stb := range serversToUnwhitelistOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.UnwhitelistPlayerRequest(ctx, aServer, cbr.PlayerName, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// UnwhitelistPlayerRequest func
func (r *Reactions) UnwhitelistPlayerRequest(ctx context.Context, server gcscmodels.Server, playerName string, unwhitelistSuccess chan UnwhitelistSuccess, unwhitelistError chan UnwhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.UnwhitelistPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName)
	if err != nil {
		unwhitelistError <- UnwhitelistError{
			Server:     server,
			Message:    err.Message(),
			Error:      err.Error(),
			PlayerName: playerName,
		}
		return
	}

	unwhitelistSuccess <- UnwhitelistSuccess{
		Server:     server,
		PlayerName: playerName,
	}
	return
}

// HandleUnwhitelistPlayerResponses func
func (r *Reactions) HandleUnwhitelistPlayerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, unwhitelistSuccess chan UnwhitelistSuccess, unwhitelistError chan UnwhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []UnwhitelistSuccess
	var errs []UnwhitelistError

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
		case success := <-unwhitelistSuccess:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			successes = append(successes, success)
		case err := <-unwhitelistError:
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

	var unwhitelistPlayerSuccess UnwhitelistPlayerSuccess
	var unwhitelistErrorTypes map[string]UnwhitelistPlayerError = make(map[string]UnwhitelistPlayerError)

	for _, success := range successes {
		unwhitelistPlayerSuccess.Servers = append(unwhitelistPlayerSuccess.Servers, success.Server)
		unwhitelistPlayerSuccess.PlayerName = success.PlayerName

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

		if val, ok := unwhitelistErrorTypes[errMsg]; ok {
			servers := append(val.Servers, err.Server)
			unwhitelistErrorTypes[errMsg] = UnwhitelistPlayerError{
				Message:    errMsg,
				Servers:    servers,
				PlayerName: err.PlayerName,
			}
		} else {
			unwhitelistErrorTypes[errMsg] = UnwhitelistPlayerError{
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

	if len(unwhitelistPlayerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &unwhitelistPlayerSuccess)
	}

	for _, err := range unwhitelistErrorTypes {
		var anErr UnwhitelistPlayerError = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Unwhitelisted %s", playerName)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *UnwhitelistPlayerError) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to unwhitelist player on servers"
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
func (bps *UnwhitelistPlayerSuccess) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "This may take up to 5 minutes to take effect."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s unwhitelisted on %d server(s)", bps.PlayerName, len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
