package runners

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/nitradoservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// MaxEmbedSize
const MaxEmbedSize int = 5500

// MaxEmbedFields
const MaxEmbedFields int = 22

// MaxEmbedFieldSize
const MaxEmbedFieldSize int = 900

// Runners struct
type Runners struct {
	Session            *discordgo.Session
	Config             *configs.Config
	Cache              *cache.Cache
	GuildConfigService *guildconfigservice.GuildConfigService
	NitradoService     *nitradoservice.NitradoService
}

// Error struct
type Error struct {
	Message string `json:"message"`
	Err     error  `json:"error"`
}

// RunnerOutputParams struct
type RunnerOutputParams struct {
	Title       string
	Description string
}

// Error func
func (e *Error) Error() string {
	return e.Err.Error()
}

// ConvertToEmbedField for Error struct
func (e *Error) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	return &discordgo.MessageEmbedField{
		Name:   e.Message,
		Value:  e.Error(),
		Inline: false,
	}, nil
}

// StartRunners func
func (r *Runners) StartRunners() {
	ctx := context.Background()

	go r.Logs(ctx, r.Config.Runners.Logs.Delay)
	go r.OnlinePlayers(ctx, r.Config.Runners.Players.Delay)
	// go r.StatusRunner(r.Config.Runners.Status.Delay)
	// go r.ServicesRunner(r.Config.Runners.Services.Delay)
}

// LogsOutput func
func (r *Runners) LogsOutput(ctx context.Context, runnerParams RunnerOutputParams, channel gcscmodels.ServerOutputChannel, server gcscmodels.Server, embeddableFields []discordapi.EmbeddableField, embeddableErrors []discordapi.EmbeddableField) ([]*discordgo.Message, *Error) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	params := discordapi.EmbeddableParams{
		Title:       runnerParams.Title,
		Description: runnerParams.Description,
		Color:       r.Config.Bot.OkColor,
		TitleURL:    r.Config.Bot.DocumentationURL,
		Footer:      "Retrieved",
	}

	if len(embeddableErrors) > 0 {
		params.Color = r.Config.Bot.WarnColor
	}

	combinedFields := append(embeddableFields, embeddableErrors...)
	embeds := discordapi.CreateEmbeds(params, combinedFields)

	var messages []*discordgo.Message
	for _, embed := range embeds {
		message, err := discordapi.SendMessage(r.Session, channel.ChannelID, nil, &embed)
		if err != nil {
			tempCtx := logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message), zap.Int("status_code", err.Code))
			logger := logging.Logger(tempCtx)
			logger.Error("runner_log")

			if err.Code == 10003 {
				_, dsocErr := guildconfigservice.DeleteServerOutputChannel(ctx, r.GuildConfigService, server.GuildID, int64(channel.ID))
				if dsocErr != nil {
					gcCtx := logging.AddValues(ctx, zap.NamedError("error", dsocErr.Err), zap.String("error_message", dsocErr.Message))
					logger := logging.Logger(gcCtx)
					logger.Error("error_log")
				}
			}

			return nil, &Error{
				Message: err.Message,
				Err:     err.Err,
			}
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// OnlinePlayersOutput func
func (r *Runners) OnlinePlayersOutput(ctx context.Context, channel gcscmodels.ServerOutputChannel, server gcscmodels.Server, totalPlayers int, embeddableFields []discordapi.EmbeddableField, embeddableErrors []discordapi.EmbeddableField) ([]*discordgo.Message, *Error) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var onlinePlayersOutputChannelMessages *models.OnlinePlayersOutputChannelMessages
	cacheKey := onlinePlayersOutputChannelMessages.CacheKey(r.Config.CacheSettings.OnlinePlayersOutputChannelMessages.Base, channel.ChannelID, int64(server.ID))
	stcErr := r.Cache.GetStruct(ctx, cacheKey, &onlinePlayersOutputChannelMessages)
	if stcErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", stcErr.Err), zap.String("error_message", stcErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("runner_log")

		return nil, &Error{
			Message: stcErr.Message,
			Err:     stcErr.Err,
		}
	}

	freq := r.Config.Runners.Players.Frequency * time.Second
	params := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("%s", server.Name),
		Description: fmt.Sprintf("**%d Players Online**\nOnline players are retrieved every %.1f minutes.", totalPlayers, freq.Seconds()/60),
		Color:       r.Config.Bot.OkColor,
		TitleURL:    r.Config.Bot.DocumentationURL,
		Footer:      "Retrieved",
	}

	if len(embeddableErrors) > 0 {
		params.Color = r.Config.Bot.WarnColor
	}

	combinedFields := append(embeddableFields, embeddableErrors...)
	embeds := discordapi.CreateEmbeds(params, combinedFields)

	var messages []*discordgo.Message
	var modelMessages []models.Message
	for key, embed := range embeds {
		if onlinePlayersOutputChannelMessages != nil {
			if key >= len(onlinePlayersOutputChannelMessages.Messages) {
				message, err := discordapi.SendMessage(r.Session, channel.ChannelID, nil, &embed)
				if err != nil {
					if err.Code == 10003 {
						_, dsocErr := guildconfigservice.DeleteServerOutputChannel(ctx, r.GuildConfigService, server.GuildID, int64(channel.ID))
						if dsocErr != nil {
							gcCtx := logging.AddValues(ctx, zap.NamedError("error", dsocErr.Err), zap.String("error_message", dsocErr.Message))
							logger := logging.Logger(gcCtx)
							logger.Error("error_log")
						}
					} else {
						tempCtx := logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message), zap.Int("status_code", err.Code))
						logger := logging.Logger(tempCtx)
						logger.Error("runner_log")
					}

					return nil, &Error{
						Message: err.Message,
						Err:     err.Err,
					}
				}
				messages = append(messages, message)
				modelMessages = append(modelMessages, models.Message{
					ID: message.ID,
					Channel: models.Channel{
						ID: message.ChannelID,
					},
				})
			} else {
				message, err := discordapi.EditMessage(r.Session, channel.ChannelID, onlinePlayersOutputChannelMessages.Messages[key].ID, nil, &embed)
				if err != nil {
					if err.Code == 10003 {
						_, dsocErr := guildconfigservice.DeleteServerOutputChannel(ctx, r.GuildConfigService, server.GuildID, int64(channel.ID))
						if dsocErr != nil {
							gcCtx := logging.AddValues(ctx, zap.NamedError("error", dsocErr.Err), zap.String("error_message", dsocErr.Message))
							logger := logging.Logger(gcCtx)
							logger.Error("error_log")
						}
					} else if err.Code == 10008 {
						newmessage, nerr := discordapi.SendMessage(r.Session, channel.ChannelID, nil, &embed)
						if nerr != nil {
							tempCtx := logging.AddValues(ctx, zap.NamedError("error", nerr.Err), zap.String("error_message", nerr.Message), zap.Int("status_code", nerr.Code))
							logger := logging.Logger(tempCtx)
							logger.Error("runner_log")

							if err.Code == 10003 {
								_, dsocErr := guildconfigservice.DeleteServerOutputChannel(ctx, r.GuildConfigService, server.GuildID, int64(channel.ID))
								if dsocErr != nil {
									gcCtx := logging.AddValues(ctx, zap.NamedError("error", dsocErr.Err), zap.String("error_message", dsocErr.Message))
									logger := logging.Logger(gcCtx)
									logger.Error("error_log")
								}
							}

							return nil, &Error{
								Message: nerr.Message,
								Err:     nerr.Err,
							}
						}
						messages = append(messages, newmessage)
						modelMessages = append(modelMessages, models.Message{
							ID: newmessage.ID,
							Channel: models.Channel{
								ID: newmessage.ChannelID,
							},
						})
					} else {
						tempCtx := logging.AddValues(ctx, zap.NamedError("error", err.Err), zap.String("error_message", err.Message), zap.Int("status_code", err.Code))
						logger := logging.Logger(tempCtx)
						logger.Error("runner_log")
					}
				} else {
					messages = append(messages, message)
					modelMessages = append(modelMessages, models.Message{
						ID: message.ID,
						Channel: models.Channel{
							ID: message.ChannelID,
						},
					})
				}
			}
		}
	}

	setErr := r.Cache.SetStruct(ctx, cacheKey, &models.OnlinePlayersOutputChannelMessages{
		Messages: modelMessages,
		Channel: models.Channel{
			ID: channel.ChannelID,
		},
	}, r.Config.CacheSettings.OnlinePlayersOutputChannelMessages.TTL)
	if setErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", setErr.Err), zap.String("error_message", setErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("runner_log")

		return nil, &Error{
			Message: stcErr.Message,
			Err:     stcErr.Err,
		}
	}

	if onlinePlayersOutputChannelMessages != nil {
		for _, prevMessage := range onlinePlayersOutputChannelMessages.Messages {
			foundMessage := false
			for _, newMessage := range modelMessages {
				if newMessage.ID == prevMessage.ID {
					foundMessage = true
					break
				}
			}

			if !foundMessage {
				dmErr := discordapi.DeleteMessage(r.Session, channel.ChannelID, prevMessage.ID)
				if dmErr != nil {
					tempCtx := logging.AddValues(ctx, zap.NamedError("error", dmErr.Err), zap.String("error_message", dmErr.Message))
					logger := logging.Logger(tempCtx)
					logger.Error("runner_log")

					if dmErr.Code == 10003 {
						_, dsocErr := guildconfigservice.DeleteServerOutputChannel(ctx, r.GuildConfigService, server.GuildID, int64(channel.ID))
						if dsocErr != nil {
							gcCtx := logging.AddValues(ctx, zap.NamedError("error", dsocErr.Err), zap.String("error_message", dsocErr.Message))
							logger := logging.Logger(gcCtx)
							logger.Error("error_log")
						}
					}

					if dmErr.Code == 10008 {
						// TODO: Handle message not found by logging
					} else {
						return nil, &Error{
							Message: dmErr.Message,
							Err:     dmErr.Err,
						}
					}
				}
			}
		}
	}

	return messages, nil
}
