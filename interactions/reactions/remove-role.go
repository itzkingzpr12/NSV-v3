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

// RemoveRoleSuccessOutput struct
type RemoveRoleSuccessOutput struct {
	Commands []string
}

// RemoveRoleErrorOutput struct
type RemoveRoleErrorOutput struct {
	Commands []string
}

// RemoveRole func
func (r *Reactions) RemoveRole(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var reactionModel *models.RemoveRoleReaction
	cacheKey := reactionModel.CacheKey(r.Config.CacheSettings.RemoveRoleReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &reactionModel)
	if cErr != nil {
		r.ErrorOutput(ctx, "Failed to remove role", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if reactionModel == nil {
		r.ErrorOutput(ctx, "Failed to remove role", mra.ChannelID, Error{
			Message: "remove role message has expired",
			Err:     errors.New("please run the remove role command again"),
		})
		return
	}

	var successOutput RemoveRoleSuccessOutput
	var errorOutput RemoveRoleErrorOutput

	for _, perm := range reactionModel.GuildServicePermissions {
		_, gspErr := guildconfigservice.DeleteGuildServicePermission(ctx, r.GuildConfigService, mra.GuildID, int64(perm.ID))
		if gspErr != nil {
			newCtx := logging.AddValues(ctx,
				zap.NamedError("error", gspErr),
				zap.String("error_message", gspErr.Message),
				zap.String("role_id", reactionModel.RoleID),
				zap.String("permission_command", perm.CommandName),
			)
			logger := logging.Logger(newCtx)
			logger.Error("error_log")

			errorOutput.Commands = append(errorOutput.Commands, perm.CommandName)
		} else {
			successOutput.Commands = append(successOutput.Commands, perm.CommandName)
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(successOutput.Commands) > 0 {
		embeddableFields = append(embeddableFields, &successOutput)
	}

	if len(errorOutput.Commands) > 0 {
		embeddableErrors = append(embeddableErrors, &errorOutput)
	}

	editedCommand := command
	editedCommand.Name = "Removed Role Access"
	editedCommand.Description = fmt.Sprintf("<@&%s> no longer has access to run specified commands for this bot.", reactionModel.RoleID)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for RemoveRoleSuccessOutput struct
func (out *RemoveRoleSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, cmd := range out.Commands {
		fieldVal += "\n" + cmd
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Commands"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Command(s) No Longer Accessible",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for RemoveRoleErrorOutput struct
func (out *RemoveRoleErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, cmd := range out.Commands {
		fieldVal += "\n" + cmd
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Commands"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Failed To Remove Access To Command(s)",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
