package reactions

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/nitradoservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Reactions struct
type Reactions struct {
	Session                  *discordgo.Session
	Config                   *configs.Config
	Cache                    *cache.Cache
	GuildConfigService       *guildconfigservice.GuildConfigService
	NitradoService           *nitradoservice.NitradoService
	MessagesAwaitingReaction *MessagesAwaitingReaction
}

type MessagesAwaitingReaction struct {
	Messages map[string]MessageAwaitingReaction
}

// MessageAwaitingReaction struct
type MessageAwaitingReaction struct {
	Expires     int64
	Reactions   []string
	CommandName string
	User        string
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

// IsCommandReaction func
func (r *Reactions) IsCommandReaction(ctx context.Context, messageID string, emojiID string) {

}

// ExpireMessagesAwaitingReaction func
func ExpireMessagesAwaitingReaction(messagesAwaitingReaction *MessagesAwaitingReaction) {
	ticker := time.NewTicker(60 * time.Second)

	for range ticker.C {
		for key, messageAwaitingReaction := range messagesAwaitingReaction.Messages {
			if messageAwaitingReaction.Expires < time.Now().Unix() {
				delete(messagesAwaitingReaction.Messages, key)
			}
		}
	}
}

// Factory func
func (r *Reactions) Factory(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, mar MessageAwaitingReaction) {
	ctx = logging.AddValues(ctx,
		zap.String("scope", logging.GetFuncName()),
		zap.String("command", mar.CommandName),
		zap.String("emoji_id", mra.Emoji.ID),
	)

	logger := logging.Logger(ctx)
	logger.Info("reaction_log")

	// TODO: Needs additional data. For example, if it's a BAN command reaction, who are we trying to ban? Maybe a data cache key to get the info stored separately?
	// var cmr *models.CommandMessageReaction
	// cacheKey := cmr.CacheKey(r.Config.CacheSettings.CommandMessageReaction.Base, mra.MessageID)
	// cErr := r.Cache.GetStruct(ctx, cacheKey, &cmr)
	// if cErr != nil {
	// 	ctx = logging.AddValues(ctx, zap.NamedError("error", cErr.Err), zap.String("error_message", cErr.Message))
	// 	logger := logging.Logger(ctx)
	// 	logger.Error("error_log")
	// 	return
	// } else if cmr == nil {
	// 	ctx = logging.AddValues(ctx, zap.NamedError("error", errors.New("no cached entry")), zap.String("error_message", "failed to find cached entry"))
	// 	logger := logging.Logger(ctx)
	// 	logger.Error("error_log")
	// 	return
	// }

	var command configs.Command
	for _, aCommand := range r.Config.Commands {
		if aCommand.Name == mar.CommandName {
			command = aCommand
			break
		}
	}

	if command.Name == "" {
		r.ErrorOutput(ctx, "Reaction failed", mra.ChannelID, Error{
			Message: "Unable to find command data for reaction",
			Err:     fmt.Errorf("%s command not found", mar.CommandName),
		})
		return
	}

	if !command.Enabled {
		r.ErrorOutput(ctx, "Reaction failed", mra.ChannelID, Error{
			Message: "Command for reaction is not enabled",
			Err:     fmt.Errorf("%s command not enabled", mar.CommandName),
		})
		return
	}

	switch command.Name {
	case "Ban Player":
		r.BanPlayer(ctx, s, mra, command)
	case "Unban Player":
		r.UnbanPlayer(ctx, s, mra, command)
	case "Stop Server":
		r.StopServer(ctx, s, mra, command)
	case "Restart Server":
		r.RestartServer(ctx, s, mra, command)
	case "Whitelist Player":
		r.WhitelistPlayer(ctx, s, mra, command)
	case "Unwhitelist Player":
		r.UnwhitelistPlayer(ctx, s, mra, command)
	case "Clear Whitelist":
		r.ClearWhitelist(ctx, s, mra, command)
	case "Create Channels":
		r.CreateChannels(ctx, s, mra, command)
	case "Set Output":
		r.SetOutput(ctx, s, mra, command)
	case "Add Role":
		r.AddRole(ctx, s, mra, command)
	case "Remove Role":
		r.RemoveRole(ctx, s, mra, command)
	default:
		// TODO: Output error
	}
}

// ErrorOutput func
func (r *Reactions) ErrorOutput(ctx context.Context, content string, channelID string, err Error) ([]*discordgo.Message, *Error) {
	newCtx := logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message))
	logger := logging.Logger(newCtx)
	logger.Error("error_log")

	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	params := discordapi.EmbeddableParams{
		Title:        "Error",
		Description:  "`" + content + "`",
		Color:        r.Config.Bot.ErrorColor,
		TitleURL:     r.Config.Bot.DocumentationURL,
		Footer:       "Error",
		ThumbnailURL: r.Config.Bot.ErrorThumbnail,
	}

	var embeddableFields []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &err)

	embeds := discordapi.CreateEmbeds(params, embeddableFields)

	var messages []*discordgo.Message
	for _, embed := range embeds {
		message, smErr := discordapi.SendMessage(r.Session, channelID, nil, &embed)
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
func (r *Reactions) Output(ctx context.Context, channelID string, command configs.Command, embeddableFields []discordapi.EmbeddableField, embeddableErrors []discordapi.EmbeddableField) ([]*discordgo.Message, *Error) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	params := discordapi.EmbeddableParams{
		Title:        command.Name,
		Description:  command.Description,
		Color:        r.Config.Bot.OkColor,
		TitleURL:     r.Config.Bot.DocumentationURL,
		Footer:       "Executed",
		ThumbnailURL: r.Config.Bot.OkThumbnail,
	}

	if len(embeddableErrors) > 0 {
		params.Color = r.Config.Bot.WarnColor
		params.ThumbnailURL = r.Config.Bot.WarnThumbnail
	}

	combinedFields := append(embeddableFields, embeddableErrors...)
	embeds := discordapi.CreateEmbeds(params, combinedFields)

	var messages []*discordgo.Message
	for _, embed := range embeds {
		message, err := discordapi.SendMessage(r.Session, channelID, nil, &embed)
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
