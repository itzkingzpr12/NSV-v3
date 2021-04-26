package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/nitrado_setups"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// SetupCommand struct
type SetupCommand struct {
	Params SetupCommandParams
}

// SetupCommandParams struct
type SetupCommandParams struct{}

// SetupOutput struct
type SetupOutput struct {
	NitradoSetup gcscmodels.CreateNitradoSetupResponse
}

// SetupProcessingOutput struct
type SetupProcessingOutput struct{}

// Setup func
func (c *Commands) Setup(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, scErr := parseSetupCommand(command, mc)
	if scErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *scErr)
		return
	}

	// Output "Processing Setup" message
	setupProcessingOutput := SetupProcessingOutput{}

	var ef []discordapi.EmbeddableField
	var ee []discordapi.EmbeddableField

	ef = append(ef, &setupProcessingOutput)

	embedParams := discordapi.EmbeddableParams{
		Title:       command.Name,
		Description: command.Description,
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	if len(ee) == 0 {
		embedParams.ThumbnailURL = c.Config.Bot.WorkingThumbnail
	} else {
		embedParams.ThumbnailURL = c.Config.Bot.WarnThumbnail
	}

	_, spoErr := c.Output(ctx, mc.ChannelID, embedParams, ef, ee)
	if spoErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to send setup processing message",
			Err:     spoErr,
		})
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

	dGuild, dgErr := s.Guild(mc.GuildID)
	if dgErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", dgErr), zap.String("error_message", "unable to get Discord server information"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to get Discord server information",
			Err:     dgErr,
		})
		return
	}

	nitradoSetupBody := gcscmodels.CreateNitradoSetupRequest{
		GuildID:          mc.GuildID,
		GuildName:        dGuild.Name,
		ContactID:        mc.Author.ID,
		ContactName:      mc.Author.Username,
		GuildServiceName: c.Config.Bot.GuildService,
	}
	createNitradoSetupParams := nitrado_setups.NewCreateNitradoSetupParamsWithTimeout(300)
	createNitradoSetupParams.SetContext(context.Background())
	createNitradoSetupParams.SetGuild(mc.GuildID)
	createNitradoSetupParams.SetBody(&nitradoSetupBody)
	nitradoSetupResponse, cnsErr := c.GuildConfigService.Client.NitradoSetups.CreateNitradoSetup(createNitradoSetupParams, c.GuildConfigService.Auth)
	if cnsErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cnsErr), zap.String("error_message", "unable to create nitrado setup"))
		logger := logging.Logger(ctx)
		logger.Error("error_log")

		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to setup Nitrado Server Manager V2",
			Err:     errors.New("failed bot setup"),
		})
		return
	}

	setupOutput := SetupOutput{
		NitradoSetup: *nitradoSetupResponse.Payload,
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	embeddableFields = append(embeddableFields, &setupOutput)

	embedParams2 := discordapi.EmbeddableParams{
		Title:       "Finished Setup",
		Description: "Setup has been completed and your servers have been linked to the bot. Please continue with any post-setup steps that are remaining.",
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	if len(embeddableErrors) == 0 {
		embedParams.ThumbnailURL = c.Config.Bot.OkThumbnail
	} else {
		embedParams.ThumbnailURL = c.Config.Bot.WarnThumbnail
	}

	c.Output(ctx, mc.ChannelID, embedParams2, embeddableFields, embeddableErrors)
	return
}

// parseListServersCommand func
func parseSetupCommand(command configs.Command, mc *discordgo.MessageCreate) (*SetupCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	return &SetupCommand{
		Params: SetupCommandParams{},
	}, nil
}

// ConvertToEmbedField for Error struct
func (so *SetupOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("Nitrado Accounts: %d\nNew Servers: %d", len(so.NitradoSetup.NitradoTokens), len(so.NitradoSetup.Servers))

	return &discordgo.MessageEmbedField{
		Name:   "Successful Setup",
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for Error struct
func (spo *SetupProcessingOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	return &discordgo.MessageEmbedField{
		Name:   "Setup in Progress",
		Value:  "Please wait while the bot retrieves all of your servers on the linked Nitrado account(s). This may take up to 1 minute.",
		Inline: false,
	}, nil
}
