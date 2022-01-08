package interactions

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/commands"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/reactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/nitradoservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Interactions struct
type Interactions struct {
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
	Code    int    `json:"code"`
}

// Error func
func (ie *Error) Error() string {
	return ie.Err.Error()
}

// SetupHandlers func
func (i *Interactions) SetupHandlers() {
	i.MessagesAwaitingReaction = &reactions.MessagesAwaitingReaction{
		Messages: make(map[string]reactions.MessageAwaitingReaction),
	}

	go reactions.ExpireMessagesAwaitingReaction(i.MessagesAwaitingReaction)

	i.Session.AddHandler(i.MessageCreate)
	i.Session.AddHandler(i.MessageReaction)
}

// MessageCreate func
func (i *Interactions) MessageCreate(s *discordgo.Session, mc *discordgo.MessageCreate) {
	requestID := uuid.New()

	ctx := context.Background()
	ctx = logging.AddValues(
		ctx,
		zap.String("request_id", requestID.String()),
		zap.String("scope", logging.GetFuncName()),
		zap.String("guild_id", mc.GuildID),
		zap.String("channel_id", mc.ChannelID),
		zap.String("message_id", mc.Message.ID),
		zap.String("user_id", mc.Author.ID),
		zap.String("user_name", mc.Author.Username),
	)

	// Ignore message if user is self
	if mc.Author.ID == s.State.User.ID {
		return
	}

	// Ignore message if user is a bot
	if mc.Author.Bot {
		return
	}

	// Check if the message is a command
	if strings.HasPrefix(strings.ToLower(mc.Content), i.Config.Bot.Prefix) {
		commands := commands.Commands{
			Session:                  i.Session,
			Config:                   i.Config,
			Cache:                    i.Cache,
			GuildConfigService:       i.GuildConfigService,
			NitradoService:           i.NitradoService,
			MessagesAwaitingReaction: i.MessagesAwaitingReaction,
		}
		commands.Factory(ctx, s, mc)
		return
	}
}

// MessageReaction func
func (i *Interactions) MessageReaction(s *discordgo.Session, mra *discordgo.MessageReactionAdd) {
	requestID := uuid.New()

	ctx := context.Background()
	ctx = logging.AddValues(
		ctx,
		zap.String("request_id", requestID.String()),
		zap.String("scope", logging.GetFuncName()),
		zap.String("guild_id", mra.GuildID),
		zap.String("message_id", mra.MessageID),
		zap.String("user_id", mra.UserID),
	)

	// Ignore reaction if user is self
	if mra.UserID == s.State.User.ID {
		return
	}

	if i.MessagesAwaitingReaction == nil {
		return
	}

	if val, ok := i.MessagesAwaitingReaction.Messages[mra.MessageID]; ok {
		if val.User != mra.UserID {
			return
		}

		validReaction := false
		for _, reaction := range val.Reactions {
			if reaction == mra.Emoji.ID {
				validReaction = true
				break
			}
		}
		if !validReaction {
			return
		}
	} else {
		return
	}

	reactions := reactions.Reactions{
		Session:                  i.Session,
		Config:                   i.Config,
		Cache:                    i.Cache,
		GuildConfigService:       i.GuildConfigService,
		NitradoService:           i.NitradoService,
		MessagesAwaitingReaction: i.MessagesAwaitingReaction,
	}

	reactions.Factory(ctx, s, mra, i.MessagesAwaitingReaction.Messages[mra.MessageID])
}
