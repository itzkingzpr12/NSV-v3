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

type RefreshBansSuccessOutput struct {
	ServerCount int
	BanCount    int
}

type RefreshBansSuccess struct {
	Server     gcscmodels.Server
	PlayerName string
}

type RefreshBanError struct {
	Server     gcscmodels.Server
	Message    string
	Error      string
	PlayerName string
}

type ServerBans struct {
	Server gcscmodels.Server
	Bans   []string
}

// RefreshBans func
func (r *Reactions) RefreshBans(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var cbr *models.RefreshBansReaction
	cacheKey := cbr.CacheKey(r.Config.CacheSettings.RefreshBansReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &cbr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to refresh bans", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if cbr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "refresh bans reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to refresh bans", mra.ChannelID, Error{
			Message: "refresh bans message has expired",
			Err:     errors.New("please run the refresh bans command again"),
		})
		return
	}

	if len(cbr.ServerBans) == 0 {
		r.ErrorOutput(ctx, "Failed to refresh bans", mra.ChannelID, Error{
			Message: "no servers found to refresh bans on",
			Err:     errors.New("your bans may already be in sync"),
		})
		return
	}

	delete(r.MessagesAwaitingReaction.Messages, mra.MessageID)

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

	var serversBans []ServerBans
	var totalBans int
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		for serverID, cachedServerBans := range cbr.ServerBans {
			if aServer.ID == serverID {
				var bans []string = cachedServerBans
				serversBans = append(serversBans, ServerBans{
					Server: *aServer,
					Bans:   bans,
				})
				totalBans += len(bans)
			}
		}
	}

	if len(serversBans) == 0 {
		r.ErrorOutput(ctx, "Failed to refresh bans", mra.ChannelID, Error{
			Message: "no servers found to refresh bans in guild config",
			Err:     errors.New("please run the refresh bans command again"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan RefreshBansSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan RefreshBanError, len(guildFeed.Payload.Guild.Servers))

	go r.HandleRefreshBansResponses(ctx, s, mra, command, totalBans, successChannel, errorChannel)

	for _, sb := range serversBans {
		for _, aPlayer := range sb.Bans {
			var player string = aPlayer
			var aServer gcscmodels.Server = sb.Server
			wp.Submit(func() {
				r.RefreshBansRequest(ctx, aServer, player, successChannel, errorChannel)
			})
		}
	}

	return

}

// RefreshBansRequest func
func (r *Reactions) RefreshBansRequest(ctx context.Context, server gcscmodels.Server, playerName string, banSuccess chan RefreshBansSuccess, banError chan RefreshBanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := r.NitradoService.Client.BanPlayer(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName)
	if err != nil {
		banError <- RefreshBanError{
			Server:     server,
			Message:    err.Message(),
			Error:      err.Error(),
			PlayerName: playerName,
		}
		return
	}

	banSuccess <- RefreshBansSuccess{
		Server:     server,
		PlayerName: playerName,
	}
	return
}

// HandleRefreshBansResponses func
func (r *Reactions) HandleRefreshBansResponses(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command, servers int, banSuccess chan RefreshBansSuccess, banError chan RefreshBanError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []RefreshBansSuccess
	var errs []RefreshBanError

	var timer *time.Timer = time.NewTimer(3600 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-banSuccess:
			count++
			successes = append(successes, success)
		case err := <-banError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var refreshBansSuccess RefreshBansSuccessOutput

	var uniqueServerSuccesses map[uint64]bool = make(map[uint64]bool, 0)
	var uniqueBansSuccesses map[string]bool = make(map[string]bool, 0)

	for _, success := range successes {
		if _, ok := uniqueServerSuccesses[success.Server.ID]; !ok {
			uniqueServerSuccesses[success.Server.ID] = true
		}

		if _, ok := uniqueBansSuccesses[success.PlayerName]; !ok {
			uniqueBansSuccesses[success.PlayerName] = true
		}
	}

	refreshBansSuccess.BanCount = len(uniqueBansSuccesses)
	refreshBansSuccess.ServerCount = len(uniqueServerSuccesses)

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if refreshBansSuccess.ServerCount > 0 && refreshBansSuccess.BanCount > 0 {
		embeddableFields = append(embeddableFields, &refreshBansSuccess)
	} else {
		r.ErrorOutput(ctx, command.Name, mra.ChannelID, Error{
			Message: "No bans applied",
			Err:     errors.New("Failed to apply any new bans across your cluster"),
		})
		return
	}

	editedCommand := command
	editedCommand.Name = "Refreshed Bans"
	editedCommand.Description = "All servers that had incomplete banlists were updated. This may take up to 5 minutes to take effect."

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *RefreshBansSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("Servers Refreshed: %d\nPlayers Banned: %d", bps.ServerCount, bps.BanCount)

	return &discordgo.MessageEmbedField{
		Name:   "Finished Refresh",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
