package runners

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gammazero/workerpool"
	"github.com/google/uuid"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	nsv2 "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

// OnlinePlayersSuccessOutput
type OnlinePlayersSuccessOutput struct {
	Data []PlayerData
}

// PlayerData struct
type PlayerData struct {
	Gamertag string
}

// OnlinePlayersErrorOutput
type OnlinePlayersErrorOutput struct {
	Message string
	Err     Error
}

func (r *Runners) OnlinePlayers(ctx context.Context, delay time.Duration) {
	ctx = logging.AddValues(ctx,
		zap.String("scope", logging.GetFuncName()),
		zap.String("runner", "players"),
	)

	if delay != 0 {
		time.Sleep(time.Second * delay)
	}

	ticker := time.NewTicker(r.Config.Runners.Players.Frequency * time.Second)

	wp := workerpool.New(r.Config.Runners.Players.Workers)

	for range ticker.C {
		requestID := uuid.New()
		gCtx := logging.AddValues(ctx, zap.String("request_id", requestID.String()))

		if wp.WaitingQueueSize() > 0 {
			newCtx := logging.AddValues(ctx,
				zap.Int("queue_size", wp.WaitingQueueSize()),
				zap.NamedError("error", errors.New("queue not empty")),
				zap.String("error_message", "cannot start new online players run with non-empty queue"),
			)
			logger := logging.Logger(newCtx)
			logger.Error("runner_log")
			continue
		} else {
			newCtx := logging.AddValues(gCtx, zap.String("runner_message", "Started online players runner"))
			logger := logging.Logger(newCtx)
			logger.Info("runner_log")
		}

		allGuilds, agErr := guildconfigservice.GetAllGuilds(gCtx, r.GuildConfigService)
		if agErr != nil {
			newCtx := logging.AddValues(gCtx,
				zap.NamedError("error", agErr),
				zap.String("error_message", agErr.Message),
			)
			logger := logging.Logger(newCtx)
			logger.Error("runner_log")
			continue
		}

		if allGuilds.Payload == nil {
			newCtx := logging.AddValues(gCtx,
				zap.NamedError("error", errors.New("nil payload")),
				zap.String("error_message", "nil payload in all guilds request"),
			)
			logger := logging.Logger(newCtx)
			logger.Error("runner_log")
			continue
		}

		if allGuilds.Payload.Guilds == nil {
			newCtx := logging.AddValues(gCtx,
				zap.NamedError("error", errors.New("nil guilds")),
				zap.String("error_message", "nil guilds in all guilds request"),
			)
			logger := logging.Logger(newCtx)
			logger.Error("runner_log")
			continue
		}

		for _, aGuild := range allGuilds.Payload.Guilds {
			agCtx := logging.AddValues(gCtx, zap.String("guild_id", aGuild.ID))

			if !aGuild.Enabled {
				continue
			}

			guildFeed, gfErr := guildconfigservice.GetGuildFeed(agCtx, r.GuildConfigService, aGuild.ID)
			if gfErr != nil {
				newCtx := logging.AddValues(agCtx,
					zap.NamedError("error", gfErr),
					zap.String("error_message", gfErr.Message),
				)
				logger := logging.Logger(newCtx)
				logger.Error("runner_log")
				continue
			}

			if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, r.Config.Bot.GuildService, "Servers"); vErr != nil {
				// newCtx := logging.AddValues(agCtx,
				// 	zap.NamedError("error", vErr),
				// 	zap.String("error_message", vErr.Message),
				// )
				// logger := logging.Logger(newCtx)
				// logger.Info("runner_log")
				continue
			}

			for _, server := range guildFeed.Payload.Guild.Servers {
				serverCtx := logging.AddValues(agCtx,
					zap.Uint64("server_id", server.ID),
					zap.Int64("server_nitrado_id", server.NitradoID),
				)

				if !server.Enabled {
					continue
				}

				if len(server.ServerOutputChannels) == 0 {
					continue
				}

				var onlinePlayersOutputChannel *gcscmodels.ServerOutputChannel
				for _, oc := range server.ServerOutputChannels {
					if !oc.Enabled {
						continue
					}

					if oc.OutputChannelTypeID == "players" {
						var tempAdminLogOutputChannel gcscmodels.ServerOutputChannel = *oc
						onlinePlayersOutputChannel = &tempAdminLogOutputChannel
					}

					if onlinePlayersOutputChannel != nil {
						break
					}
				}

				var aServer gcscmodels.Server = *server

				wp.Submit(func() {
					r.GetOnlinePlayersRequest(serverCtx, aServer, onlinePlayersOutputChannel)
				})
			}
		}
	}
}

// GetOnlinePlayersRequest func
func (r *Runners) GetOnlinePlayersRequest(ctx context.Context, server gcscmodels.Server, onlinePlayersOutput *gcscmodels.ServerOutputChannel) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if onlinePlayersOutput == nil {
		return
	}

	logs, err := r.NitradoService.Client.GetPlayers(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), true, false)
	if err != nil {
		ctx = logging.AddValues(ctx,
			zap.NamedError("error", err),
			zap.String("error_message", err.Message()),
		)
		logger := logging.Logger(ctx)
		logger.Error("runner_log")

		go r.WriteOnlinePlayers(ctx, server, onlinePlayersOutput, logs.Players, &OnlinePlayersErrorOutput{
			Message: err.Message(),
			Err: Error{
				Err: err,
			},
		})
		return
	}

	// FOR TESTING
	// for i := 0; i < 300; i++ {
	// 	logs.Players = append(logs.Players, nsv2.Player{
	// 		Name: fmt.Sprintf("Test__Player__%d", rand.Intn(5)),
	// 	})
	// }

	go r.WriteOnlinePlayers(ctx, server, onlinePlayersOutput, logs.Players, nil)
}

// WriteOnlinePlayers func
func (r *Runners) WriteOnlinePlayers(ctx context.Context, server gcscmodels.Server, onlinePlayersOutput *gcscmodels.ServerOutputChannel, onlinePlayers []nsv2.Player, errs *OnlinePlayersErrorOutput) {
	var outputs []OnlinePlayersSuccessOutput
	var output OnlinePlayersSuccessOutput

	var embedFieldCharacterCount int = 25 // Set to 50 to account for embed field titles
	for _, entry := range onlinePlayers {
		name := strings.Replace(entry.Name, "_", "\\_", -1)
		name = strings.Replace(name, "*", "\\*", -1)
		name = "🟢 " + name

		data := PlayerData{
			Gamertag: name,
		}

		if embedFieldCharacterCount+len(name)+5 < MaxEmbedFieldSize {
			output.Data = append(output.Data, data)
			embedFieldCharacterCount += len(name) + 5
		} else {
			outputs = append(outputs, output)
			output = OnlinePlayersSuccessOutput{
				Data: []PlayerData{
					data,
				},
			}
			embedFieldCharacterCount = 25
		}
	}

	if len(output.Data) > 0 {
		outputs = append(outputs, output)
	} else if len(onlinePlayers) == 0 { // In case there are no players
		outputs = append(outputs, output)
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(outputs) == 0 {
		return
	}

	for i := 0; i < len(outputs); i++ {
		embeddableFields = append(embeddableFields, &outputs[i])
	}

	if errs != nil {
		embeddableErrors = append(embeddableErrors, errs)
	}

	r.OnlinePlayersOutput(ctx, *onlinePlayersOutput, server, len(onlinePlayers), embeddableFields, embeddableErrors)
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *OnlinePlayersSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	for _, entry := range bps.Data {
		fieldVal += fmt.Sprintf("%s\n\n", entry.Gamertag)
	}

	if fieldVal == "" {
		fieldVal = "No players online\n\u200b"
	} else {
		fieldVal = fieldVal[:len(fieldVal)-1] + "\u200b"
	}

	return &discordgo.MessageEmbedField{
		Name:   "\u200b",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *OnlinePlayersErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := bps.Message
	fieldVal := fmt.Sprintf("%s", bps.Err.Error())

	if name == "" {
		name = "Unknown Error"
	}

	if fieldVal == "" {
		fieldVal = "Unknown Error"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
