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

// AddRoleSuccessOutput struct
type AddRoleSuccessOutput struct {
	Commands []string
}

// AddRoleErrorOutput struct
type AddRoleErrorOutput struct {
	Commands []string
}

// AddRole func
func (r *Reactions) AddRole(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var reactionModel *models.AddRoleReaction
	cacheKey := reactionModel.CacheKey(r.Config.CacheSettings.AddRoleReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &reactionModel)
	if cErr != nil {
		r.ErrorOutput(ctx, "Failed to add role", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if reactionModel == nil {
		r.ErrorOutput(ctx, "Failed to add role", mra.ChannelID, Error{
			Message: "add role message has expired",
			Err:     errors.New("please run the add role command again"),
		})
		return
	}

	var successOutput AddRoleSuccessOutput
	var errorOutput AddRoleErrorOutput

	for _, cmd := range reactionModel.Commands {
		_, gspErr := guildconfigservice.CreateGuildServicePermission(ctx, r.GuildConfigService, mra.GuildID, reactionModel.GuildServiceID, reactionModel.RoleID, cmd.Name)
		if gspErr != nil {
			newCtx := logging.AddValues(ctx,
				zap.NamedError("error", gspErr),
				zap.String("error_message", gspErr.Message),
				zap.String("role_id", reactionModel.RoleID),
				zap.String("permission_command", cmd.Name),
			)
			logger := logging.Logger(newCtx)
			logger.Error("error_log")

			errorOutput.Commands = append(errorOutput.Commands, cmd.Name)
		} else {
			successOutput.Commands = append(successOutput.Commands, cmd.Name)
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
	editedCommand.Name = "Added Role Access"
	editedCommand.Description = fmt.Sprintf("<@&%s> has been provided access to run commands for this bot.", reactionModel.RoleID)

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for AddRoleSuccessOutput struct
func (out *AddRoleSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, cmd := range out.Commands {
		fieldVal += "\n" + cmd
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Commands"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Command(s) Now Accessible",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for AddRoleErrorOutput struct
func (out *AddRoleErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, cmd := range out.Commands {
		fieldVal += "\n" + cmd
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Commands"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Failed To Give Access To Command(s)",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
