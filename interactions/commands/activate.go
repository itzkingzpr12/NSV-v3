package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guild_feeds"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guild_services"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guilds"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// ActivateCommand struct
type ActivateCommand struct {
	Params ActivateCommandParams
}

// ActivateCommandParams struct
type ActivateCommandParams struct {
	Token string
}

// ActivateOutput struct
type ActivateOutput struct {
	GuildCreated          bool
	GuildServiceCreated   bool
	GuildServiceActivated bool
}

// Activate func
func (c *Commands) Activate(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	isAdmin, iaErr := c.IsAdmin(ctx, mc.GuildID, mc.Member.Roles)
	if iaErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *iaErr)
		return
	}
	if !isAdmin {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unauthorized to use this command",
			Err:     errors.New("user is not administrator"),
		})
		return
	}

	activateCommand, acErr := parseActivateCommand(command, mc)
	if acErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *acErr)
		return
	}

	var activationToken *models.ActivationToken
	cacheKey := activationToken.CacheKey(c.Config.CacheSettings.ActivationToken.Base, activateCommand.Params.Token)
	stcErr := c.Cache.GetStruct(ctx, cacheKey, &activationToken)
	if stcErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", stcErr.Err), zap.String("error_message", stcErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: stcErr.Message,
			Err:     stcErr.Err,
		})
		return
	}

	if activationToken == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "You must provide a valid setup token to do your initial setup",
			Err:     errors.New("invalid activation token"),
		})
		return
	}

	guildNeedsCreation := false
	guildNeedsGuildService := false
	guildNeedsGuildServiceActivation := false
	var guildService *gcscmodels.GuildService

	guildFeedParams := guild_feeds.NewGetGuildFeedByIDParamsWithTimeout(30)
	guildFeedParams.Guild = mc.GuildID
	guildFeedParams.GuildID = mc.GuildID
	guildFeedParams.Context = context.Background()

	guildFeed, gfErr := c.GuildConfigService.Client.GuildFeeds.GetGuildFeedByID(guildFeedParams, c.GuildConfigService.Auth)
	if gfErr != nil {
		if _, ok := gfErr.(*guild_feeds.GetGuildFeedByIDNotFound); !ok {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to find bot information",
				Err:     gfErr,
			})
			return
		}

		guildNeedsCreation = true
		guildNeedsGuildService = true
	} else {
		if guildFeed.Payload == nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to find bot information",
				Err:     errors.New("guild feed nil payload"),
			})
			return
		}

		if guildFeed.Payload.Guild == nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to find bot information",
				Err:     errors.New("guild feed nil guild"),
			})
			return
		}

		if guildFeed.Payload.Guild.Enabled == false {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Discord server not enabled to use BIC Development bots",
				Err:     errors.New("guild disabled"),
			})
			return
		}

		if guildFeed.Payload.Guild.GuildServices == nil {
			guildNeedsGuildService = true
		} else {
			guildServiceExists := false
			for _, aGuildService := range guildFeed.Payload.Guild.GuildServices {
				if aGuildService.Name == c.Config.Bot.GuildService {
					guildService = aGuildService
					guildServiceExists = true
					if aGuildService.Enabled == false {
						guildNeedsGuildServiceActivation = true
					}
					break
				}
			}

			if !guildServiceExists {
				guildNeedsGuildService = true
			}
		}
	}

	dGuild, dgErr := s.Guild(mc.GuildID)
	if dgErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to get Discord server information",
			Err:     dgErr,
		})
		return
	}

	if guildNeedsCreation {
		guildBody := gcscmodels.Guild{
			ID:      mc.GuildID,
			Name:    dGuild.Name,
			Enabled: true,
		}
		createGuildParams := guilds.NewCreateGuildParamsWithTimeout(10)
		createGuildParams.SetContext(context.Background())
		createGuildParams.SetBody(&guildBody)
		_, cgErr := c.GuildConfigService.Client.Guilds.CreateGuild(createGuildParams, c.GuildConfigService.Auth)
		if cgErr != nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to add Discord server to bot",
				Err:     cgErr,
			})
			return
		}
	}

	if guildNeedsGuildService {
		guildServiceBody := gcscmodels.GuildService{
			GuildID: mc.GuildID,
			Name:    c.Config.Bot.GuildService,
			Enabled: true,
		}
		createGuildServiceParams := guild_services.NewCreateGuildServiceParamsWithTimeout(10)
		createGuildServiceParams.SetContext(context.Background())
		createGuildServiceParams.SetGuild(mc.GuildID)
		createGuildServiceParams.SetBody(&guildServiceBody)
		_, cgsErr := c.GuildConfigService.Client.GuildServices.CreateGuildService(createGuildServiceParams, c.GuildConfigService.Auth)
		if cgsErr != nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to enable Nitrado Server Manager V2",
				Err:     cgsErr,
			})
			return
		}
	} else if guildNeedsGuildServiceActivation {
		guildServiceBody := gcscmodels.UpdateGuildServiceRequest{
			Enabled: true,
			GuildID: guildService.GuildID,
			Name:    guildService.Name,
		}
		updateGuildServiceParams := guild_services.NewUpdateGuildServiceParamsWithTimeout(10)
		updateGuildServiceParams.SetGuild(mc.GuildID)
		updateGuildServiceParams.SetGuildServiceID(int64(guildService.ID))
		updateGuildServiceParams.SetContext(context.Background())
		updateGuildServiceParams.SetBody(&guildServiceBody)

		_, ugsErr := c.GuildConfigService.Client.GuildServices.UpdateGuildService(updateGuildServiceParams, c.GuildConfigService.Auth)
		if ugsErr != nil {
			c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
				Message: "Failed to enable Nitrado Server Manager V2",
				Err:     ugsErr,
			})
			return
		}
	}

	atExpErr := c.Cache.Expire(ctx, cacheKey)
	if atExpErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", atExpErr.Err), zap.String("error_message", atExpErr.Message))
		logger := logging.Logger(ctx)
		logger.Error("error_log")
	}

	activateOutput := ActivateOutput{
		GuildCreated:          guildNeedsCreation,
		GuildServiceCreated:   guildNeedsGuildService,
		GuildServiceActivated: guildNeedsGuildServiceActivation,
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &activateOutput)

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
}

// parseActivateCommand func
func parseActivateCommand(command configs.Command, mc *discordgo.MessageCreate) (*ActivateCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	return &ActivateCommand{
		Params: ActivateCommandParams{
			Token: splitContent[1],
		},
	}, nil
}

// ConvertToEmbedField for Error struct
func (ao *ActivateOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := "Successful Activation"
	fieldVal := ""

	if ao.GuildCreated {
		fieldVal += "Discord Server Added to Bot\n"
	}

	if ao.GuildServiceCreated || ao.GuildServiceActivated {
		fieldVal += "Nitrado Server Manager V2 Activated"
	}

	if fieldVal == "" {
		fieldVal = "Nitrado Server Manager V2 is already activated for your Discord"
		name = "Already Activated"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
