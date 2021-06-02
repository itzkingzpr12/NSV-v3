package reactions

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// CreateChannelsSuccessOutput struct
type CreateChannelsSuccessOutput struct {
	Name     string
	Channels []*discordgo.Channel
}

// CreateChannelsErrorOutput struct
type CreateChannelsErrorOutput struct {
	ChannelNames []string
}

// CreateChannels func
func (r *Reactions) CreateChannels(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var ccr *models.CreateChannelsReaction
	cacheKey := ccr.CacheKey(r.Config.CacheSettings.CreateChannelsReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &ccr)
	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to create channels", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if ccr == nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "create channels reaction has expired"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		r.ErrorOutput(ctx, "Failed to create channels", mra.ChannelID, Error{
			Message: "create channels message has expired",
			Err:     errors.New("please run the create channels command again"),
		})
		return
	}

	if ccr.AdminChannelName == "" && ccr.ChatChannelName == "" && ccr.PlayersChannelName == "" && ccr.KillsChannelName == "" {
		r.ErrorOutput(ctx, "No channels to create", mra.ChannelID, Error{
			Message: "All channels already created",
			Err:     errors.New("please check your server listing to see channels"),
		})
		return
	}

	successOutput := CreateChannelsSuccessOutput{
		Name: ccr.Name,
	}

	errorOutput := CreateChannelsErrorOutput{}

	var parentCategory *discordgo.Channel
	if ccr.Category.ID != "" {
		category, catErr := discordapi.GetChannel(s, ccr.Category.ID)
		if catErr != nil {
			newCTX := logging.AddValues(ctx, zap.NamedError("error", catErr), zap.String("error_message", catErr.Message))
			logger := logging.Logger(newCTX)
			logger.Error("error_log")
		} else {
			parentCategory = category
		}
	} else {
		category, catErr := discordapi.CreateChannel(s, mra.GuildID, discordgo.GuildChannelCreateData{
			Name:     "Server Logs: " + ccr.Name,
			Type:     discordgo.ChannelTypeGuildCategory,
			ParentID: ccr.Category.ID,
			PermissionOverwrites: []*discordgo.PermissionOverwrite{
				{
					ID:   mra.GuildID,
					Type: discordgo.PermissionOverwriteTypeRole,
					Deny: discordgo.PermissionViewChannel,
				},
			},
		})
		if catErr != nil {
			r.ErrorOutput(ctx, "Failed to create category for channels", mra.ChannelID, Error{
				Message: catErr.Message,
				Err:     catErr,
			})
			return
		} else {
			parentCategory = category
		}
	}

	if ccr.AdminChannelName != "" {
		channelData := discordgo.GuildChannelCreateData{
			Name:     ccr.AdminChannelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: parentCategory.ID,
		}

		if parentCategory != nil {
			channelData.PermissionOverwrites = parentCategory.PermissionOverwrites
		}

		newChannel, ncErr := discordapi.CreateChannel(s, mra.GuildID, channelData)
		if ncErr != nil {
			newCTX := logging.AddValues(ctx, zap.NamedError("error", ncErr), zap.String("error_message", ncErr.Message))
			logger := logging.Logger(newCTX)
			logger.Error("error_log")

			errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.AdminChannelName)
		} else {
			_, csocErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, newChannel.ID, ccr.Server.ID, "admin")
			if csocErr != nil {
				newCTX := logging.AddValues(ctx, zap.NamedError("error", csocErr), zap.String("error_message", csocErr.Message))
				logger := logging.Logger(newCTX)
				logger.Error("error_log")

				errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.AdminChannelName)
			} else {
				successOutput.Channels = append(successOutput.Channels, newChannel)
			}
		}
	}

	if ccr.ChatChannelName != "" {
		channelData := discordgo.GuildChannelCreateData{
			Name:     ccr.ChatChannelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: parentCategory.ID,
		}

		if parentCategory != nil {
			channelData.PermissionOverwrites = parentCategory.PermissionOverwrites
		}

		newChannel, ncErr := discordapi.CreateChannel(s, mra.GuildID, channelData)
		if ncErr != nil {
			newCTX := logging.AddValues(ctx, zap.NamedError("error", ncErr), zap.String("error_message", ncErr.Message))
			logger := logging.Logger(newCTX)
			logger.Error("error_log")

			errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.ChatChannelName)
		} else {
			_, csocErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, newChannel.ID, ccr.Server.ID, "chat")
			if csocErr != nil {
				newCTX := logging.AddValues(ctx, zap.NamedError("error", csocErr), zap.String("error_message", csocErr.Message))
				logger := logging.Logger(newCTX)
				logger.Error("error_log")

				errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.ChatChannelName)
			} else {
				successOutput.Channels = append(successOutput.Channels, newChannel)
			}
		}
	}

	if ccr.KillsChannelName != "" {
		channelData := discordgo.GuildChannelCreateData{
			Name:     ccr.KillsChannelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: parentCategory.ID,
		}

		if parentCategory != nil {
			channelData.PermissionOverwrites = parentCategory.PermissionOverwrites
		}

		newChannel, ncErr := discordapi.CreateChannel(s, mra.GuildID, channelData)
		if ncErr != nil {
			newCTX := logging.AddValues(ctx, zap.NamedError("error", ncErr), zap.String("error_message", ncErr.Message))
			logger := logging.Logger(newCTX)
			logger.Error("error_log")

			errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.KillsChannelName)
		} else {
			_, csocErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, newChannel.ID, ccr.Server.ID, "kills")
			if csocErr != nil {
				newCTX := logging.AddValues(ctx, zap.NamedError("error", csocErr), zap.String("error_message", csocErr.Message))
				logger := logging.Logger(newCTX)
				logger.Error("error_log")

				errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.KillsChannelName)
			} else {
				successOutput.Channels = append(successOutput.Channels, newChannel)
			}
		}
	}

	if ccr.PlayersChannelName != "" {
		channelData := discordgo.GuildChannelCreateData{
			Name:     ccr.PlayersChannelName,
			Type:     discordgo.ChannelTypeGuildText,
			ParentID: parentCategory.ID,
		}

		if parentCategory != nil {
			channelData.PermissionOverwrites = parentCategory.PermissionOverwrites
		}

		newChannel, ncErr := discordapi.CreateChannel(s, mra.GuildID, channelData)
		if ncErr != nil {
			newCTX := logging.AddValues(ctx, zap.NamedError("error", ncErr), zap.String("error_message", ncErr.Message))
			logger := logging.Logger(newCTX)
			logger.Error("error_log")

			errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.PlayersChannelName)
		} else {
			_, csocErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, newChannel.ID, ccr.Server.ID, "players")
			if csocErr != nil {
				newCTX := logging.AddValues(ctx, zap.NamedError("error", csocErr), zap.String("error_message", csocErr.Message))
				logger := logging.Logger(newCTX)
				logger.Error("error_log")

				errorOutput.ChannelNames = append(errorOutput.ChannelNames, ccr.PlayersChannelName)
			} else {
				successOutput.Channels = append(successOutput.Channels, newChannel)
			}
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(successOutput.Channels) > 0 {
		embeddableFields = append(embeddableFields, &successOutput)
	}

	if len(errorOutput.ChannelNames) > 0 {
		embeddableErrors = append(embeddableErrors, &errorOutput)
	}

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Created Channels for: %s", ccr.Server.Name)
	editedCommand.Description = "Channels have been created and linked for their corresponding outputs. Please give the bot a few minutes to output to these channels."

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for CreateChannelsSuccessOutput struct
func (out *CreateChannelsSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	for _, channel := range out.Channels {
		fieldVal += fmt.Sprintf("<#%s>\n", channel.ID)
	}

	if fieldVal == "" {
		fieldVal += "No channels created"
	}

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%d Channel(s) Created for: %s", len(out.Channels), out.Name),
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for CreateChannelsErrorOutput struct
func (out *CreateChannelsErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, name := range out.ChannelNames {
		fieldVal += "\n" + name
	}

	if fieldVal == "```" {
		fieldVal += "\nUnknown"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%d Channels Failed to Create", len(out.ChannelNames)),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
