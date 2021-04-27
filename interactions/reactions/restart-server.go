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

type RestartServerSuccess struct {
	Servers []gcscmodels.Server
}

type RestartServerError struct {
	Message string
	Servers []gcscmodels.Server
}

type RestartSuccess struct {
	Server gcscmodels.Server
}

type RestartError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

// RestartServer func
func (r *Reactions) RestartServer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.RestartReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.RestartReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to restart server", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "restart server reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to restart server", mra.ChannelID, Error{
			Message: "restart server message has expired",
			Err:     errors.New("please run the restart server command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to restart server", mra.ChannelID, Error{
			Message: "no servers found to restart server on",
			Err:     errors.New("please run the restart server command again"),
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

	var serversToRestartOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToRestartOn = append(serversToRestartOn, *aServer)
				break
			}
		}
	}

	if len(serversToRestartOn) == 0 {
		r.ErrorOutput(ctx, "Failed to restart server", mra.ChannelID, Error{
			Message: "no servers found to restart server on from guild config",
			Err:     errors.New("please run the restart server command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan RestartSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan RestartError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleRestartServerResponses(ctx, s, mra, command, len(serversToRestartOn), successChannel, errorChannel)

	for _, stb := range serversToRestartOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.RestartServerRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// RestartServerRequest func
func (r *Reactions) RestartServerRequest(ctx context.Context, server gcscmodels.Server, restartSuccess chan RestartSuccess, restartError chan RestartError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.RestartGameserver(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), "Restart executed by Nitrado Server Manager V2", "")
	if err != nil {
		restartError <- RestartError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	restartSuccess <- RestartSuccess{
		Server: server,
	}
	return
}

// HandleRestartServerResponses func
func (r *Reactions) HandleRestartServerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, restartSuccess chan RestartSuccess, restartError chan RestartError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []RestartSuccess
	var errs []RestartError

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
		case success := <-restartSuccess:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			successes = append(successes, success)
		case err := <-restartError:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			errs = append(errs, err)
		default:
			break
		}
	}

	var RestartServerSuccess RestartServerSuccess
	var restartServerErrorTypes map[string]RestartServerError = make(map[string]RestartServerError)

	for _, success := range successes {
		RestartServerSuccess.Servers = append(RestartServerSuccess.Servers, success.Server)
	}

	for _, err := range errs {
		if val, ok := restartServerErrorTypes[err.Error]; ok {
			servers := append(val.Servers, err.Server)
			restartServerErrorTypes[err.Error] = RestartServerError{
				Message: err.Error,
				Servers: servers,
			}
		} else {
			restartServerErrorTypes[err.Error] = RestartServerError{
				Message: err.Error,
				Servers: []gcscmodels.Server{
					err.Server,
				},
			}
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(RestartServerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &RestartServerSuccess)
	}

	for _, err := range restartServerErrorTypes {
		var anErr RestartServerError = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = "Restarted Server(s)"

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *RestartServerError) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to restart server on servers"
	}

	for _, server := range bpe.Servers {
		fieldVal += fmt.Sprintf("(%d) - %s\n", server.NitradoID, server.Name)
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
func (bps *RestartServerSuccess) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "The restart will start immediately and may adhere to your \"restart countdown\" settings."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("Restarted %d server(s)", len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
