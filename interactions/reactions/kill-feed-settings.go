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

// KillFeedSettingsSuccessOutput struct
type KillFeedSettingsSuccessOutput struct {
	SettingName    string
	SettingChanges []SettingChange
}

// SettingChange struct
type SettingChange struct {
	NitradoID       int64
	SettingName     string
	OriginalSetting string
	NewSetting      string
}

// KillFeedSettingsErrorOutput struct
type KillFeedSettingsErrorOutput struct {
	SettingName       string
	FailedUpdateCount int
}

// KillFeedSettings func
func (r *Reactions) KillFeedSettings(ctx context.Context, s *discordgo.Session, mra *discordgo.MessageReactionAdd, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var reactionModel *models.KillFeedSettingsReaction
	cacheKey := reactionModel.CacheKey(r.Config.CacheSettings.KillFeedSettingsReaction.Base, mra.MessageID)
	cErr := r.Cache.GetStruct(ctx, cacheKey, &reactionModel)
	if cErr != nil {
		r.ErrorOutput(ctx, "Failed to set kill feed setting", mra.ChannelID, Error{
			Message: cErr.Message,
			Err:     cErr,
		})
		return
	} else if reactionModel == nil {
		r.ErrorOutput(ctx, "Failed to kill feed setting", mra.ChannelID, Error{
			Message: "kill feed setting message has expired",
			Err:     errors.New("please run the kill feed setting command again"),
		})
		return
	}

	var successOutput KillFeedSettingsSuccessOutput
	var errorOutput KillFeedSettingsErrorOutput

	for _, server := range reactionModel.Servers {

		/*
			killFeedPvP := c.Config.Reactions["kill_feed_pvp"]
			killFeedPvD := c.Config.Reactions["kill_feed_pvd"]
			killFeedWvP := c.Config.Reactions["kill_feed_wvp"]
			killFeedDvD := c.Config.Reactions["kill_feed_dvd"]
		*/

		var settingName string
		switch mra.MessageReaction.Emoji.ID {
		case r.Config.Reactions["kill_feed_pvp"].ID:
			settingName = "pvp"
			successOutput.SettingName = "PvP Logs"
			errorOutput.SettingName = "PvP Logs"
		case r.Config.Reactions["kill_feed_pvd"].ID:
			settingName = "pvd"
			successOutput.SettingName = "Player vs. Tame Logs"
			errorOutput.SettingName = "Player vs. Tame Logs"
		case r.Config.Reactions["kill_feed_wvp"].ID:
			settingName = "wvp"
			successOutput.SettingName = "Player vs. Wild Dino Logs"
			errorOutput.SettingName = "Player vs. Wild Dino Logs"
		case r.Config.Reactions["kill_feed_dvd"].ID:
			settingName = "dvd"
			successOutput.SettingName = "Tame vs. Tame Logs"
			errorOutput.SettingName = "Tame vs. Tame Logs"
		case r.Config.Reactions["kill_feed_dvw"].ID:
			settingName = "dvw"
			successOutput.SettingName = "Tame vs. Wild Dino Logs"
			errorOutput.SettingName = "Tame vs. Wild Dino Logs"
		default:
			continue
		}

		settingValue := "enabled"
		var setting *models.ServerOutputChannelSetting
		for _, aSetting := range server.OutputChannel.ServerOutputChannelSettings {
			if aSetting.SettingName == settingName {
				setting = &aSetting
				settingValue = aSetting.SettingValue
				break
			}
		}

		var newSettingValue string
		if settingValue == "enabled" {
			newSettingValue = "disabled"
		} else {
			newSettingValue = "enabled"
		}

		if setting == nil {
			_, socsErr := guildconfigservice.CreateServerOutputChannelSetting(ctx, r.GuildConfigService, mra.GuildID, server.OutputChannel.ID, settingName, newSettingValue)
			if socsErr != nil {
				newCtx := logging.AddValues(ctx,
					zap.NamedError("error", socsErr),
					zap.String("error_message", socsErr.Message),
					zap.Uint64("server_id", server.Server.ID),
					zap.String("setting_name", settingName),
					zap.String("setting_value", settingValue),
				)
				logger := logging.Logger(newCtx)
				logger.Error("error_log")

				errorOutput.FailedUpdateCount += 1
				continue
			}

			successOutput.SettingChanges = append(successOutput.SettingChanges, SettingChange{
				NitradoID:       server.Server.NitradoID,
				SettingName:     settingName,
				OriginalSetting: settingValue,
				NewSetting:      newSettingValue,
			})
		} else {
			_, socsErr := guildconfigservice.UpdateServerOutputChannelSetting(ctx, r.GuildConfigService, mra.GuildID, setting.ID, server.OutputChannel.ID, settingName, newSettingValue)
			if socsErr != nil {
				newCtx := logging.AddValues(ctx,
					zap.NamedError("error", socsErr),
					zap.String("error_message", socsErr.Message),
					zap.Uint64("server_id", server.Server.ID),
					zap.String("setting_name", settingName),
					zap.String("setting_value", settingValue),
				)
				logger := logging.Logger(newCtx)
				logger.Error("error_log")

				errorOutput.FailedUpdateCount += 1
				continue
			}
		}

		successOutput.SettingChanges = append(successOutput.SettingChanges, SettingChange{
			NitradoID:       server.Server.NitradoID,
			SettingName:     settingName,
			OriginalSetting: settingValue,
			NewSetting:      newSettingValue,
		})
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	if len(successOutput.SettingChanges) > 0 {
		embeddableFields = append(embeddableFields, &successOutput)
	}

	if errorOutput.FailedUpdateCount > 0 {
		embeddableErrors = append(embeddableErrors, &errorOutput)
	}

	editedCommand := command
	editedCommand.Name = "Updated Kill Feed Settings"
	editedCommand.Description = "Settings updated for the kill feed logging."

	r.Output(ctx, mra.ChannelID, editedCommand, embeddableFields, embeddableErrors)
	return
}

// ConvertToEmbedField for KillFeedSettingsSuccessOutput struct
func (out *KillFeedSettingsSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := fmt.Sprintf("Updated %s setting for %d server(s)", out.SettingName, len(out.SettingChanges))

	totalEnabled := 0
	totalDisabled := 0

	for _, change := range out.SettingChanges {
		if change.NewSetting == "enabled" {
			totalEnabled += 1
		} else if change.NewSetting == "disabled" {
			totalDisabled += 1
		}
	}

	if len(out.SettingChanges) > 1 {
		fieldVal = fmt.Sprintf("Servers Enabled: %d\nServers Disabled: %d", totalEnabled, totalDisabled)
	} else if len(out.SettingChanges) == 1 {
		fieldVal = fmt.Sprintf("Original Setting: %s\nNew Setting: %s", out.SettingChanges[0].OriginalSetting, out.SettingChanges[0].NewSetting)
	} else {
		fieldVal = "Something went wrong"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for KillFeedSettingsErrorOutput struct
func (out *KillFeedSettingsErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprintf("Failed to update %s setting", out.SettingName),
		Value:  fmt.Sprintf("Servers: %d", out.FailedUpdateCount),
		Inline: false,
	}, nil
}
