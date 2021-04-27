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

type StopServerSuccess struct {
	Servers []gcscmodels.Server
}

type StopServerError struct {
	Message string
	Servers []gcscmodels.Server
}

type StopSuccess struct {
	Server gcscmodels.Server
}

type StopError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

// StopServer func
func (r *Reactions) StopServer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.StopReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.StopReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to stop server", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "stop server reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to stop server", mra.ChannelID, Error{
			Message: "stop server message has expired",
			Err:     errors.New("please run the stop server command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to stop server", mra.ChannelID, Error{
			Message: "no servers found to stop server on",
			Err:     errors.New("please run the stop server command again"),
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

	var serversToStopOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToStopOn = append(serversToStopOn, *aServer)
				break
			}
		}
	}

	if len(serversToStopOn) == 0 {
		r.ErrorOutput(ctx, "Failed to stop server", mra.ChannelID, Error{
			Message: "no servers found to stop server on from guild config",
			Err:     errors.New("please run the stop server command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan StopSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan StopError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleStopServerResponses(ctx, s, mra, command, len(serversToStopOn), successChannel, errorChannel)

	for _, stb := range serversToStopOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.StopServerRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// StopServerRequest func
func (r *Reactions) StopServerRequest(ctx context.Context, server gcscmodels.Server, stopSuccess chan StopSuccess, stopError chan StopError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.StopGameserver(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), "Stop executed by Nitrado Server Manager V2", "")
	if err != nil {
		stopError <- StopError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	stopSuccess <- StopSuccess{
		Server: server,
	}
	return
}

// HandleStopServerResponses func
func (r *Reactions) HandleStopServerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, stopSuccess chan StopSuccess, stopError chan StopError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []StopSuccess
	var errs []StopError

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
		case success := <-stopSuccess:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			successes = append(successes, success)
		case err := <-stopError:
			if timer == nil {
				timer = time.NewTimer(120 * time.Second)
			}
			count++
			errs = append(errs, err)
		default:
			break
		}
	}

	var StopServerSuccess StopServerSuccess
	var stopServerErrorTypes map[string]StopServerError = make(map[string]StopServerError)

	for _, success := range successes {
		StopServerSuccess.Servers = append(StopServerSuccess.Servers, success.Server)
	}

	for _, err := range errs {
		if val, ok := stopServerErrorTypes[err.Error]; ok {
			servers := append(val.Servers, err.Server)
			stopServerErrorTypes[err.Error] = StopServerError{
				Message: err.Error,
				Servers: servers,
			}
		} else {
			stopServerErrorTypes[err.Error] = StopServerError{
				Message: err.Error,
				Servers: []gcscmodels.Server{
					err.Server,
				},
			}
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(StopServerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &StopServerSuccess)
	}

	for _, err := range stopServerErrorTypes {
		var anErr StopServerError = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = "Stopped Server(s)"

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *StopServerError) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to stop server on servers"
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
func (bps *StopServerSuccess) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "This will take effect immediately."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("Stopped %d server(s)", len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
