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

// AdminLogsSuccessOutput
type AdminLogsSuccessOutput struct {
	Data      []AdminLogData
	Timestamp int64
}

// AdminLogData struct
type AdminLogData struct {
	Command string
	Name    string
}

// ChatLogsSuccessOutput
type ChatLogsSuccessOutput struct {
	Data      []ChatLogData
	Timestamp int64
}

// ChatLogData struct
type ChatLogData struct {
	Gamertag string
	Name     string
	Message  string
}

// KillLogsSuccessOutputMini struct
type KillLogsSuccessOutputMini struct {
	Data      []KillLogData
	Timestamp int64
}

// KillLogsSuccessOutputFull struct
type KillLogsSuccessOutputFull struct {
	Data KillLogData
}

// KillLogData struct
type KillLogData struct {
	PvEKill        bool
	KilledName     string
	KilledLevel    int
	KilledDinoType string
	KilledTribe    string
	KillerName     string
	KillerLevel    int
	KillerDinoType string
	KillerTribe    string
	Timestamp      int64
}

// ChatLogsErrorOutput
type ChatLogsErrorOutput struct{}

func (r *Runners) Logs(ctx context.Context, delay time.Duration) {
	ctx = logging.AddValues(ctx,
		zap.String("scope", logging.GetFuncName()),
		zap.String("runner", "logs"),
	)

	if delay != 0 {
		time.Sleep(time.Second * delay)
	}

	ticker := time.NewTicker(r.Config.Runners.Logs.Frequency * time.Second)

	wp := workerpool.New(r.Config.Runners.Logs.Workers)

	for range ticker.C {
		requestID := uuid.New()
		gCtx := logging.AddValues(ctx, zap.String("request_id", requestID.String()))

		if wp.WaitingQueueSize() > 0 {
			newCtx := logging.AddValues(gCtx,
				zap.Int("queue_size", wp.WaitingQueueSize()),
				zap.NamedError("error", errors.New("queue not empty")),
				zap.String("error_message", "cannot start new logs run with non-empty queue"),
			)
			logger := logging.Logger(newCtx)
			logger.Error("runner_log")
			continue
		} else {
			newCtx := logging.AddValues(gCtx, zap.String("runner_message", "Started log runner"))
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

				var adminLogOutputChannel *gcscmodels.ServerOutputChannel
				var chatLogOutputChannel *gcscmodels.ServerOutputChannel
				var killLogOutputChannel *gcscmodels.ServerOutputChannel
				for _, oc := range server.ServerOutputChannels {
					if !oc.Enabled {
						continue
					}

					if oc.OutputChannelTypeID == "admin" {
						var tempAdminLogOutputChannel gcscmodels.ServerOutputChannel = *oc
						adminLogOutputChannel = &tempAdminLogOutputChannel
					}

					if oc.OutputChannelTypeID == "chat" {
						var tempChatLogOutputChannel gcscmodels.ServerOutputChannel = *oc
						chatLogOutputChannel = &tempChatLogOutputChannel
					}

					if oc.OutputChannelTypeID == "kills" {
						var tempKillLogOutputChannel gcscmodels.ServerOutputChannel = *oc
						killLogOutputChannel = &tempKillLogOutputChannel
					}

					if adminLogOutputChannel != nil && chatLogOutputChannel != nil && killLogOutputChannel != nil {
						break
					}
				}

				getChat := false
				getAdmin := false
				getKills := false

				if chatLogOutputChannel != nil {
					getChat = true
				}

				if adminLogOutputChannel != nil {
					getAdmin = true
				}

				if killLogOutputChannel != nil {
					getKills = true
				}

				var aServer gcscmodels.Server = *server

				wp.Submit(func() {
					r.GetLogsRequest(serverCtx, aServer, adminLogOutputChannel, chatLogOutputChannel, killLogOutputChannel, getChat, getAdmin, getKills)
				})
			}
		}
	}
}

// GetLogsRequest func
func (r *Runners) GetLogsRequest(ctx context.Context, server gcscmodels.Server, adminLogOutput *gcscmodels.ServerOutputChannel, chatLogOutput *gcscmodels.ServerOutputChannel, killLogOutput *gcscmodels.ServerOutputChannel, getChat bool, getAdmin bool, getKills bool) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	logs, err := r.NitradoService.Client.GetLogs(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), getChat, getAdmin, getKills, true)
	if err != nil {
		ctx = logging.AddValues(ctx,
			zap.NamedError("error", err),
			zap.String("error_message", err.Message()),
		)
		logger := logging.Logger(ctx)
		logger.Error("runner_log")
		return
	}

	// FOR TESTING
	// for i := 0; i < 300; i++ {
	// 	logs.AdminLogs = append(logs.AdminLogs, nsv2.AdminLog{
	// 		Name:      fmt.Sprintf("Test_Player_%d", rand.Intn(5)),
	// 		Command:   "admincheat Summon Shapeshifter_Small_Character_BP_C",
	// 		Timestamp: time.Now().Unix(),
	// 	})
	// 	logs.PlayerLogs = append(logs.PlayerLogs, nsv2.PlayerLog{
	// 		Gamertag: fmt.Sprintf("Test_Player_%d", rand.Intn(5)),
	// 		Name:     "Some Human",
	// 		Message:  "**How** are __you__ *doing* today?",
	// 	})
	// }

	if adminLogOutput != nil && len(logs.AdminLogs) > 0 {
		go r.WriteAdminLogs(ctx, server, adminLogOutput, logs.AdminLogs)
	}

	if chatLogOutput != nil && len(logs.PlayerLogs) > 0 {
		go r.WriteChatLogs(ctx, server, chatLogOutput, logs.PlayerLogs)
	}

	if killLogOutput != nil && len(logs.KillLogs) > 0 {
		go r.WriteKillLogs(ctx, server, killLogOutput, logs.KillLogs)
	}
}

// WriteAdminLogs func
func (r *Runners) WriteAdminLogs(ctx context.Context, server gcscmodels.Server, chatLogOutput *gcscmodels.ServerOutputChannel, adminLogs []nsv2.AdminLog) {
	var outputs []AdminLogsSuccessOutput
	var output AdminLogsSuccessOutput

	var embedFieldCharacterCount int = 50 // Set to 50 to account for embed field titles
	var prevPlayerName string = ""
	for _, entry := range adminLogs {
		name := strings.Replace(entry.Name, "_", "\\_", -1)
		name = strings.Replace(name, "*", "\\*", -1)
		command := strings.Replace(entry.Command, "_", "\\_", -1)
		command = strings.Replace(command, "*", "\\*", -1)
		data := AdminLogData{
			Command: command,
			Name:    name,
		}

		if output.Timestamp == 0 {
			output.Timestamp = entry.Timestamp
		}

		if prevPlayerName == name && embedFieldCharacterCount+len(command)+2 < MaxEmbedFieldSize && len(output.Data) > 0 {
			prevData := output.Data[len(output.Data)-1]
			prevData.Command += "\n" + command
			output.Data[len(output.Data)-1] = prevData
			embedFieldCharacterCount += len(command) + 2
			continue
		}

		if embedFieldCharacterCount+len(command)+len(name)+10 < MaxEmbedFieldSize {
			output.Data = append(output.Data, data)
			embedFieldCharacterCount += len(command) + len(name) + 10
		} else {
			outputs = append(outputs, output)
			output = AdminLogsSuccessOutput{
				Data: []AdminLogData{
					data,
				},
				Timestamp: entry.Timestamp,
			}
			embedFieldCharacterCount = 50
		}

		prevPlayerName = name
	}

	if len(output.Data) > 0 {
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

	freq := r.Config.Runners.Logs.Frequency * time.Second
	r.LogsOutput(ctx, RunnerOutputParams{
		Title:       server.Name,
		Description: fmt.Sprintf("Admin logs are retrieved every %.1f minutes.", freq.Seconds()/60),
	}, *chatLogOutput, server, embeddableFields, embeddableErrors)
}

// WriteChatLogs func
func (r *Runners) WriteChatLogs(ctx context.Context, server gcscmodels.Server, chatLogOutput *gcscmodels.ServerOutputChannel, chatLogs []nsv2.PlayerLog) {
	var outputs []ChatLogsSuccessOutput
	var output ChatLogsSuccessOutput

	var embedFieldCharacterCount int = 50 // Set to 50 to account for embed field titles
	var prevPlayerGT string = ""
	for _, entry := range chatLogs {
		name := strings.Replace(entry.Name, "_", "\\_", -1)
		name = strings.Replace(name, "*", "\\*", -1)
		gt := strings.Replace(entry.Gamertag, "_", "\\_", -1)
		gt = strings.Replace(gt, "*", "\\*", -1)
		message := strings.Replace(entry.Message, "_", "\\_", -1)
		message = strings.Replace(message, "*", "\\*", -1)
		data := ChatLogData{
			Gamertag: gt,
			Name:     name,
			Message:  message,
		}

		if output.Timestamp == 0 {
			output.Timestamp = entry.Timestamp
		}

		if prevPlayerGT == gt && embedFieldCharacterCount+len(message)+2 < MaxEmbedFieldSize && len(output.Data) > 0 {
			prevData := output.Data[len(output.Data)-1]
			prevData.Message += "\n" + message
			output.Data[len(output.Data)-1] = prevData
			embedFieldCharacterCount += len(message) + 2
			continue
		}

		if embedFieldCharacterCount+len(gt)+len(name)+len(message)+10 < MaxEmbedFieldSize {
			output.Data = append(output.Data, data)
			embedFieldCharacterCount += len(gt) + len(name) + len(message) + 10
		} else {
			outputs = append(outputs, output)
			output = ChatLogsSuccessOutput{
				Data: []ChatLogData{
					data,
				},
				Timestamp: entry.Timestamp,
			}
			embedFieldCharacterCount = 50
		}

		prevPlayerGT = gt
	}

	if len(output.Data) > 0 {
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

	freq := r.Config.Runners.Logs.Frequency * time.Second
	r.LogsOutput(ctx, RunnerOutputParams{
		Title:       server.Name,
		Description: fmt.Sprintf("Chat logs are retrieved every %.1f minutes.", freq.Seconds()/60),
	}, *chatLogOutput, server, embeddableFields, embeddableErrors)
}

// WriteChatLogs func
func (r *Runners) WriteKillLogs(ctx context.Context, server gcscmodels.Server, killLogOutput *gcscmodels.ServerOutputChannel, killLogs []nsv2.KillLog) {
	var miniOutputs []KillLogsSuccessOutputMini
	var miniOutput KillLogsSuccessOutputMini

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	minified := len(killLogs) > 320

	var embedFieldCharacterCount int = 20 // Set to 20 to account for descriptive text
	for _, entry := range killLogs {
		killedName := strings.Replace(entry.KilledName, "_", "\\_", -1)
		killedName = strings.Replace(killedName, "*", "\\*", -1)
		killerName := strings.Replace(entry.KillerName, "_", "\\_", -1)
		killerName = strings.Replace(killerName, "*", "\\*", -1)

		data := KillLogData{
			PvEKill:        entry.PvEKill,
			KilledName:     killedName,
			KilledLevel:    entry.KilledLevel,
			KilledDinoType: entry.KilledDinoType,
			KilledTribe:    entry.KilledTribe,
			KillerName:     killerName,
			KillerLevel:    entry.KillerLevel,
			KillerDinoType: entry.KillerDinoType,
			KillerTribe:    entry.KillerTribe,
			Timestamp:      entry.Timestamp,
		}

		if !minified {
			embeddableFields = append(embeddableFields, &KillLogsSuccessOutputFull{
				Data: data,
			})
			continue
		}

		if miniOutput.Timestamp == 0 {
			miniOutput.Timestamp = entry.Timestamp
		}

		outputLength := len(killedName) + len(entry.KilledDinoType) + len(entry.KilledTribe) + len(killerName) + len(entry.KillerDinoType) + len(entry.KillerTribe)

		if embedFieldCharacterCount+outputLength+40 < MaxEmbedFieldSize {
			miniOutput.Data = append(miniOutput.Data, data)
			embedFieldCharacterCount += outputLength + 40
		} else {
			miniOutputs = append(miniOutputs, miniOutput)
			miniOutput = KillLogsSuccessOutputMini{
				Data: []KillLogData{
					data,
				},
				Timestamp: entry.Timestamp,
			}
			embedFieldCharacterCount = 20
		}
	}

	if len(miniOutput.Data) > 0 {
		miniOutputs = append(miniOutputs, miniOutput)
	}

	if !minified {
		r.LogsOutput(ctx, RunnerOutputParams{
			Title:       server.Name,
			Description: fmt.Sprintf("PvP kill feed for %s. All time is in UTC.", time.Now().UTC().Format("January 2, 2006")),
		}, *killLogOutput, server, embeddableFields, embeddableErrors)
		return
	}

	if len(miniOutputs) == 0 {
		return
	}

	for i := 0; i < len(miniOutputs); i++ {
		embeddableFields = append(embeddableFields, &miniOutputs[i])
	}

	r.LogsOutput(ctx, RunnerOutputParams{
		Title:       server.Name,
		Description: fmt.Sprintf("PvP kill feed for %s. All time is in UTC.", time.Now().UTC().Format("January 2, 2006")),
	}, *killLogOutput, server, embeddableFields, embeddableErrors)
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *AdminLogsSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	for _, entry := range bps.Data {
		fieldVal += fmt.Sprintf("\n**%s**", entry.Name)
		fieldVal += "\n" + entry.Command + "\n"
	}

	if fieldVal == "" {
		fieldVal = "No logs"
	} else {
		fieldVal += "\u200b"
	}

	dateTime := time.Unix(bps.Timestamp, 0)
	utcTime := fmt.Sprintf("__%s__", dateTime.UTC().String())

	if utcTime == "" {
		utcTime = "Unknown Time"
	}

	return &discordgo.MessageEmbedField{
		Name:   utcTime,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (bps *ChatLogsSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	for _, entry := range bps.Data {
		fieldVal += fmt.Sprintf("\n**%s (%s)**", entry.Gamertag, entry.Name)
		fieldVal += "\n" + entry.Message + "\n"
	}

	if fieldVal == "" {
		fieldVal = "No logs"
	} else {
		fieldVal += "\u200b"
	}

	dateTime := time.Unix(bps.Timestamp, 0)
	utcTime := fmt.Sprintf("__%s__", dateTime.UTC().String())

	if utcTime == "" {
		utcTime = "Unknown Time"
	}

	return &discordgo.MessageEmbedField{
		Name:   utcTime,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for KillLogsSuccessOutputFull struct
func (bps *KillLogsSuccessOutputFull) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := ""

	dateTime := time.Unix(bps.Data.Timestamp, 0)
	utcHHMM := dateTime.UTC().Format(time.Kitchen)

	killedMessage := ""
	if bps.Data.PvEKill {
		killedMessage = fmt.Sprintf("**%s was killed by a %s**", bps.Data.KilledName, bps.Data.KillerName)
		fieldVal += fmt.Sprintf("*%s - %s*\n__Killer Info__:\nCreature: %s\nLevel: %d\n\n", "Wild Dino Kill", utcHHMM, bps.Data.KillerName, bps.Data.KillerLevel)
	} else if bps.Data.KillerDinoType != "" {
		killedMessage = fmt.Sprintf("**%s was killed by %s**", bps.Data.KilledName, bps.Data.KillerName)
		fieldVal += fmt.Sprintf("*%s - %s*\n__Killer Info__:\nName: %s\nCreature: %s\nTribe: %s\nLevel: %d\n\n", "Tamed Dino Kill", utcHHMM, bps.Data.KillerName, bps.Data.KillerDinoType, bps.Data.KillerTribe, bps.Data.KillerLevel)
	} else {
		killedMessage = fmt.Sprintf("**%s was killed by %s**", bps.Data.KilledName, bps.Data.KillerName)
		fieldVal += fmt.Sprintf("*%s - %s*\n__Killer Info__:\nName: %s\nTribe: %s\nLevel: %d\n\n", "Player Kill", utcHHMM, bps.Data.KillerName, bps.Data.KillerTribe, bps.Data.KillerLevel)
	}

	name = killedMessage

	if bps.Data.KilledDinoType != "" {
		if bps.Data.KilledTribe != "" {
			fieldVal += fmt.Sprintf("__Killed Info__:\nName: %s\nCreature: %s\nTribe: %s\nLevel: %d\n\u200b", bps.Data.KilledName, bps.Data.KilledDinoType, bps.Data.KilledTribe, bps.Data.KilledLevel)
		} else {
			fieldVal += fmt.Sprintf("__Killed Info__:\nName: %s\nCreature: %s\nLevel: %d\n\u200b", bps.Data.KilledName, bps.Data.KilledDinoType, bps.Data.KilledLevel)
		}
	} else {
		fieldVal += fmt.Sprintf("__Killed Info__:\nName: %s\nTribe: %s\nLevel: %d\n\u200b", bps.Data.KilledName, bps.Data.KilledTribe, bps.Data.KilledLevel)
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for KillLogsSuccessOutputMini struct
func (bps *KillLogsSuccessOutputMini) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""

	dateTime := time.Unix(bps.Timestamp, 0)
	utcHHMM := dateTime.UTC().Format(time.Kitchen)

	if utcHHMM == "" {
		utcHHMM = "00:00"
	}

	if len(bps.Data) == 0 {
		fieldVal = "No PvP data..."
	} else {
		for _, killLog := range bps.Data {
			if killLog.PvEKill {
				fieldVal += fmt.Sprintf("**%s** (%s) was killed by a **%s**\n", killLog.KilledName, killLog.KilledTribe, killLog.KillerName)
			} else if killLog.KillerDinoType != "" {
				if killLog.KilledDinoType != "" {
					fieldVal += fmt.Sprintf("**%s** [%s] (%s) was killed by **%s** [%s] (%s)\n", killLog.KilledName, killLog.KilledDinoType, killLog.KilledTribe, killLog.KillerName, killLog.KillerDinoType, killLog.KillerTribe)
				} else {
					fieldVal += fmt.Sprintf("**%s** (%s) was killed by **%s** [%s] (%s)\n", killLog.KilledName, killLog.KilledTribe, killLog.KillerName, killLog.KillerDinoType, killLog.KillerTribe)
				}
			} else {
				if killLog.KilledDinoType != "" {
					fieldVal += fmt.Sprintf("**%s** [%s] (%s) was killed by **%s** (%s)\n", killLog.KilledName, killLog.KilledDinoType, killLog.KilledTribe, killLog.KillerName, killLog.KillerTribe)
				} else {
					fieldVal += fmt.Sprintf("**%s** (%s) was killed by **%s** (%s)\n", killLog.KilledName, killLog.KilledTribe, killLog.KillerName, killLog.KillerTribe)
				}
			}
		}
	}

	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("__**%s**__", utcHHMM),
		Value:  fieldVal,
		Inline: false,
	}, nil
}
