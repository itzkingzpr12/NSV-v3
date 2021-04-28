package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gammazero/workerpool"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	nitrado_service_v2_client "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

// GetWhitelistCommand struct
type GetWhitelistCommand struct {
	Params GetWhitelistCommandParams
}

// GetWhitelistCommandParams struct
type GetWhitelistCommandParams struct {
	ServerID int64
}

// GetWhitelistCommandConfirmationOutput struct
type GetWhitelistCommandConfirmationOutput struct {
	Servers []gcscmodels.Server
}

type GetWhitelistSuccess struct {
	Players []nitrado_service_v2_client.Player
	Server  gcscmodels.Server
}

type GetWhitelistError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

type GetWhitelistSuccessOutput struct {
	Players []nitrado_service_v2_client.Player
	Server  gcscmodels.Server
}

type GetWhitelistErrorOutput struct {
	Message string
	Servers []gcscmodels.Server
}

// GetWhitelist func
func (c *Commands) GetWhitelist(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseGetWhitelistCommand(command, mc)
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

	var servers []gcscmodels.Server
	for _, aServer := range guildFeed.Payload.Guild.Servers {
		if !aServer.Enabled {
			continue
		}

		if aServer.ServerTypeID != "arkps" {
			continue
		}

		if parsedCommand.Params.ServerID != 0 {
			if parsedCommand.Params.ServerID == aServer.NitradoID {
				servers = append(servers, *aServer)
				break
			}
			continue
		}

		servers = append(servers, *aServer)
	}

	if len(servers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find PS servers to get whitelist",
			Err:     errors.New("invalid server id or no PS servers set up"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan GetWhitelistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan GetWhitelistError, len(guildFeed.Payload.Guild.Servers))

	go c.HandleGetWhitelistResponses(ctx, s, mc, command, len(servers), successChannel, errorChannel)

	for _, stb := range servers {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			c.GetWhitelistRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	return
}

// parseGetWhitelistCommand func
func parseGetWhitelistCommand(command configs.Command, mc *discordgo.MessageCreate) (*GetWhitelistCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	if len(splitContent) == 1 {
		return &GetWhitelistCommand{
			Params: GetWhitelistCommandParams{},
		}, nil
	}

	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		return nil, &Error{
			Message: fmt.Sprintf("Unable to get whitelist due to invalid server ID"),
			Err:     errors.New("invalid server id"),
		}
	}

	return &GetWhitelistCommand{
		Params: GetWhitelistCommandParams{
			ServerID: serverIDInt,
		},
	}, nil
}

// GetWhitelistRequest func
func (c *Commands) GetWhitelistRequest(ctx context.Context, server gcscmodels.Server, getWhitelistSuccess chan GetWhitelistSuccess, getWhitelistError chan GetWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	resp, err := c.NitradoService.Client.GetWhitelist(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), false)
	if err != nil {
		getWhitelistError <- GetWhitelistError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	getWhitelistSuccess <- GetWhitelistSuccess{
		Players: resp.Players,
		Server:  server,
	}
	return
}

// HandleGetWhitelistResponses func
func (c *Commands) HandleGetWhitelistResponses(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command, servers int, getWhitelistSuccess chan GetWhitelistSuccess, getWhitelistError chan GetWhitelistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []GetWhitelistSuccess
	var errs []GetWhitelistError

	var timer *time.Timer = time.NewTimer(120 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-getWhitelistSuccess:
			count++
			successes = append(successes, success)
		case err := <-getWhitelistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var getWhitelistErrorTypes map[string]GetWhitelistErrorOutput = make(map[string]GetWhitelistErrorOutput)

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	for _, success := range successes {
		characterCount := len(success.Server.Name)

		var getWhitelistSuccessOutput GetWhitelistSuccessOutput
		getWhitelistSuccessOutput.Server = success.Server
		for _, player := range success.Players {
			if characterCount+len(player.Name) >= 800 {
				var tempGetWhitelistSuccessOutput GetWhitelistSuccessOutput = getWhitelistSuccessOutput
				embeddableFields = append(embeddableFields, &tempGetWhitelistSuccessOutput)
				getWhitelistSuccessOutput = GetWhitelistSuccessOutput{}
				characterCount = len(success.Server.Name)
			}

			getWhitelistSuccessOutput.Server = success.Server
			getWhitelistSuccessOutput.Players = append(getWhitelistSuccessOutput.Players, player)
			characterCount += len(player.Name)
		}

		embeddableFields = append(embeddableFields, &getWhitelistSuccessOutput)
	}

	for _, err := range errs {
		if val, ok := getWhitelistErrorTypes[err.Error]; ok {
			servers := append(val.Servers, err.Server)
			getWhitelistErrorTypes[err.Error] = GetWhitelistErrorOutput{
				Message: err.Error,
				Servers: servers,
			}
		} else {
			getWhitelistErrorTypes[err.Error] = GetWhitelistErrorOutput{
				Message: err.Error,
				Servers: []gcscmodels.Server{
					err.Server,
				},
			}
		}
	}

	for _, err := range getWhitelistErrorTypes {
		var anErr GetWhitelistErrorOutput = err
		embeddableErrors = append(embeddableErrors, &anErr)
	}

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

// ConvertToEmbedField for GetWhitelistErrorOutput struct
func (bpe *GetWhitelistErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := ""
	name := bpe.Message
	if name == "" {
		name = "Failed to Get Whitelist"
	}

	for _, server := range bpe.Servers {
		fieldVal += server.Name + "\n"
	}

	if fieldVal == "" {
		fieldVal = "Unknown servers"
	} else if len(fieldVal) > 800 {
		fieldVal = fieldVal[:800]
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for GetWhitelistSuccessOutput struct
func (bps *GetWhitelistSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := bps.Server.Name
	fieldVal := "```"

	for _, player := range bps.Players {
		if player.Name == "" {
			continue
		}

		fieldVal += "\n" + player.Name
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Players Whitelisted"
	}

	fieldVal += "\n```"

	if name == "" {
		name = fmt.Sprint(bps.Server.NitradoID)
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
