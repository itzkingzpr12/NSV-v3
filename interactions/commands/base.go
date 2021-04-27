package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/reactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/nitradoservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Commands struct
type Commands struct {
	Session                  *discordgo.Session
	Config                   *configs.Config
	Cache                    *cache.Cache
	GuildConfigService       *guildconfigservice.GuildConfigService
	NitradoService           *nitradoservice.NitradoService
	MessagesAwaitingReaction *reactions.MessagesAwaitingReaction
}

// Error struct
type Error struct {
	Message string `json:"message"`
	Err     error  `json:"error"`
}

// Error func
func (e *Error) Error() string {
	return e.Err.Error()
}

// HasPrefix func
func (c *Commands) HasPrefix(ctx, message string) bool {
	prefixLen := len(c.Config.Bot.Prefix)
	if len(message) < prefixLen {
		return false
	}

	if strings.ToLower(message[:prefixLen]) != c.Config.Bot.Prefix {
		return false
	}

	return true
}

// getCommand func
func getCommand(commands []configs.Command, prefix string, content string) (configs.Command, *Error) {
	prefixLen := len(prefix)
	if len(content) <= prefixLen {
		return configs.Command{}, &Error{
			Message: "Not a command",
			Err:     errors.New("not a command"),
		}
	}

	return getCommandConfig(commands, strings.SplitN(content[prefixLen:], " ", 2)[0])

}

// Factory func
func (c *Commands) Factory(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate) {
	ctx = logging.AddValues(ctx,
		zap.String("scope", logging.GetFuncName()),
		zap.String("message_content", mc.Content),
	)

	command, gcErr := getCommand(c.Config.Commands, c.Config.Bot.Prefix, mc.Content)
	if gcErr != nil {
		if gcErr.Error() == "not a command" {
			return
		}

		ctx = logging.AddValues(ctx, zap.NamedError("error", gcErr.Err), zap.String("error_message", gcErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")
		return
	}

	if !command.Enabled {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "This command has not been enabled for use",
			Err:     errors.New("command not enabled"),
		})
		return
	}

	ctx = logging.AddValues(ctx,
		zap.String("command", command.Name),
	)

	logger := logging.Logger(ctx)
	logger.Info("command_log")

	switch command.Name {
	case "Add Nitrado Token":
		break
	default:
		if mc.GuildID == "" {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "This command cannot be used through DM",
				Err:     errors.New("must be used in discord server"),
			})
			return
		}
	}

	switch command.Name {
	case "List Servers":
		c.ListServers(ctx, s, mc, command)
	case "Name Server":
		c.NameServer(ctx, s, mc, command)
	case "Nitrado Token":
		c.NitradoToken(ctx, s, mc, command)
	case "Add Nitrado Token":
		c.AddNitradoToken(ctx, s, mc, command)
	case "Remove Server":
		c.RemoveServer(ctx, s, mc, command)
	case "Auto Setup":
		c.Setup(ctx, s, mc, command)
	case "Bot Activation":
		c.Activate(ctx, s, mc, command)
	case "Help":
		c.Help(ctx, s, mc, command)
	case "Ban Player":
		c.BanPlayer(ctx, s, mc, command)
	case "Unban Player":
		c.UnbanPlayer(ctx, s, mc, command)
	case "Get Banlist":
		c.GetBanlist(ctx, s, mc, command)
	case "Stop Server":
		c.StopServer(ctx, s, mc, command)
	case "Restart Server":
		c.RestartServer(ctx, s, mc, command)
	case "Whitelist Player":
		c.WhitelistPlayer(ctx, s, mc, command)
	case "Unwhitelist Player":
		c.UnwhitelistPlayer(ctx, s, mc, command)
	case "Get Whitelist":
		c.GetWhitelist(ctx, s, mc, command)
	case "Clear Whitelist":
		c.ClearWhitelist(ctx, s, mc, command)
	case "Create Channels":
		c.CreateChannels(ctx, s, mc, command)
	case "Set Output":
		c.SetOutput(ctx, s, mc, command)
	case "Add Role":
		c.AddRole(ctx, s, mc, command)
	case "Remove Role":
		c.RemoveRole(ctx, s, mc, command)
	case "Search Players":
		c.SearchPlayers(ctx, s, mc, command)
	default:
		// TODO: Output error
	}
}

// getCommandConfig func
func getCommandConfig(commands []configs.Command, command string) (configs.Command, *Error) {
	for _, val := range commands {
		if val.Long == strings.ToLower(command) || val.Short == strings.ToLower(command) {
			return val, nil
		}
	}

	return configs.Command{}, &Error{
		Message: fmt.Sprintf("No command found with name: %s", command),
		Err:     errors.New("invalid command"),
	}
}

// ErrorOutput func
func (c *Commands) ErrorOutput(ctx context.Context, command configs.Command, content string, channelID string, err Error) ([]*discordgo.Message, *Error) {
	newCtx := logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message))
	logger := logging.Logger(newCtx)
	logger.Error("error_log")

	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	params := discordapi.EmbeddableParams{
		Title:        "Error",
		Description:  "`" + content + "`",
		Color:        c.Config.Bot.ErrorColor,
		TitleURL:     c.Config.Bot.DocumentationURL,
		Footer:       "Error",
		ThumbnailURL: c.Config.Bot.ErrorThumbnail,
	}

	var embeddableFields []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &err)
	embeddableFields = append(embeddableFields, &HelpOutput{
		Command: command,
		Prefix:  c.Config.Bot.Prefix,
	})

	embeds := discordapi.CreateEmbeds(params, embeddableFields)

	var messages []*discordgo.Message
	for _, embed := range embeds {
		message, smErr := discordapi.SendMessage(c.Session, channelID, nil, &embed)
		if smErr != nil {
			ctx = logging.AddValues(ctx, zap.NamedError("error", smErr.Err), zap.String("error_message", smErr.Message), zap.Int("status_code", smErr.Code))
			logger := logging.Logger(ctx)
			logger.Error("error_log")

			return nil, &Error{
				Message: smErr.Message,
				Err:     smErr.Err,
			}
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// Output func
func (c *Commands) Output(ctx context.Context, channelID string, params discordapi.EmbeddableParams, embeddableFields []discordapi.EmbeddableField, embeddableErrors []discordapi.EmbeddableField) ([]*discordgo.Message, *Error) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if len(embeddableErrors) > 0 {
		params.Color = c.Config.Bot.WarnColor
	} else {
		params.Color = c.Config.Bot.OkColor
	}

	combinedFields := append(embeddableFields, embeddableErrors...)
	embeds := discordapi.CreateEmbeds(params, combinedFields)

	var messages []*discordgo.Message
	for _, embed := range embeds {
		message, err := discordapi.SendMessage(c.Session, channelID, nil, &embed)
		if err != nil {
			ctx = logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message), zap.Int("status_code", err.Code))
			logger := logging.Logger(ctx)
			logger.Error("error_log")

			return nil, &Error{
				Message: err.Message,
				Err:     err.Err,
			}
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// ConvertToEmbedField for Error struct
func (e *Error) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	return &discordgo.MessageEmbedField{
		Name:   e.Message,
		Value:  e.Error(),
		Inline: false,
	}, nil
}

// IsAdmin func
func (c *Commands) IsAdmin(ctx context.Context, guildID string, roles []string) (bool, *Error) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	discRoles, grErr := discordapi.GetGuildRoles(c.Session, guildID)
	if grErr != nil {
		return false, &Error{
			Message: "Failed to get Guild roles to verify Administrator access",
			Err:     grErr.Err,
		}
	}

	for _, memberRole := range roles {
		for _, guildRole := range discRoles {
			if memberRole == guildRole.ID && (guildRole.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
				return true, nil
			}
		}
	}

	return false, nil
}

// IsApproved func
func (c *Commands) IsApproved(ctx context.Context, guildFeed *gcscmodels.Guild, commandName string, roles []string) bool {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if guildFeed == nil {
		return false
	}

	if guildFeed.GuildServices == nil {
		return false
	}

	for _, memberRole := range roles {
		for _, guildService := range guildFeed.GuildServices {
			if guildService.Name != c.Config.Bot.GuildService {
				continue
			}

			if guildService.GuildServicePermissions == nil {
				return false
			}

			for _, permission := range guildService.GuildServicePermissions {
				if permission.CommandName != commandName {
					continue
				}

				if permission.RoleID != memberRole {
					continue
				}

				return true
			}
		}
	}

	return false
}
