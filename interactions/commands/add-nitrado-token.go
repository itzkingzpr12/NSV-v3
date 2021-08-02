package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/nitrado_tokens"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	nsv2 "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

// AddNitradoTokenCommand struct
type AddNitradoTokenCommand struct {
	Params AddNitradoTokenCommandParams
}

// AddNitradoTokenCommandParams struct
type AddNitradoTokenCommandParams struct {
	Token string
}

// AddNitradoTokenOutput struct
type AddNitradoTokenOutput struct {
	Guild           models.Guild `json:"guild"`
	NitradoTokenKey string       `json:"nitrado_token_key"`
	IsNew           bool         `json:"is_new"`
}

// AddNitradoToken func
func (c *Commands) AddNitradoToken(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if mc.GuildID != "" {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Only usable in DM with the bot for security",
			Err:     fmt.Errorf("Use the `%snitradotoken` command in your Discord to start a DM with the bot", c.Config.Bot.Prefix),
		})
		return
	}

	addNitradoTokenCommand, err := parseAddNitradoTokenCommand(command, mc)
	if err != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *err)
		return
	}

	var nitradoTokenGuild *models.NitradoTokenGuild

	cacheKey := nitradoTokenGuild.CacheKey(c.Config.CacheSettings.NitradoTokenGuild.Base, mc.Author.ID)
	getCacheErr := c.Cache.GetStruct(ctx, cacheKey, &nitradoTokenGuild)
	if getCacheErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: getCacheErr.Message,
			Err:     getCacheErr.Err,
		})
		return
	}

	if nitradoTokenGuild == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "You haven't initiated a Nitrado Token addition request yet",
			Err:     fmt.Errorf("Before you can use this command, you must first run `%snitradotoken` in your Discord. That will open a DM where you can reply with a new token", c.Config.Bot.Prefix),
		})
		return
	}

	ctx = logging.AddValues(ctx, zap.String("guild_id", nitradoTokenGuild.Guild.ID))

	guildFeed, gfErr := guildconfigservice.GetGuildFeed(ctx, c.GuildConfigService, nitradoTokenGuild.Guild.ID)
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

	var accounts map[string][]int64 = make(map[string][]int64)
	if guildFeed.Payload.Guild.NitradoTokens != nil {
		for _, nitradoToken := range guildFeed.Payload.Guild.NitradoTokens {
			accounts[nitradoToken.Token] = []int64{}
		}
	}

	if guildFeed.Payload.Guild.Servers != nil {
		for _, server := range guildFeed.Payload.Guild.Servers {
			if server.NitradoToken == nil {
				continue
			}

			if _, ok := accounts[server.NitradoToken.Token]; !ok {
				continue
			}

			accounts[server.NitradoToken.Token] = append(accounts[server.NitradoToken.Token], server.NitradoID)
		}
	}

	createResponse, createErr := c.NitradoService.Client.UpdateNitradoToken(nsv2.UpdateNitradoTokenRequest{
		NitradoToken: addNitradoTokenCommand.Params.Token,
		Accounts:     accounts,
	})
	if createErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: createErr.Message(),
			Err:     createErr,
		})
		return
	}

	if createResponse.Error != "" {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to add Nitrado Token",
			Err:     errors.New(createResponse.Error),
		})
		return
	}

	isNewToken := false
	if createResponse.Message == "Successfully added Nitrado Token" {
		isNewToken = true
		body := gcscmodels.NitradoToken{
			GuildID: nitradoTokenGuild.Guild.ID,
			Token:   createResponse.NitradoTokenKey,
			Enabled: true,
		}

		createNitradoTokenParams := nitrado_tokens.NewCreateNitradoTokenParamsWithTimeout(10)
		createNitradoTokenParams.SetGuild(nitradoTokenGuild.Guild.ID)
		createNitradoTokenParams.SetContext(context.Background())
		createNitradoTokenParams.SetBody(&body)

		_, cntErr := c.GuildConfigService.Client.NitradoTokens.CreateNitradoToken(createNitradoTokenParams, c.GuildConfigService.Auth)
		if cntErr != nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to add Nitrado Token to Guild Config",
				Err:     cntErr,
			})
			return
		}
	}

	anto := AddNitradoTokenOutput{
		Guild:           nitradoTokenGuild.Guild,
		NitradoTokenKey: createResponse.NitradoTokenKey,
		IsNew:           isNewToken,
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &anto)

	embedParams := discordapi.EmbeddableParams{
		Title:       command.Name,
		Description: command.Description,
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	if len(embeddableErrors) == 0 {
		embedParams.ThumbnailURL = c.Config.Bot.OkThumbnail
	} else {
		embedParams.ThumbnailURL = c.Config.Bot.WarnThumbnail
	}

	c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)
	return
}

// parseAddNitradoTokenCommand func
func parseAddNitradoTokenCommand(command configs.Command, mc *discordgo.MessageCreate) (*AddNitradoTokenCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	return &AddNitradoTokenCommand{
		Params: AddNitradoTokenCommandParams{
			Token: splitContent[1],
		},
	}, nil
}

// ConvertToEmbedField for NitradoTokenOutput struct
func (nto *AddNitradoTokenOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := "Added New Nitrado Token"
	if !nto.IsNew {
		name = "Updated Nitrado Token"
	}

	fieldVal := fmt.Sprintf("Discord Server ID: %s\nNitrado Token Key: %s", nto.Guild.ID, nto.NitradoTokenKey)

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
