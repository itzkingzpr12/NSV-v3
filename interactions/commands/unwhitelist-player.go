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

// UnwhitelistPlayerCommand struct
type UnwhitelistPlayerCommand struct {
	Params UnwhitelistPlayerCommandParams
}

// UnwhitelistPlayerCommandParams struct
type UnwhitelistPlayerCommandParams struct {
	PlayerName string
	ServerID   int64
}

// UnwhitelistPlayerCommandConfirmationOutput struct
type UnwhitelistPlayerCommandConfirmationOutput struct {
	Servers    []gcscmodels.Server
	PlayerName string
}

// UnwhitelistPlayer func
func (c *Commands) UnwhitelistPlayer(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseUnwhitelistPlayerCommand(command, mc)
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

		if aServer.ServerTypeID != "arkps" {
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
			Message: "Unable to find PS servers to unwhitelist on",
			Err:     errors.New("invalid server id or no PS servers set up"),
		})
		return
	}

	if _, ok := c.Config.Reactions["unwhitelist"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing unwhitelist reaction"),
		})
		return
	}

	reaction := c.Config.Reactions["unwhitelist"]

	reactionModel := models.UnwhitelistReaction{
		PlayerName: parsedCommand.Params.PlayerName,
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

	embeddableFields = append(embeddableFields, &UnwhitelistPlayerCommandConfirmationOutput{
		Servers:    servers,
		PlayerName: parsedCommand.Params.PlayerName,
	})

	embedParams := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("Unwhitelist %s", parsedCommand.Params.PlayerName),
		Description: fmt.Sprintf("Unwhitelisting may take up to 5 minutes for Nitrado to process. Please press the <%s> reaction to confirm the unwhitelist.", reaction.FullEmoji),
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

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.UnwhitelistReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.UnwhitelistReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.UnwhitelistReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert unwhitelist reaction TTL to int64",
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

// parseUnwhitelistPlayerCommand func
func parseUnwhitelistPlayerCommand(command configs.Command, mc *discordgo.MessageCreate) (*UnwhitelistPlayerCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	accountName := ""
	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		accountName = strings.Join(splitContent[1:], " ")
	} else if len(splitContent) > 2 {
		accountName = strings.Join(splitContent[2:], " ")
	} else {
		return nil, &Error{
			Message: "Missing player account name",
			Err:     errors.New("no player account name"),
		}
	}

	return &UnwhitelistPlayerCommand{
		Params: UnwhitelistPlayerCommandParams{
			PlayerName: accountName,
			ServerID:   serverIDInt,
		},
	}, nil
}

// ConvertToEmbedField for UnwhitelistPlayerCommandConfirmationOutput struct
func (bpc *UnwhitelistPlayerCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := ""
	fieldVal := ""

	if len(bpc.Servers) == 1 {
		name = fmt.Sprintf("%s will be unwhitelisted on %s", bpc.PlayerName, bpc.Servers[0].Name)
	} else {
		name = fmt.Sprintf("%s will be unwhitelisted on %d servers", bpc.PlayerName, len(bpc.Servers))
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
