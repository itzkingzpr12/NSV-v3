package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// NitradoTokenCommand struct
type NitradoTokenCommand struct {
	Params NitradoTokenCommandParams
}

// NitradoTokenCommandParams struct
type NitradoTokenCommandParams struct{}

// NitradoTokenOutput struct
type NitradoTokenOutput struct {
	Guild models.Guild `json:"guild"`
}

// NitradoToken func
func (c *Commands) NitradoToken(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := parseNitradoTokenCommand(command, mc)
	if err != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *err)
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

	nitradoTokenGuild := models.NitradoTokenGuild{
		Guild: models.Guild{
			ID: mc.GuildID,
		},
		User: models.User{
			ID: mc.Author.ID,
		},
	}

	dmChannel, dmErr := discordapi.CreateDMChannel(s, mc.Author.ID)
	if dmErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", dmErr.Err), zap.String("error_message", dmErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: dmErr.Message,
			Err:     dmErr.Err,
		})
		return
	}

	cacheKey := nitradoTokenGuild.CacheKey(c.Config.CacheSettings.NitradoTokenGuild.Base, mc.Author.ID)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &nitradoTokenGuild, c.Config.CacheSettings.NitradoTokenGuild.TTL)
	if setCacheErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", setCacheErr.Err), zap.String("error_message", setCacheErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: setCacheErr.Message,
			Err:     setCacheErr.Err,
		})
		return
	}

	nto := NitradoTokenOutput{
		Guild: models.Guild{
			ID: mc.GuildID,
		},
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	antCommand, antErr := getCommandConfig(c.Config.Commands, "addtoken")
	if antErr != nil {
		embeddableFields = append(embeddableFields, &nto)
	} else {
		ant := HelpOutput{
			Command: antCommand,
			Prefix:  c.Config.Bot.Prefix,
		}
		embeddableFields = append(embeddableFields, &ant)
	}

	embedParams := discordapi.EmbeddableParams{
		Title:       command.Name,
		Description: command.Description,
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	if len(embeddableErrors) == 0 {
		embedParams.ThumbnailURL = c.Config.Bot.WorkingThumbnail
	} else {
		embedParams.ThumbnailURL = c.Config.Bot.WarnThumbnail
	}

	c.Output(ctx, dmChannel.ID, embedParams, embeddableFields, embeddableErrors)
	return
}

// parseNitradoTokenCommand func
func parseNitradoTokenCommand(command configs.Command, mc *discordgo.MessageCreate) (*NitradoTokenCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	return &NitradoTokenCommand{
		Params: NitradoTokenCommandParams{},
	}, nil
}

// ConvertToEmbedField for NitradoTokenOutput struct
func (nto *NitradoTokenOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("Discord Server ID: %s", nto.Guild.ID)

	return &discordgo.MessageEmbedField{
		Name:   "Add New Nitrado Tokens",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
