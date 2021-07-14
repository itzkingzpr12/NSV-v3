package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// HelpCommand struct
type HelpCommand struct {
	Params HelpCommandParams
}

// HelpCommandParams struct
type HelpCommandParams struct {
	CategoryName string
}

// HelpOutput struct
type HelpOutput struct {
	Command configs.Command                     `json:"command"`
	Prefix  string                              `json:"prefix"`
	Roles   []gcscmodels.GuildServicePermission `json:"roles"`
}

// HelpCategoryOutput struct
type HelpCategoryOutput struct {
	CategoryName      string
	CategoryShortName string
	Prefix            string
	HelpCommand       configs.Command
}

// Help func
func (c *Commands) Help(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, err := parseHelpCommand(command, mc)
	if err != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *err)
		return
	}

	var permissions []*gcscmodels.GuildServicePermission
	guildFeed, gfErr := guildconfigservice.GetGuildFeed(ctx, c.GuildConfigService, mc.GuildID)
	if gfErr == nil {
		if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, c.Config.Bot.GuildService, "GuildServices"); vErr == nil {
			for _, service := range guildFeed.Payload.Guild.GuildServices {
				if service.Name == c.Config.Bot.GuildService {
					permissions = service.GuildServicePermissions
					break
				}
			}
		}
	}

	var categories map[string][]configs.Command = make(map[string][]configs.Command, 0)
	for _, command := range c.Config.Commands {
		if !command.Enabled {
			continue
		}

		if command.Name == "Help" {
			continue
		}

		if val, ok := categories[command.CategoryShort]; ok {
			val = append(val, command)
			categories[command.CategoryShort] = val
		} else {
			categories[command.CategoryShort] = []configs.Command{
				command,
			}
		}
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if parsedCommand.Params.CategoryName == "" {
		var keys []string
		for key := range categories {
			keys = append(keys, key)
		}

		sort.SliceStable(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})

		for _, key := range keys {
			if val, ok := categories[key]; ok {
				if len(val) == 0 {
					continue
				}

				embeddableFields = append(embeddableFields, &HelpCategoryOutput{
					CategoryName:      val[0].Category,
					CategoryShortName: val[0].CategoryShort,
					Prefix:            c.Config.Bot.Prefix,
					HelpCommand:       command,
				})
			}
		}

		embedParams := discordapi.EmbeddableParams{
			Title:       command.Name,
			Description: "Categories of commands that can be used with this bot. Please identify which category you are looking for help information on and use the listed command to retrieve it.",
			TitleURL:    c.Config.Bot.DocumentationURL,
			Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
		}

		c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)
		return
	}

	if _, ok := categories[parsedCommand.Params.CategoryName]; !ok {
		c.ErrorOutput(ctx, command, "Failed to find Help info", mc.ChannelID, Error{
			Message: fmt.Sprintf("No Command Category: %s", parsedCommand.Params.CategoryName),
			Err:     errors.New("invalid help category name"),
		})
		return
	}

	fullCategoryName := ""
	for _, aCommand := range categories[parsedCommand.Params.CategoryName] {
		if aCommand.Name == "Help" {
			continue
		}

		if !aCommand.Enabled {
			continue
		}

		helpOutput := &HelpOutput{
			Command: aCommand,
			Prefix:  c.Config.Bot.Prefix,
		}

		if fullCategoryName == "" {
			fullCategoryName = aCommand.Category
		}

		if permissions != nil {
			for _, permission := range permissions {
				if permission.CommandName == aCommand.Name {
					aPermission := *permission
					helpOutput.Roles = append(helpOutput.Roles, aPermission)
				}
			}
		}
		embeddableFields = append(embeddableFields, helpOutput)
	}

	embedParams := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("%s: %s", command.Name, fullCategoryName),
		Description: command.Description,
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)
	return
}

// parseHelpCommand func
func parseHelpCommand(command configs.Command, mc *discordgo.MessageCreate) (*HelpCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	if len(splitContent) < 2 {
		return &HelpCommand{
			Params: HelpCommandParams{},
		}, nil
	} else {
		return &HelpCommand{
			Params: HelpCommandParams{
				CategoryName: strings.ToLower(splitContent[1]),
			},
		}, nil
	}
}

// ConvertToEmbedField for Help struct
func (h *HelpOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	usages := ""
	for _, usage := range h.Command.Usage {
		if usages == "" {
			usages = h.Prefix + usage
		} else {
			usages += "\n" + h.Prefix + usage
		}
	}
	examples := ""
	for _, example := range h.Command.Examples {
		if examples == "" {
			examples = h.Prefix + example
		} else {
			examples += "\n" + h.Prefix + example
		}
	}

	roles := ""
	for _, role := range h.Roles {
		if !role.Enabled {
			continue
		}

		if roles == "" {
			roles = fmt.Sprintf("<@&%s>", role.RoleID)
		} else {
			roles += fmt.Sprintf(" <@&%s>", role.RoleID)
		}
	}

	value := ""
	if roles == "" {
		value = fmt.Sprintf("%s\n**USAGE:**\n```\n%s\n```\n**EXAMPLES:**\n```\n%s\n```\n\u200b", h.Command.Description, usages, examples)
	} else {
		value = fmt.Sprintf("%s\n**USAGE:**\n```\n%s\n```\n**EXAMPLES:**\n```\n%s\n```\n**ROLES:** %s\n\u200b", h.Command.Description, usages, examples, roles)
	}

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("**__%s__**", h.Command.Name),
		Value:  value,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for Help struct
func (h *HelpCategoryOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("**%s**", h.CategoryName),
		Value:  fmt.Sprintf("To see related commands, please run this command:\n```\n%s%s %s\n```\n\u200b", h.Prefix, h.HelpCommand.Long, h.CategoryShortName),
		Inline: false,
	}, nil
}
