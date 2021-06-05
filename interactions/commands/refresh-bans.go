package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gammazero/workerpool"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/reactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// RefreshBansCommand struct
type RefreshBansCommand struct {
	Params RefreshBansCommandParams
}

// RefreshBansCommandParams struct
type RefreshBansCommandParams struct {
	ServerIDs []int64
}

// RefreshBansCommandInProgressOutput struct
type RefreshBansCommandInProgressOutput struct {
	ServerCount int
}

// RefreshBansCommandConfirmationOutput struct
type RefreshBansCommandConfirmationOutput struct {
	Bans    int
	Servers int
}

// RefreshBans func
func (c *Commands) RefreshBans(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseRefreshBansCommand(command, mc)
	if nscErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *nscErr)
		return
	}

	guildFeed, gfErr := guildconfigservice.GetGuildFeed(ctx, c.GuildConfigService, mc.GuildID)
	if gfErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: gfErr.Message,
			Err:     gfErr,
		})
		return
	}

	if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, c.Config.Bot.GuildService, "Servers"); vErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: vErr.Message,
			Err:     vErr,
		})
		return
	}

	if !c.IsApproved(ctx, guildFeed.Payload.Guild, command.Name, mc.Member.Roles) {
		isAdmin, iaErr := c.IsAdmin(ctx, mc.GuildID, mc.Member.Roles)
		if iaErr != nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *iaErr)
			return
		}
		if !isAdmin {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Unauthorized to use this command",
				Err:     errors.New("user is not authorized"),
			})
			return
		}
	}

	var servers []gcscmodels.Server
	var syncServers []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		if len(parsedCommand.Params.ServerIDs) == 0 {
			syncServers = append(syncServers, *aServer)
		} else {
			for _, syncServer := range parsedCommand.Params.ServerIDs {
				if aServer.NitradoID == syncServer {
					syncServers = append(syncServers, *aServer)
				}
			}
		}

		servers = append(servers, *aServer)
	}

	if len(parsedCommand.Params.ServerIDs) > 0 && len(syncServers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Invalid server(s) to refresh",
			Err:     errors.New("unknown servers requested in refresh"),
		})
		return
	}

	if len(servers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find servers to get banlist",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	var ef []discordapi.EmbeddableField
	var ee []discordapi.EmbeddableField

	ef = append(ef, &RefreshBansCommandInProgressOutput{
		ServerCount: len(servers),
	})

	embedParams := discordapi.EmbeddableParams{
		Title:        command.Name,
		Description:  command.Description,
		TitleURL:     c.Config.Bot.DocumentationURL,
		Footer:       fmt.Sprintf("Executed by %s", mc.Author.Username),
		ThumbnailURL: c.Config.Bot.WorkingThumbnail,
	}

	_, spoErr := c.Output(ctx, mc.ChannelID, embedParams, ef, ee)
	if spoErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to send refresh bans processing message",
			Err:     spoErr,
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan GetBanlistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan GetBanlistError, len(guildFeed.Payload.Guild.Servers))

	go c.HandleRefreshBansResponses(ctx, s, mc, command, len(servers), syncServers, successChannel, errorChannel)

	for _, stb := range servers {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			c.GetBanlistRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	return
}

// parseRefreshBansCommand func
func parseRefreshBansCommand(command configs.Command, mc *discordgo.MessageCreate) (*RefreshBansCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	if len(splitContent) == 1 {
		return &RefreshBansCommand{
			Params: RefreshBansCommandParams{},
		}, nil
	}

	var serverIDs []int64

	for _, serverID := range splitContent[1:] {
		serverIDInt, sidErr := strconv.ParseInt(serverID, 10, 64)
		if sidErr != nil {
			return nil, &Error{
				Message: fmt.Sprintf("Invalid Server ID: %s", serverID),
				Err:     errors.New("invalid server id"),
			}
		}

		serverIDs = append(serverIDs, serverIDInt)
	}

	return &RefreshBansCommand{
		Params: RefreshBansCommandParams{
			ServerIDs: serverIDs,
		},
	}, nil
}

// HandleRefreshBansResponses func
func (c *Commands) HandleRefreshBansResponses(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command, servers int, syncServers []gcscmodels.Server, getBanlistSuccess chan GetBanlistSuccess, getBanlistError chan GetBanlistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []GetBanlistSuccess
	var errs []GetBanlistError

	var timer *time.Timer = time.NewTimer(120 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-getBanlistSuccess:
			count++
			successes = append(successes, success)
		case err := <-getBanlistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	if len(successes) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to get banlists",
			Err:     errors.New("unable to retrieve banlists"),
		})
		return
	}

	var uniqueBans map[string]bool = make(map[string]bool, 0)
	var existingServerBans map[uint64][]string = make(map[uint64][]string, 0)
	var newServerBans map[uint64][]string = make(map[uint64][]string, 0)

	for _, aServer := range syncServers {
		for _, banlist := range successes {
			if aServer.ID == banlist.Server.ID {
				var bans []string
				for _, ban := range banlist.Players {
					bans = append(bans, ban.Name)
				}

				if _, ok := existingServerBans[aServer.ID]; !ok {
					existingServerBans[aServer.ID] = bans
				} else {
					existingServerBans[aServer.ID] = append(existingServerBans[aServer.ID], bans...)
				}
			}
		}

		if _, ok := existingServerBans[aServer.ID]; !ok {
			existingServerBans[aServer.ID] = []string{}
		}
	}

	for _, banlist := range successes {
		for _, ban := range banlist.Players {
			if _, ok := uniqueBans[ban.Name]; ok {
				continue
			}

			uniqueBans[ban.Name] = true

			for key, val := range existingServerBans {
				addBan := true
				for _, prevBan := range val {
					if prevBan == ban.Name {
						addBan = false
						break
					}
				}

				if addBan {
					newServerBans[key] = append(newServerBans[key], ban.Name)
				}
			}
		}
	}

	foundBansForSync := false
	for key, newBans := range newServerBans {
		if len(newBans) == 0 {
			delete(newServerBans, key)
		} else {
			foundBansForSync = true
		}
	}

	if !foundBansForSync {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Your bans are already in sync",
			Err:     errors.New("nothing to sync"),
		})
		return
	}

	output := RefreshBansCommandConfirmationOutput{
		Bans:    len(uniqueBans),
		Servers: len(syncServers),
	}

	if _, ok := c.Config.Reactions["refreshbans"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing refreshbans reaction"),
		})
		return
	}

	reaction := c.Config.Reactions["refreshbans"]

	reactionModel := models.RefreshBansReaction{
		ServerBans: newServerBans,
		Reactions: []models.Reaction{
			{
				Name: reaction.Name,
				ID:   reaction.ID,
			},
		},
		User: &models.User{
			ID:   mc.Author.ID,
			Name: mc.Author.Username,
		},
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &output)

	embedParams := discordapi.EmbeddableParams{
		Title:        command.Name,
		Description:  fmt.Sprintf("Refreshing bans may take a while to process.\nPress the <%s> reaction to confirm the ban refresh.", reaction.FullEmoji),
		TitleURL:     c.Config.Bot.DocumentationURL,
		Footer:       fmt.Sprintf("Executed by %s", mc.Author.Username),
		ThumbnailURL: c.Config.Bot.OkThumbnail,
	}

	successMessages, sErr := c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)

	if sErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: sErr.Message,
			Err:     sErr.Err,
		})
		return
	}
	if len(successMessages) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to get output messages",
			Err:     errors.New("no messages in response"),
		})
		return
	}

	arErr := discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, reaction.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.RefreshBansReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.RefreshBansReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.BanReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert ban reaction TTL to int64",
			Err:     ttlErr,
		})
		return
	}

	c.MessagesAwaitingReaction.Messages[successMessages[0].ID] = reactions.MessageAwaitingReaction{
		Expires:     time.Now().Unix() + ttl,
		Reactions:   []string{reaction.ID},
		CommandName: command.Name,
		User:        mc.Author.ID,
	}

	return
}

// ConvertToEmbedField for RefreshBansCommandConfirmationOutput struct
func (bpc *RefreshBansCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := fmt.Sprintf("Refresh %d Bans Across %d Servers", bpc.Bans, bpc.Servers)
	fieldVal := fmt.Sprintf("Your cluster has %d banned players. These bans will be replicated across %d servers. Once confirmed, this process may take a few minutes.", bpc.Bans, bpc.Servers)

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for RefreshBansCommandInProgressOutput struct
func (bpc *RefreshBansCommandInProgressOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := "Starting Refresh Bans Process"
	fieldVal := fmt.Sprintf("Please wait while we analyze the bans across all %d of your servers.", bpc.ServerCount)

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
