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

type BanPlayerSuccessOutput struct {
	Servers    []gcscmodels.Server
	PlayerName string
}

type BanPlayerErrorOutput struct {
	Message    string
	Servers    []gcscmodels.Server
	PlayerName string
}

type BanSuccess struct {
	Server     gcscmodels.Server
	PlayerName string
}

type BanError struct {
	Server     gcscmodels.Server
	Message    string
	Error      string
	PlayerName string
}

// BanPlayer func
func (r *Reactions) BanPlayer(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.BanReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.BanReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to ban player", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "ban reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to ban player", mra.ChannelID, Error{
			Message: "ban player message has expired",
			Err:     errors.New("please run the ban command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to ban player", mra.ChannelID, Error{
			Message: "no servers found to ban on",
			Err:     errors.New("please run the ban command again"),
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

	var serversToBanOn []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				serversToBanOn = append(serversToBanOn, *aServer)
				break
			}
		}
	}

	if len(serversToBanOn) == 0 {
		r.ErrorOutput(ctx, "Failed to ban player", mra.ChannelID, Error{
			Message: "no servers found to ban on from guild config",
			Err:     errors.New("please run the ban command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan BanSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan BanError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleBanPlayerResponses(ctx, s, mra, command, len(serversToBanOn), successChannel, errorChannel)

	for _, stb := range serversToBanOn {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.BanPlayerRequest(ctx, aServer, cbr.PlayerName, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// BanPlayerRequest func
func (r *Reactions) BanPlayerRequest(ctx context.Context, server gcscmodels.Server, playerName string, banSuccess chan BanSuccess, banError chan BanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.BanPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName)
	if err != nil {
		banError <- BanError{
			Server:     server,
			Message:    err.Message(),
			Error:      err.Error(),
			PlayerName: playerName,
		}
		return
	}

	banSuccess <- BanSuccess{
		Server:     server,
		PlayerName: playerName,
	}
	return
}

// HandleBanPlayerResponses func
func (r *Reactions) HandleBanPlayerResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, banSuccess chan BanSuccess, banError chan BanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []BanSuccess
	var errs []BanError

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

	var banPlayerSuccess BanPlayerSuccessOutput
	var banErrorTypes map[string]BanPlayerErrorOutput = make(map[string]BanPlayerErrorOutput)

	for _, success := range successes {
		banPlayerSuccess.Servers = append(banPlayerSuccess.Servers, success.Server)
		banPlayerSuccess.PlayerName = success.PlayerName

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

		if val, ok := banErrorTypes[errMsg]; ok {
			servers := append(val.Servers, err.Server)
			banErrorTypes[errMsg] = BanPlayerErrorOutput{
				Message:    errMsg,
				Servers:    servers,
				PlayerName: err.PlayerName,
			}
		} else {
			banErrorTypes[errMsg] = BanPlayerErrorOutput{
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

	if len(banPlayerSuccess.Servers) > 0 {
		embeddableFields = append(embeddableFields, &banPlayerSuccess)
	}

	for _, err := range banErrorTypes {
		var anErr BanPlayerErrorOutput = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Banned %s", playerName)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *BanPlayerErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to ban player on servers"
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
func (bps *BanPlayerSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "This may take up to 5 minutes to take effect."

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s banned on %d server(s)", bps.PlayerName, len(bps.Servers)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
