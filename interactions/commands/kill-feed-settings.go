package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions/reactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// KillFeedSettingsCommand struct
type KillFeedSettingsCommand struct {
	Params KillFeedSettingsCommandParams
}

// KillFeedSettingsCommandParams struct
type KillFeedSettingsCommandParams struct {
	ServerID int64
}

// KillFeedSettingsCommandConfirmationOutput struct
type KillFeedSettingsCommandConfirmationOutput struct {
	ServerCount int
	Servers     []models.KillFeedServer
}

// KillFeedSettings func
func (c *Commands) KillFeedSettings(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseKillFeedSettingsCommand(command, mc)
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

	if vErr := guildconfigservice.ValidateGuildFeed(guildFeed, c.Config.Bot.GuildService, "Servers"); vErr != nil {
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

	if _, ok := c.Config.Reactions["kill_feed_pvp"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_pvp reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["kill_feed_pvd"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_pvd reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["kill_feed_pvd"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_pvd reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["kill_feed_wvp"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_wvp reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["kill_feed_dvd"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_dvd reaction"),
		})
		return
	}

	if _, ok := c.Config.Reactions["kill_feed_dvw"]; !ok {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find reactions for command",
			Err:     errors.New("missing kill_feed_dvw reaction"),
		})
		return
	}

	killFeedPvP := c.Config.Reactions["kill_feed_pvp"]
	killFeedPvD := c.Config.Reactions["kill_feed_pvd"]
	killFeedWvP := c.Config.Reactions["kill_feed_wvp"]
	killFeedDvD := c.Config.Reactions["kill_feed_dvd"]
	killFeedDvW := c.Config.Reactions["kill_feed_dvw"]

	reactionModel := models.KillFeedSettingsReaction{
		Reactions: []models.Reaction{
			{
				Name: killFeedPvP.Name,
				ID:   killFeedPvP.ID,
			},
			{
				Name: killFeedPvD.Name,
				ID:   killFeedPvD.ID,
			},
			{
				Name: killFeedWvP.Name,
				ID:   killFeedWvP.ID,
			},
			{
				Name: killFeedDvD.Name,
				ID:   killFeedDvD.ID,
			},
			{
				Name: killFeedDvW.Name,
				ID:   killFeedDvW.ID,
			},
		},
		User: &models.User{
			ID:   mc.Author.ID,
			Name: mc.Author.Username,
		},
	}

	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		if aServer.ServerOutputChannels == nil {
			continue
		}

		if parsedCommand.Params.ServerID != 0 && parsedCommand.Params.ServerID != aServer.NitradoID {
			continue
		}

		for _, outputChannel := range aServer.ServerOutputChannels {
			if !outputChannel.Enabled {
				continue
			}

			if outputChannel.OutputChannelTypeID != "kills" {
				continue
			}

			killFeedServer := models.KillFeedServer{
				Server: models.Server{
					ID:        aServer.ID,
					NitradoID: aServer.NitradoID,
					Name:      aServer.Name,
				},
				OutputChannel: models.ServerOutputChannel{
					ID:                  outputChannel.ID,
					ChannelID:           outputChannel.ChannelID,
					OutputChannelTypeID: outputChannel.OutputChannelTypeID,
					ServerID:            outputChannel.ServerID,
					Enabled:             outputChannel.Enabled,
				},
			}

			if outputChannel.ServerOutputChannelSettings != nil {
				for _, setting := range outputChannel.ServerOutputChannelSettings {
					if !setting.Enabled {
						continue
					}

					killFeedServer.OutputChannel.ServerOutputChannelSettings = append(killFeedServer.OutputChannel.ServerOutputChannelSettings, models.ServerOutputChannelSetting{
						ID:                    setting.ID,
						ServerOutputChannelID: setting.ServerOutputChannelID,
						SettingName:           setting.SettingName,
						SettingValue:          setting.SettingValue,
						Enabled:               setting.Enabled,
					})
				}
			}

			reactionModel.Servers = append(reactionModel.Servers, killFeedServer)
			break
		}
	}

	if len(reactionModel.Servers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find server to toggle kill feed settings",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	var killFeedSettingsOutput KillFeedSettingsCommandConfirmationOutput
	killFeedSettingsOutput.ServerCount = len(reactionModel.Servers)
	killFeedSettingsOutput.Servers = reactionModel.Servers

	embeddableFields = append(embeddableFields, &killFeedSettingsOutput)

	embedParams := discordapi.EmbeddableParams{
		Title:       "Toggle Kill Feed Settings",
		Description: fmt.Sprintf("Please press the relevant reaction to toggle the kill feed setting.\n\n<%s> **Player vs. Player**\n<%s> **Player vs. Tame**\n<%s> **Player vs. Wild Dino**\n<%s> **Tame vs. Tame**", killFeedPvP.FullEmoji, killFeedPvD.FullEmoji, killFeedWvP.FullEmoji, killFeedDvD.FullEmoji),
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

	arErr := discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killFeedPvP.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killFeedPvD.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killFeedWvP.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killFeedDvD.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	arErr = discordapi.AddReaction(s, mc.ChannelID, successMessages[0].ID, killFeedDvW.FullEmoji)
	if arErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: arErr.Message,
			Err:     arErr.Err,
		})
		return
	}

	cacheKey := reactionModel.CacheKey(c.Config.CacheSettings.KillFeedSettingsReaction.Base, successMessages[0].ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &reactionModel, c.Config.CacheSettings.KillFeedSettingsReaction.TTL)
	if setCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	ttl, ttlErr := strconv.ParseInt(c.Config.CacheSettings.KillFeedSettingsReaction.TTL, 10, 64)
	if ttlErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to convert set output reaction TTL to int64",
			Err:     ttlErr,
		})
		return
	}

	c.MessagesAwaitingReaction.Messages[successMessages[0].ID] = reactions.MessageAwaitingReaction{
		Expires: time.Now().Unix() + ttl,
		Reactions: []string{
			killFeedPvP.ID,
			killFeedPvD.ID,
			killFeedWvP.ID,
			killFeedDvD.ID,
			killFeedDvW.ID,
		},
		CommandName: command.Name,
		User:        mc.Author.ID,
	}

	return
}

// parseKillFeedSettingsCommand func
func parseKillFeedSettingsCommand(command configs.Command, mc *discordgo.MessageCreate) (*KillFeedSettingsCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	var serverID int64

	if len(splitContent) > 1 {
		serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
		if sidErr != nil {
			return nil, &Error{
				Message: "Invalid Server ID provided",
				Err:     errors.New("invalid server id"),
			}
		}

		serverID = serverIDInt
	}

	return &KillFeedSettingsCommand{
		Params: KillFeedSettingsCommandParams{
			ServerID: serverID,
		},
	}, nil
}

// ConvertToEmbedField for KillFeedSettingsCommandConfirmationOutput struct
func (so *KillFeedSettingsCommandConfirmationOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := "Unknown"

	if so.ServerCount > 1 {
		fieldVal += fmt.Sprintf("Toggling the settings for %d servers.", so.ServerCount)
		name = "Toggling Kill Feed Settings"
	} else if so.ServerCount == 1 {
		name = so.Servers[0].Server.Name

		if len(so.Servers[0].OutputChannel.ServerOutputChannelSettings) > 0 {
			for _, setting := range so.Servers[0].OutputChannel.ServerOutputChannelSettings {
				switch setting.SettingName {
				case "pvp":
					if setting.SettingValue != "enabled" {
						fieldVal += "**Player vs. Player Log Enabled:** NO\n"
					} else {
						fieldVal += "**Player vs. Player Log Enabled:** YES\n"
					}
				case "pvd":
					if setting.SettingValue != "enabled" {
						fieldVal += "**Player vs. Tame Log Enabled:** NO\n"
					} else {
						fieldVal += "**Player vs. Tame Log Enabled:** YES\n"
					}
				case "pvw":
					if setting.SettingValue != "enabled" {
						fieldVal += "**Player vs. Wild Dino Log Enabled:** NO\n"
					} else {
						fieldVal += "**Player vs. Wild Dino Log Enabled:** YES\n"
					}
				case "dvd":
					if setting.SettingValue != "enabled" {
						fieldVal += "**Tame vs. Tame Log Enabled:** NO\n"
					} else {
						fieldVal += "**Tame vs. Tame Log Enabled:** YES\n"
					}
				case "dvw":
					if setting.SettingValue != "enabled" {
						fieldVal += "**Tame vs. Wild Dino Log Enabled:** NO\n"
					} else {
						fieldVal += "**Tame vs. Wild Dino Log Enabled:** YES\n"
					}
				}
			}
		}
	}

	if fieldVal == "" {
		fieldVal = "Toggling settings for multiple servers. This will swap the current setting for each server."
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
