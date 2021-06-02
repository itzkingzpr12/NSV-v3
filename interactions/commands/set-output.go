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

// SetOutputCommand struct
type SetOutputCommand struct {
	Params SetOutputCommandParams
}

// SetOutputCommandParams struct
type SetOutputCommandParams struct {
	ServerID  int64
	ChannelID string
}

// SetOutputCommandConfirmationOutput struct
type SetOutputCommandConfirmationOutput struct {
	CurrentAdminChannelID   string
	CurrentChatChannelID    string
	CurrentPlayersChannelID string
	CurrentKillsChannelID   string
	NewChannelID            string
}

// SetOutput func
func (c *Commands) SetOutput(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseSetOutputCommand(command, mc)
	if nscErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *nscErr)
		return
	}

	_, gcErr := discordapi.GetChannel(s, parsedCommand.Params.ChannelID)
	if gcErr != nil {
		if gcErr.Code == 10003 {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Unable to find channel",
				Err:     gcErr,
			})
			return
		}

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to access channel",
			Err:     gcErr,
		})
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

	var server gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		if parsedCommand.Params.ServerID == aServer.NitradoID {
			server = *aServer
			break
		}

		continue
	}

	if server.ID == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find server to set output for",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	if _, ok := c.Config.Reactions["set_output_admin"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing set_output_admin reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["set_output_chat"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing set_output_chat reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["set_output_kill"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing set_output_kill reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["set_output_players"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing set_output_players reaction"),
		})
		return
	}

	adminReaction := c.Config.Reactions["set_output_admin"]
	chatReaction := c.Config.Reactions["set_output_chat"]
	playersReaction := c.Config.Reactions["set_output_players"]
	killReaction := c.Config.Reactions["set_output_kill"]

	reactionModel := models.SetOutputReaction{
		Reactions: []models.Reaction{
			{
				Name: adminReaction.Name,
				ID:   adminReaction.ID,
			},
			{
				Name: chatReaction.Name,
				ID:   chatReaction.ID,
			},
			{
				Name: killReaction.Name,
				ID:   killReaction.ID,
			},
			{
				Name: playersReaction.Name,
				ID:   playersReaction.ID,
			},
		},
		User: &models.User{
			ID:   mc.Author.ID,
			Name: mc.Author.Username,
		},
		Server: models.Server{
			ID:        server.ID,
			NitradoID: server.NitradoID,
			Name:      server.Name,
		},
		NewChannel: models.Channel{
			ID: parsedCommand.Params.ChannelID,
		},
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	var setOutputOutput SetOutputCommandConfirmationOutput
	setOutputOutput.NewChannelID = parsedCommand.Params.ChannelID

	for _, channel := range server.ServerOutputChannels {
		if channel.OutputChannelType == nil {
			continue
		}

		switch channel.OutputChannelTypeID {
		case "admin":
			_, dcErr := discordapi.GetChannel(s, channel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			setOutputOutput.CurrentAdminChannelID = channel.ChannelID
			reactionModel.ServerOutputChannelIDAdmin = channel.ID
		case "chat":
			_, dcErr := discordapi.GetChannel(s, channel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			setOutputOutput.CurrentChatChannelID = channel.ChannelID
			reactionModel.ServerOutputChannelIDChat = channel.ID
		case "players":
			_, dcErr := discordapi.GetChannel(s, channel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			setOutputOutput.CurrentPlayersChannelID = channel.ChannelID
			reactionModel.ServerOutputChannelIDPlayers = channel.ID
		case "kills":
			_, dcErr := discordapi.GetChannel(s, channel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			setOutputOutput.CurrentKillsChannelID = channel.ChannelID
			reactionModel.ServerOutputChannelIDKills = channel.ID
		}
	}

	embeddableFields = append(embeddableFields, &setOutputOutput)

	embedParams := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("Setting Output for %s", server.Name),
		Description: fmt.Sprintf("Please press the relevant reaction to set the output channel type.\n\n<%s> **Admin Log**\n<%s> **Chat Log**\n<%s> **Kill Log**\n<%s> **Online Players**", adminReaction.FullEmoji, chatReaction.FullEmoji, killReaction.FullEmoji, playersReaction.FullEmoji),
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

	arErr := discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, adminReaction.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, chatReaction.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killReaction.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, playersReaction.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.SetOutputReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.SetOutputReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.SetOutputReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert set output reaction TTL to int64",
			Err:     ttlErr,
		})
		return
	}

	c.MessagesAwaitingReaction.Messages[successMessages[0].ID] = reactions.MessageAwaitingReaction{
		Expires: time.Now().Unix() + ttl,
		Reactions: []string{
			adminReaction.ID,
			chatReaction.ID,
			killReaction.ID,
			playersReaction.ID,
		},
		CommandName: command.Name,
		User:        mc.Author.ID,
	}

	return
}

// parseSetOutputCommand func
func parseSetOutputCommand(command configs.Command, mc *discordgo.MessageCreate) (*SetOutputCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		return nil, &Error{
			Message: "Invalid Server ID provided",
			Err:     errors.New("invalid server id"),
		}
	}

	start := strings.Index(splitContent[2], "<#")
	end := strings.Index(splitContent[2], ">")

	if start == -1 || end == -1 {
		return nil, &Error{
			Message: "Invalid channel format",
			Err:     errors.New("invalid channel"),
		}
	}

	channelID := splitContent[2][start+2 : end]

	if channelID == "" {
		return nil, &Error{
			Message: "Invalid channel format",
			Err:     errors.New("invalid channel"),
		}
	}

	return &SetOutputCommand{
		Params: SetOutputCommandParams{
			ServerID:  serverIDInt,
			ChannelID: channelID,
		},
	}, nil
}

// ConvertToEmbedField for SetOutputCommandConfirmationOutput struct
func (so *SetOutputCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	if so.CurrentAdminChannelID != "" {
		fieldVal += fmt.Sprintf("**Current Admin Channel:** <#%s>\n", so.CurrentAdminChannelID)
	} else {
		fieldVal += "**Current Admin Channel:** None\n"
	}

	if so.CurrentChatChannelID != "" {
		fieldVal += fmt.Sprintf("**Current Chat Channel:** <#%s>\n", so.CurrentChatChannelID)
	} else {
		fieldVal += "**Current Chat Channel:** None\n"
	}

	if so.CurrentKillsChannelID != "" {
		fieldVal += fmt.Sprintf("**Current Kills Channel:** <#%s>\n", so.CurrentKillsChannelID)
	} else {
		fieldVal += "**Current Kills Channel:** None\n"
	}

	if so.CurrentPlayersChannelID != "" {
		fieldVal += fmt.Sprintf("**Current Players Channel:** <#%s>\n", so.CurrentPlayersChannelID)
	} else {
		fieldVal += "**Current Players Channel:** None\n"
	}

	if so.NewChannelID != "" {
		fieldVal += fmt.Sprintf("\n\n**New Output Channel:** <#%s>\n", so.NewChannelID)
	}

	if fieldVal == "" {
		fieldVal = "No previous or new channel"
	}

	return &discordgo.MessageEmbedField{
		Name:   "Channels",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
