package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/reactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// ClearWhitelistCommand struct
type ClearWhitelistCommand struct {
	Params ClearWhitelistCommandParams
}

// ClearWhitelistCommandParams struct
type ClearWhitelistCommandParams struct {
	ServerID int64
}

// ClearWhitelistCommandConfirmationOutput struct
type ClearWhitelistCommandConfirmationOutput struct {
	Servers []gcscmodels.Server
}

// ClearWhitelist func
func (c *Commands) ClearWhitelist(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseClearWhitelistCommand(command, mc)
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
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		if parsedCommand.Params.ServerID != 0 {
			if parsedCommand.Params.ServerID == aServer.NitradoID {
				servers = append(servers, *aServer)
				break
			}
			continue
		}

		servers = append(servers, *aServer)
	}

	if len(servers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find servers to clear whitelist on",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	if _, ok := c.Config.Reactions["clear_whitelist"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing clear whitelist reaction"),
		})
		return
	}

	reaction := c.Config.Reactions["clear_whitelist"]

	reactionModel := models.ClearWhitelistReaction{
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

	for _, aServer := range servers {
		reactionModel.Servers = append(reactionModel.Servers, models.Server{
			ID: aServer.ID,
		})
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &ClearWhitelistCommandConfirmationOutput{
		Servers: servers,
	})

	embedParams := discordapi.EmbeddableParams{
		Title:       command.Name,
		Description: fmt.Sprintf("Clearing Whitelist may take up to 30 minutes for Nitrado to process. Please press the <%s> reaction to confirm clearing the whitelist.", reaction.FullEmoji),
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	if len(embeddableErrors) == 0 {
		embedParams.ThumbnailURL = c.Config.Bot.WorkingThumbnail
	} else {
		embedParams.ThumbnailURL = c.Config.Bot.WarnThumbnail
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

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.ClearWhitelistReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.ClearWhitelistReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.ClearWhitelistReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert clear whitelist reaction TTL to int64",
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

// parseClearWhitelistCommand func
func parseClearWhitelistCommand(command configs.Command, mc *discordgo.MessageCreate) (*ClearWhitelistCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	if len(splitContent) >= 2 {
		serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
		if sidErr != nil {
			return nil, &Error{
				Message: "Invalid server ID provided",
				Err:     errors.New("invalid server id"),
			}
		}
		return &ClearWhitelistCommand{
			Params: ClearWhitelistCommandParams{
				ServerID: serverIDInt,
			},
		}, nil
	}

	return &ClearWhitelistCommand{
		Params: ClearWhitelistCommandParams{},
	}, nil
}

// ConvertToEmbedField for ClearWhitelistCommandConfirmationOutput struct
func (bpc *ClearWhitelistCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := ""
	fieldVal := ""

	if len(bpc.Servers) == 1 {
		name = fmt.Sprintf("Clearing whitelist on %s", bpc.Servers[0].Name)
	} else {
		name = fmt.Sprintf("Clearing whitelist on %d servers", len(bpc.Servers))
	}

	if fieldVal == "" {
		fieldVal = "\u200b"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
