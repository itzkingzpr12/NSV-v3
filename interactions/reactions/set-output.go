package reactions

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// SetOutputSuccessOutput struct
type SetOutputSuccessOutput struct {
	NewChannelID string
	OldChannelID string
	ChannelType  string
}

// SetOutput func
func (r *Reactions) SetOutput(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var reactionModel *models.SetOutputReaction
	cacheKey := reactionModel.CacheKey(r.Config.CacheSettings.SetOutputReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &reactionModel)
	if cErr != nil {
		r.ErrorOutput(ctx, "Failed to set output", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if reactionModel == nil {
		r.ErrorOutput(ctx, "Failed to set output", mra.ChannelID, Error{
			Message: "set output message has expired",
			Err:     errors.New("please run the set output command again"),
		})
		return
	}

	var newOutputChannel *gcscmodels.ServerOutputChannel
	var oldOutputChannel *gcscmodels.ServerOutputChannel
	var channelType string
	switch mra.Emoji.ID {
	case r.Config.Reactions["set_output_admin"].ID:
		channelType = "Admin Log"
		if reactionModel.ServerOutputChannelIDAdmin != 0 {
			soc, socErr := guildconfigservice.GetServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDAdmin)
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to set get existing output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			oldOutputChannel = soc.ServerOutputChannel

			socUpdate, socUpdateErr := guildconfigservice.UpdateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDAdmin, reactionModel.NewChannel.ID, reactionModel.Server.ID, "admin")
			if socUpdateErr != nil {
				r.ErrorOutput(ctx, "Failed to update output channel", mra.ChannelID, Error{
					Message: socUpdateErr.Message,
					Err:     socUpdateErr.Err,
				})
				return
			}
			newOutputChannel = socUpdate.ServerOutputChannel
		} else {
			soc, socErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.NewChannel.ID, reactionModel.Server.ID, "admin")
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to add new output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			newOutputChannel = soc.ServerOutputChannel
		}
	case r.Config.Reactions["set_output_chat"].ID:
		channelType = "Chat Log"
		if reactionModel.ServerOutputChannelIDChat != 0 {
			soc, socErr := guildconfigservice.GetServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDChat)
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to set get existing output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			oldOutputChannel = soc.ServerOutputChannel

			socUpdate, socUpdateErr := guildconfigservice.UpdateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDChat, reactionModel.NewChannel.ID, reactionModel.Server.ID, "chat")
			if socUpdateErr != nil {
				r.ErrorOutput(ctx, "Failed to update output channel", mra.ChannelID, Error{
					Message: socUpdateErr.Message,
					Err:     socUpdateErr.Err,
				})
				return
			}
			newOutputChannel = socUpdate.ServerOutputChannel
		} else {
			soc, socErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.NewChannel.ID, reactionModel.Server.ID, "chat")
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to add new output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			newOutputChannel = soc.ServerOutputChannel
		}
	case r.Config.Reactions["set_output_players"].ID:
		channelType = "Online Players"
		if reactionModel.ServerOutputChannelIDPlayers != 0 {
			soc, socErr := guildconfigservice.GetServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDPlayers)
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to set get existing output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			oldOutputChannel = soc.ServerOutputChannel

			socUpdate, socUpdateErr := guildconfigservice.UpdateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.ServerOutputChannelIDPlayers, reactionModel.NewChannel.ID, reactionModel.Server.ID, "players")
			if socUpdateErr != nil {
				r.ErrorOutput(ctx, "Failed to update output channel", mra.ChannelID, Error{
					Message: socUpdateErr.Message,
					Err:     socUpdateErr.Err,
				})
				return
			}
			newOutputChannel = socUpdate.ServerOutputChannel
		} else {
			soc, socErr := guildconfigservice.CreateServerOutputChannel(ctx, r.GuildConfigService, mra.GuildID, reactionModel.NewChannel.ID, reactionModel.Server.ID, "players")
			if socErr != nil {
				r.ErrorOutput(ctx, "Failed to add new output channel", mra.ChannelID, Error{
					Message: socErr.Message,
					Err:     socErr.Err,
				})
				return
			}
			newOutputChannel = soc.ServerOutputChannel
		}
	default:
		r.ErrorOutput(ctx, "Invalid reaction", mra.ChannelID, Error{
			Message: "unknown reaction used",
			Err:     errors.New("invalid reaction"),
		})
		return
	}

	if newOutputChannel == nil {
		r.ErrorOutput(ctx, "Failed to set output", mra.ChannelID, Error{
			Message: "no output channel in response",
			Err:     errors.New("nil output channel"),
		})
		return
	}

	successOutput := SetOutputSuccessOutput{
		NewChannelID: newOutputChannel.ChannelID,
	}

	if oldOutputChannel != nil {
		successOutput.OldChannelID = oldOutputChannel.ChannelID
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &successOutput)

	editedCommand := command
	editedCommand.Name = fmt.Sprintf("Set %s Output for: %s", channelType, reactionModel.Server.Name)
	editedCommand.Description = "Output has been set. Please give the bot a few minutes to output to this new channel."

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for SetOutputSuccessOutput struct
func (out *SetOutputSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	if out.OldChannelID == "" {
		fieldVal += fmt.Sprintf("Old %s Channel: None\n", out.ChannelType)
	} else {
		fieldVal += fmt.Sprintf("Old %s Channel: <#%s>\n", out.ChannelType, out.OldChannelID)
	}

	fieldVal += fmt.Sprintf("New %s Channel: <#%s>\n", out.ChannelType, out.NewChannelID)

	if fieldVal == "" {
		fieldVal += "No changes made"
	}

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("%s Channel Updated", out.ChannelType),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
