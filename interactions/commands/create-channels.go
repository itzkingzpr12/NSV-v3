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

// CreateChannelsCommand struct
type CreateChannelsCommand struct {
	Params CreateChannelsCommandParams
}

// CreateChannelsCommandParams struct
type CreateChannelsCommandParams struct {
	Name     string
	ServerID int64
}

// CreateChannelsCommandConfirmationOutput struct
type CreateChannelsCommandConfirmationOutput struct {
	ExistingChannels []gcscmodels.ServerOutputChannel
	NewChannels      []string
}

// CreateChannels func
func (c *Commands) CreateChannels(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseCreateChannelsCommand(command, mc)
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
			Message: "Unable to find server to create channels for",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	if _, ok := c.Config.Reactions["create_channels"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing create_channels reaction"),
		})
		return
	}

	reaction := c.Config.Reactions["create_channels"]

	reactionModel := models.CreateChannelsReaction{
		Name: parsedCommand.Params.Name,
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
		Server: models.Server{
			ID:        server.ID,
			NitradoID: server.NitradoID,
			Name:      server.Name,
		},
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	var createChannelOutput CreateChannelsCommandConfirmationOutput

	foundAdmin := false
	foundChat := false
	foundPlayers := false

	var parentID string = ""

	for _, channel := range server.ServerOutputChannels {
		if channel.OutputChannelType == nil {
			continue
		}

		var aChannel = *channel

		switch channel.OutputChannelTypeID {
		case "admin":
			dcChan, dcErr := discordapi.GetChannel(s, aChannel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			if dcChan.ParentID != "" {
				parentID = dcChan.ParentID
			}

			createChannelOutput.ExistingChannels = append(createChannelOutput.ExistingChannels, aChannel)
			foundAdmin = true
		case "chat":
			dcChan, dcErr := discordapi.GetChannel(s, aChannel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			if dcChan.ParentID != "" {
				parentID = dcChan.ParentID
			}

			createChannelOutput.ExistingChannels = append(createChannelOutput.ExistingChannels, aChannel)
			foundChat = true
		case "players":
			dcChan, dcErr := discordapi.GetChannel(s, aChannel.ChannelID)
			if dcErr != nil {
				if dcErr.Code == 10003 {
					guildconfigservice.DeleteServerOutputChannel(ctx, c.GuildConfigService, mc.GuildID, int64(channel.ID))
				}
				break
			}

			if dcChan.ParentID != "" {
				parentID = dcChan.ParentID
			}

			createChannelOutput.ExistingChannels = append(createChannelOutput.ExistingChannels, aChannel)
			foundPlayers = true
		}
	}

	reactionModel.Category = models.Channel{
		ID: parentID,
	}

	if !foundAdmin {
		createChannelOutput.NewChannels = append(createChannelOutput.NewChannels, fmt.Sprintf("admin-log-%s", parsedCommand.Params.Name))
		reactionModel.AdminChannelName = fmt.Sprintf("admin-log-%s", parsedCommand.Params.Name)
	}

	if !foundChat {
		createChannelOutput.NewChannels = append(createChannelOutput.NewChannels, fmt.Sprintf("chat-log-%s", parsedCommand.Params.Name))
		reactionModel.ChatChannelName = fmt.Sprintf("chat-log-%s", parsedCommand.Params.Name)
	}

	if !foundPlayers {
		createChannelOutput.NewChannels = append(createChannelOutput.NewChannels, fmt.Sprintf("online-players-%s", parsedCommand.Params.Name))
		reactionModel.PlayersChannelName = fmt.Sprintf("online-players-%s", parsedCommand.Params.Name)
	}

	if len(createChannelOutput.ExistingChannels) == 0 && len(createChannelOutput.NewChannels) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to identify channels",
			Err:     errors.New("no channels"),
		})
		return
	}

	embeddableFields = append(embeddableFields, &createChannelOutput)

	embedParams := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("Creating Channels for %s", server.Name),
		Description: fmt.Sprintf("Please press the <%s> reaction to confirm creating new channels. Existing channels will be ignored.", reaction.FullEmoji),
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

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.CreateChannelsReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.CreateChannelsReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.CreateChannelsReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert create channels reaction TTL to int64",
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

// parseCreateChannelsCommand func
func parseCreateChannelsCommand(command configs.Command, mc *discordgo.MessageCreate) (*CreateChannelsCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	name := ""
	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		return nil, &Error{
			Message: "Invalid Server ID provided",
			Err:     errors.New("invalid server id"),
		}
	} else if len(splitContent) == 2 {
		name = splitContent[1]
	} else {
		name = strings.Join(splitContent[2:], "-")
	}

	return &CreateChannelsCommand{
		Params: CreateChannelsCommandParams{
			Name:     name,
			ServerID: serverIDInt,
		},
	}, nil
}

// ConvertToEmbedField for CreateChannelsCommandConfirmationOutput struct
func (so *CreateChannelsCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := "Channels"
	fieldVal := ""

	if len(so.NewChannels) > 0 {
		fieldVal += "**New Channels:**\n"
	}
	for _, channel := range so.NewChannels {
		fieldVal += fmt.Sprintf("#%s\n", channel)
	}

	if fieldVal != "" {
		fieldVal += "\n"
	}

	if len(so.ExistingChannels) > 0 {
		fieldVal += "**Existing Channels:**\n"
	}
	for _, channel := range so.ExistingChannels {
		switch channel.OutputChannelTypeID {
		case "admin":
			fieldVal += fmt.Sprintf("Admin Logs: <#%s>\n", channel.ChannelID)
		case "chat":
			fieldVal += fmt.Sprintf("Chat Logs: <#%s>\n", channel.ChannelID)
		case "players":
			fieldVal += fmt.Sprintf("Online Players: <#%s>\n", channel.ChannelID)
		}
	}

	if fieldVal == "" {
		fieldVal = "No channels found or to be created"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
