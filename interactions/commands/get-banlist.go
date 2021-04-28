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

// GetBanlistCommand struct
type GetBanlistCommand struct {
	Params GetBanlistCommandParams
}

// GetBanlistCommandParams struct
type GetBanlistCommandParams struct {
	ServerID int64
}

// GetBanlistCommandConfirmationOutput struct
type GetBanlistCommandConfirmationOutput struct {
	Servers []gcscmodels.Server
}

type GetBanlistSuccess struct {
	Players []nitrado_service_v2_client.Player
	Server  gcscmodels.Server
}

type GetBanlistError struct {
	Server  gcscmodels.Server
	Message string
	Error   string
}

type GetBanlistSuccessOutput struct {
	Players []nitrado_service_v2_client.Player
	Server  gcscmodels.Server
}

type GetBanlistErrorOutput struct {
	Message string
	Servers []gcscmodels.Server
}

// GetBanlist func
func (c *Commands) GetBanlist(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseGetBanlistCommand(command, mc)
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
			Message: "Unable to find servers to get banlist",
			Err:     errors.New("invalid server id or no servers set up"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan GetBanlistSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan GetBanlistError, len(guildFeed.Payload.Guild.Servers))

	go c.HandleGetBanlistResponses(ctx, s, mc, command, len(servers), successChannel, errorChannel)

	for _, stb := range servers {
		var aServer gcscmodels.Server = stb
		wp.Submit(func() {
			c.GetBanlistRequest(ctx, aServer, successChannel, errorChannel)
		})
	}

	return
}

// parseGetBanlistCommand func
func parseGetBanlistCommand(command configs.Command, mc *discordgo.MessageCreate) (*GetBanlistCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	if len(splitContent) == 1 {
		return &GetBanlistCommand{
			Params: GetBanlistCommandParams{},
		}, nil
	}

	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		return nil, &Error{
			Message: fmt.Sprintf("Unable to get banlist due to invalid server ID"),
			Err:     errors.New("invalid server id"),
		}
	}

	return &GetBanlistCommand{
		Params: GetBanlistCommandParams{
			ServerID: serverIDInt,
		},
	}, nil
}

// GetBanlistRequest func
func (c *Commands) GetBanlistRequest(ctx context.Context, server gcscmodels.Server, getBanlistSuccess chan GetBanlistSuccess, getBanlistError chan GetBanlistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	resp, err := c.NitradoService.Client.GetBanlist(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), false)
	if err != nil {
		getBanlistError <- GetBanlistError{
			Server:  server,
			Message: err.Message(),
			Error:   err.Error(),
		}
		return
	}

	getBanlistSuccess <- GetBanlistSuccess{
		Players: resp.Players,
		Server:  server,
	}
	return
}

// HandleGetBanlistResponses func
func (c *Commands) HandleGetBanlistResponses(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command, servers int, getBanlistSuccess chan GetBanlistSuccess, getBanlistError chan GetBanlistError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []GetBanlistSuccess
	var errs []GetBanlistError

	var timer *time.Timer = time.NewTimer(120 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-getBanlistSuccess:
			count++
			successes = append(successes, success)
		case err := <-getBanlistError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var getBanlistErrorTypes map[string]GetBanlistErrorOutput = make(map[string]GetBanlistErrorOutput)

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	for _, success := range successes {
		characterCount := len(success.Server.Name)

		var getBanlistSuccessOutput GetBanlistSuccessOutput
		getBanlistSuccessOutput.Server = success.Server
		for _, player := range success.Players {
			if characterCount+len(player.Name) >= 800 {
				var tempGetBanlistSuccessOutput GetBanlistSuccessOutput = getBanlistSuccessOutput
				embeddableFields = append(embeddableFields, &tempGetBanlistSuccessOutput)
				getBanlistSuccessOutput = GetBanlistSuccessOutput{}
				characterCount = len(success.Server.Name)
			}

			getBanlistSuccessOutput.Server = success.Server
			getBanlistSuccessOutput.Players = append(getBanlistSuccessOutput.Players, player)
			characterCount += len(player.Name)
		}

		embeddableFields = append(embeddableFields, &getBanlistSuccessOutput)
	}

	for _, err := range errs {
		if val, ok := getBanlistErrorTypes[err.Error]; ok {
			servers := append(val.Servers, err.Server)
			getBanlistErrorTypes[err.Error] = GetBanlistErrorOutput{
				Message: err.Error,
				Servers: servers,
			}
		} else {
			getBanlistErrorTypes[err.Error] = GetBanlistErrorOutput{
				Message: err.Error,
				Servers: []gcscmodels.Server{
					err.Server,
				},
			}
		}
	}

	for _, err := range getBanlistErrorTypes {
		var anErr GetBanlistErrorOutput = err
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

// ConvertToEmbedField for GetBanlistErrorOutput struct
func (bpe *GetBanlistErrorOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := "```"
	name := bpe.Message
	if bpe.Message == "" {
		name = "Failed to Get Banlist"
	}

	for _, server := range bpe.Servers {
		fieldVal += "\n" + server.Name
	}

	if fieldVal == "```" {
		fieldVal = "\nUnknown servers"
	} else if len(fieldVal) > 800 {
		fieldVal = fieldVal[:800]
	}

	fieldVal += "\n```"

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}

// ConvertToEmbedField for GetBanlistSuccessOutput struct
func (bps *GetBanlistSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := bps.Server.Name
	fieldVal := "```"

	for _, player := range bps.Players {
		fieldVal += "\n" + player.Name
	}

	if fieldVal == "```" {
		fieldVal += "\nNo Players Banned"
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
