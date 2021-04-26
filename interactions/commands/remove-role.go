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

// RemoveRoleCommand struct
type RemoveRoleCommand struct {
	Params RemoveRoleCommandParams
}

// RemoveRoleCommandParams struct
type RemoveRoleCommandParams struct {
	RoleID   string
	Commands []string
}

// RemoveRoleCommandConfirmationSuccessOutput struct
type RemoveRoleCommandConfirmationSuccessOutput struct {
	Commands []string
}

// RemoveRoleCommandConfirmationDoesntExistsOutput struct
type RemoveRoleCommandConfirmationDoesntExistOutput struct {
	Commands []string
}

// RemoveRoleCommandConfirmationErrorOutput struct
type RemoveRoleCommandConfirmationErrorOutput struct {
	Commands []string
}

// RemoveRole func
func (c *Commands) RemoveRole(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseRemoveRoleCommand(command, mc)
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

	if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, c.Config.Bot.GuildService, "GuildServices"); vErr != nil {
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

	var guildService *gcscmodels.GuildService
	for _, aGuildService := range guildFeed.Payload.Guild.GuildServices {
		if aGuildService.Name == c.Config.Bot.GuildService {
			guildService = aGuildService
			break
		}
	}

	if guildService == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "No guild service found to remove roles from",
			Err:     errors.New("no guild service"),
		})
		return
	}

	var removeRoleOutput RemoveRoleCommandConfirmationSuccessOutput
	var removeRoleDoesntExistOutput RemoveRoleCommandConfirmationDoesntExistOutput
	var removeRoleErrorOutput RemoveRoleCommandConfirmationErrorOutput
	var validCommands []models.Command
	var invalidCommands []string

	for _, possibleCommand := range parsedCommand.Params.Commands {
		foundCommand := false
		for _, command := range c.Config.Commands {
			if possibleCommand == command.Long || possibleCommand == command.Short {
				validCommands = append(validCommands, models.Command{
					Name: command.Name,
				})
				foundCommand = true
				break
			}
		}

		if !foundCommand {
			invalidCommands = append(invalidCommands, possibleCommand)
			removeRoleErrorOutput.Commands = append(removeRoleErrorOutput.Commands, possibleCommand)
		}
	}

	if len(validCommands) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "No valid commands provided in command",
			Err:     errors.New("no valid commands"),
		})
		return
	}

	var guildServicePermissions []gcscmodels.GuildServicePermission
	for _, cmd := range validCommands {
		if guildService.GuildServicePermissions != nil {
			foundPermission := false
			for _, permission := range guildService.GuildServicePermissions {
				if permission.CommandName == cmd.Name && permission.RoleID == parsedCommand.Params.RoleID {
					var aPermission gcscmodels.GuildServicePermission = *permission
					removeRoleOutput.Commands = append(removeRoleOutput.Commands, cmd.Name)
					guildServicePermissions = append(guildServicePermissions, aPermission)
					foundPermission = true
					break
				}
			}

			if !foundPermission {
				removeRoleDoesntExistOutput.Commands = append(removeRoleDoesntExistOutput.Commands, cmd.Name)
			}
		} else {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Role does not have any permissions",
				Err:     errors.New("nil guild service permissions"),
			})
			return
		}
	}

	reaction := c.Config.Reactions["remove_role"]

	reactionModel := models.RemoveRoleReaction{
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
		GuildServicePermissions: guildServicePermissions,
		RoleID:                  parsedCommand.Params.RoleID,
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(removeRoleOutput.Commands) > 0 {
		embeddableFields = append(embeddableFields, &removeRoleOutput)
	}

	if len(removeRoleDoesntExistOutput.Commands) > 0 {
		embeddableFields = append(embeddableFields, &removeRoleDoesntExistOutput)
	}

	if len(removeRoleErrorOutput.Commands) > 0 {
		embeddableErrors = append(embeddableErrors, &removeRoleErrorOutput)
	}

	embedParams := discordapi.EmbeddableParams{
		Title:       "Removing Role Access",
		Description: fmt.Sprintf("Please press the <%s> reaction to remove the <@&%s> role access to commands.", reaction.FullEmoji, parsedCommand.Params.RoleID),
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

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.RemoveRoleReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.RemoveRoleReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.RemoveRoleReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert remove role reaction TTL to int64",
			Err:     ttlErr,
		})
		return
	}

	c.MessagesAwaitingReaction.Messages[successMessages[0].ID] = reactions.MessageAwaitingReaction{
		Expires: time.Now().Unix() + ttl,
		Reactions: []string{
			reaction.ID,
		},
		CommandName: command.Name,
		User:        mc.Author.ID,
	}

	return
}

// parseRemoveRoleCommand func
func parseRemoveRoleCommand(command configs.Command, mc *discordgo.MessageCreate) (*RemoveRoleCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	start := strings.Index(splitContent[1], "<@&")
	end := strings.Index(splitContent[1], ">")

	if start == -1 || end == -1 {
		return nil, &Error{
			Message: "Invalid role in command",
			Err:     errors.New("invalid role"),
		}
	}

	roleID := splitContent[1][start+3 : end]
	if roleID == "" {
		return nil, &Error{
			Message: "Invalid role in command",
			Err:     errors.New("empty role"),
		}
	}

	return &RemoveRoleCommand{
		Params: RemoveRoleCommandParams{
			RoleID:   roleID,
			Commands: splitContent[2:],
		},
	}, nil
}

// ConvertToEmbedField for RemoveRoleCommandConfirmationOutput struct
func (so *RemoveRoleCommandConfirmationSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, command := range so.Commands {
		fieldVal += "\n" + command
	}

	if fieldVal == "```" {
		fieldVal += "\nNone"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Role Will Lose Access To",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for RemoveRoleCommandConfirmationErrorOutput struct
func (so *RemoveRoleCommandConfirmationErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, command := range so.Commands {
		fieldVal += "\n" + command
	}

	if fieldVal == "```" {
		fieldVal += "\nNone"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Unknown Commands",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for RemoveRoleCommandConfirmationOutput struct
func (so *RemoveRoleCommandConfirmationDoesntExistOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"

	for _, command := range so.Commands {
		fieldVal += "\n" + command
	}

	if fieldVal == "```" {
		fieldVal += "\nNone"
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   "Role Doesn't Have Access To",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
