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

type WhitelistPlayerSuccess struct {
	Servers    []gcscmodels.Server
	PlayerName string
}

type WhitelistPlayerError struct {
	Message    string
	Servers    []gcscmodels.Server
	PlayerName string
}

type WhitelistSuccess struct {
	Server     gcscmodels.Server
	PlayerName string
}

type WhitelistError struct {
	Server     gcscmodels.Server
	Message    string
	Error      string
	PlayerName string
}

// WhitelistPlayer func
func (r *Reactions) WhitelistPlayer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.WhitelistReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.WhitelistReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to whitelist player", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "whitelist reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to whitelist player", mra.ChannelID, Error{
			Message: "whitelist player message has expired",
			Err:     errors.New("please run the whitelist command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to whitelist player", mra.ChannelID, Error{
			Message: "no servers found to whitelist on",
			Err:     errors.New("please run the whitelist command again"),
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

	var serversToWhitelistOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToWhitelistOn = append(serversToWhitelistOn, *aServer)
				break
			}
		}
	}

	if len(serversToWhitelistOn) == 0 {
		r.ErrorOutput(ctx, "Failed to whitelist player", mra.ChannelID, Error{
			Message: "no servers found to whitelist on from guild config",
			Err:     errors.New("please run the whitelist command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan WhitelistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan WhitelistError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleWhitelistPlayerResponses(ctx, s, mra, command, len(serversToWhitelistOn), successChannel, errorChannel)

	for _, stb := range serversToWhitelistOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.WhitelistPlayerRequest(ctx, aServer, cbr.PlayerName, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// WhitelistPlayerRequest func
func (r *Reactions) WhitelistPlayerRequest(ctx context.Context, server gcscmodels.Server, playerName string, whitelistSuccess chan WhitelistSuccess, whitelistError chan WhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.WhitelistPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName)
	if err != nil {
		whitelistError <- WhitelistError{
			Server:     server,
			Message:    err.Message(),
			Error:      err.Error(),
			PlayerName: playerName,
		}
		return
	}

	whitelistSuccess <- WhitelistSuccess{
		Server:     server,
		PlayerName: playerName,
	}
	return
}

// HandleWhitelistPlayerResponses func
func (r *Reactions) HandleWhitelistPlayerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, whitelistSuccess chan WhitelistSuccess, whitelistError chan WhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []WhitelistSuccess
	var errs []WhitelistError

	var timer *time.Timer = time.NewTimer(120 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-whitelistSuccess:
			count++
			successes = append(successes, success)
		case err := <-whitelistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var playerName string

	var whitelistPlayerSuccess WhitelistPlayerSuccess
	var whitelistErrorTypes map[string]WhitelistPlayerError = make(map[string]WhitelistPlayerError)

	for _, success := range successes {
		whitelistPlayerSuccess.Servers = append(whitelistPlayerSuccess.Servers, success.Server)
		whitelistPlayerSuccess.PlayerName = success.PlayerName

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

		if val, ok := whitelistErrorTypes[errMsg]; ok {
			servers := append(val.Servers, err.Server)
			whitelistErrorTypes[errMsg] = WhitelistPlayerError{
				Message:    errMsg,
				Servers:    servers,
				PlayerName: err.PlayerName,
			}
		} else {
			whitelistErrorTypes[errMsg] = WhitelistPlayerError{
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

	if len(whitelistPlayerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &whitelistPlayerSuccess)
	}

	for _, err := range whitelistErrorTypes {
		var anErr WhitelistPlayerError = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Whitelisted %s", playerName)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *WhitelistPlayerError) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to whitelist player on servers"
	}

	for _, server := range bpe.Servers {
		fieldVal += fmt.Sprintf("**%d** - %s\n", server.NitradoID, server.Name)
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
func (bps *WhitelistPlayerSuccess) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "This may take up to 5 minutes to take effect."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s whitelisted on %d server(s)", bps.PlayerName, len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
