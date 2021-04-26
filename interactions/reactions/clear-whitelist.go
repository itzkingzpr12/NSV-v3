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
	nitrado_service_v2_client "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

type ReactionSuccessOutput struct {
	ServerCount        int
	UnwhitelistedCount int
}

type ReactionErrorOutput struct {
	Message                string
	FailedUnwhitelistCount int
}

type GetWhitelistServerErrorOutput struct {
	Message string
	Servers []gcscmodels.Server
}

type GetWhitelistSuccess struct {
	Server  gcscmodels.Server
	Players []nitrado_service_v2_client.Player
}

type GetWhitelistError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

type ClearWhitelistSuccess struct {
	Server gcscmodels.Server
	Player nitrado_service_v2_client.Player
}

type ClearWhitelistError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

// ClearWhitelist func
func (r *Reactions) ClearWhitelist(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.ClearWhitelistReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.ClearWhitelistReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to clear whitelist", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "clear whitelist reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to clear whitelist", mra.ChannelID, Error{
			Message: "clear whitelist message has expired",
			Err:     errors.New("please run the clear whitelist command again"),
		})
		return
	}

	if len(cbr.Servers) == 0 {
		r.ErrorOutput(ctx, "Failed to clear whitelist", mra.ChannelID, Error{
			Message: "no servers found to clear whitelist on",
			Err:     errors.New("please run the clear whitelist command again"),
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

	var servers []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for _, cachedServer := range cbr.Servers {
			if aServer.ID == cachedServer.ID {
				servers = append(servers, *aServer)
				break
			}
		}
	}

	if len(servers) == 0 {
		r.ErrorOutput(ctx, "Failed to clear whitelist", mra.ChannelID, Error{
			Message: "no servers found to clear whitelist on from guild config",
			Err:     errors.New("please run the clear whitelist command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan GetWhitelistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan GetWhitelistError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleGetWhitelistResponse(ctx, s, mra, command, len(servers), successChannel, errorChannel)

	for _, stb := range servers {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			r.GetWhitelistRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

	return

}

// GetWhitelistRequest func
func (r *Reactions) GetWhitelistRequest(ctx context.Context, server gcscmodels.Server, getWhitelistSuccess chan GetWhitelistSuccess, getWhitelistError chan GetWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	whitelist, err := r.NitradoService.Client.GetWhitelist(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), false)
	if err != nil {
		getWhitelistError <- GetWhitelistError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	var players []nitrado_service_v2_client.Player

	for _, player := range whitelist.Players {
		if player.Name == "" {
			continue
		}

		var aPlayer nitrado_service_v2_client.Player = player
		players = append(players, aPlayer)
	}
	getWhitelistSuccess <- GetWhitelistSuccess{
		Server:  server,
		Players: players,
	}
	return
}

// HandleGetWhitelistResponse func
func (r *Reactions) HandleGetWhitelistResponse(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, getWhitelistSuccess chan GetWhitelistSuccess, getWhitelistError chan GetWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []GetWhitelistSuccess
	var errs []GetWhitelistError

	var timer *time.Timer = time.NewTimer(300 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-getWhitelistSuccess:
			count++
			successes = append(successes, success)
		case err := <-getWhitelistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	cwsErr := r.ClearWhitelistOnServers(ctx, s, mra, command, successes, errs)
	if cwsErr != nil {
		r.ErrorOutput(ctx, "Failed to clear whitelist", mra.ChannelID, *cwsErr)
		return
	}

	return
}

func (r *Reactions) ClearWhitelistOnServers(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, serverWhitelists []GetWhitelistSuccess, serverErrors []GetWhitelistError) *Error {
	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	totalPlayers := 0
	for _, aWhitelist := range serverWhitelists {
		totalPlayers += len(aWhitelist.Players)
	}

	if totalPlayers == 0 {
		return &Error{
			Message: "Whitelist is already cleared",
			Err:     errors.New("empty whitelist"),
		}
	}

	successChannel := make(chan ClearWhitelistSuccess, totalPlayers)
	errorChannel := make(chan ClearWhitelistError, totalPlayers)

	go r.HandleClearWhitelistResponse(ctx, s, mra, command, totalPlayers, serverErrors, successChannel, errorChannel)

	for _, sw := range serverWhitelists {
		var server gcscmodels.Server = sw.Server
		for _, aPlayer := range sw.Players {
			var player nitrado_service_v2_client.Player = aPlayer
			wp.Submit(func() {
				r.ClearWhitelistRequest(ctx, server, player, successChannel, errorChannel)
			})
		}
	}

	return nil
}

// ClearWhitelistRequest func
func (r *Reactions) ClearWhitelistRequest(ctx context.Context, server gcscmodels.Server, player nitrado_service_v2_client.Player, clearWhitelistSuccess chan ClearWhitelistSuccess, clearWhitelistError chan ClearWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.UnwhitelistPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), player.Name)
	if err != nil {
		clearWhitelistError <- ClearWhitelistError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	clearWhitelistSuccess <- ClearWhitelistSuccess{
		Server: server,
		Player: player,
	}
	return
}

// HandleClearWhitelistResponse func
func (r *Reactions) HandleClearWhitelistResponse(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, getWhitelistErrors []GetWhitelistError, clearWhitelistSuccess chan ClearWhitelistSuccess, clearWhitelistError chan ClearWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []ClearWhitelistSuccess
	var errs []ClearWhitelistError

	var timer *time.Timer = time.NewTimer(900 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-clearWhitelistSuccess:
			count++
			successes = append(successes, success)
		case err := <-clearWhitelistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var reactionSuccessOutput ReactionSuccessOutput
	var reactionErrorTypes map[string]ReactionErrorOutput = make(map[string]ReactionErrorOutput, 0)
	var getWhitelistServerErrorOutput map[string]GetWhitelistServerErrorOutput = make(map[string]GetWhitelistServerErrorOutput, 0)

	var uniquePlayers map[string]bool = make(map[string]bool, 0)
	var uniqueServers map[uint64]bool = make(map[uint64]bool, 0)
	for _, success := range successes {
		if _, ok := uniquePlayers[success.Player.Name]; !ok {
			reactionSuccessOutput.UnwhitelistedCount++
			uniquePlayers[success.Player.Name] = true
		}
		if _, ok := uniqueServers[success.Server.ID]; !ok {
			reactionSuccessOutput.ServerCount++
			uniqueServers[success.Server.ID] = true
		}
	}

	for _, err := range errs {
		errMsg := err.Error

		if val, ok := reactionErrorTypes[errMsg]; ok {
			val.FailedUnwhitelistCount += 1
			reactionErrorTypes[errMsg] = val
		} else {
			reactionErrorTypes[errMsg] = ReactionErrorOutput{
				Message:                errMsg,
				FailedUnwhitelistCount: 1,
			}
		}
	}

	for _, err := range getWhitelistErrors {
		errMsg := err.Error

		if val, ok := getWhitelistServerErrorOutput[errMsg]; ok {
			val.Servers = append(val.Servers, err.Server)
			getWhitelistServerErrorOutput[errMsg] = val
		} else {
			getWhitelistServerErrorOutput[errMsg] = GetWhitelistServerErrorOutput{
				Message: errMsg,
				Servers: []gcscmodels.Server{
					err.Server,
				},
			}
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if reactionSuccessOutput.UnwhitelistedCount > 0 {
		embeddableFields = append(embeddableFields, &reactionSuccessOutput)
	}

	for _, anErr := range reactionErrorTypes {
		var err ReactionErrorOutput = anErr
		embeddableErrors = append(embeddableErrors, &err)
	}

	for _, anErr := range getWhitelistServerErrorOutput {
		var err GetWhitelistServerErrorOutput = anErr
		embeddableErrors = append(embeddableErrors, &err)
	}

	editedCommand := command
	editedCommand.Name = "Cleared Whitelist"

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)

	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bpe *ReactionErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("```%d users failed to unwhitelist```", bpe.FailedUnwhitelistCount)
	name := bpe.Message
	if name == "" {
		name = "Failed to unwhitelist users"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *ReactionSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```\nPlease wait up to 30 minutes for all players to be cleared from the whitelist. You can use the \"n!whitelist\" command to view your whitelist.\n```"

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%d players unwhitelisted across %d servers", bps.UnwhitelistedCount, bps.ServerCount),
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for GetWhitelistServerErrorOutput struct
func (bpe *GetWhitelistServerErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"
	name := bpe.Message
	if name == "" {
		name = "Failed to clear whitelist on servers"
	}

	for _, server := range bpe.Servers {
		fieldVal += server.Name + "\n"
	}

	fieldVal += "```"

	if fieldVal == "``````" {
		fieldVal = "```Unknown servers```"
	} else if len(fieldVal) > 800 {
		fieldVal = fieldVal[:800]
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
