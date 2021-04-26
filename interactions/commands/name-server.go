package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/servers"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// NameServerCommand struct
type NameServerCommand struct {
	Params NameServerParams
}

// NameServerParams struct
type NameServerParams struct {
	ServerID int64
	Name     string
}

// NameServerOutput struct
type NameServerOutput struct {
	ServerOld gcscmodels.Server
	ServerNew gcscmodels.Server
}

// NameServer func
func (c *Commands) NameServer(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	nameServerCommand, nscErr := parseNameServerCommand(command, mc)
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

	var oldServer *gcscmodels.Server
	for _, server := range guildFeed.Payload.Guild.Servers {
		if server.NitradoID == nameServerCommand.Params.ServerID {
			oldServer = server
			break
		}
	}

	if oldServer == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Server does not exist with ID",
			Err:     errors.New("invalid server id"),
		})
		return
	}

	serverParamsBody := gcscmodels.UpdateServerRequest{
		BoostSettingID: oldServer.BoostSettingID,
		Enabled:        oldServer.Enabled,
		GuildID:        oldServer.GuildID,
		Name:           nameServerCommand.Params.Name,
		NitradoID:      oldServer.NitradoID,
		NitradoTokenID: oldServer.NitradoTokenID,
		ServerTypeID:   oldServer.ServerTypeID,
	}
	serverParams := servers.NewUpdateServerParamsWithTimeout(10)
	serverParams.Guild = mc.GuildID
	serverParams.ServerID = int64(oldServer.ID)
	serverParams.Context = context.Background()
	serverParams.Body = &serverParamsBody

	serverResponse, srErr := c.GuildConfigService.Client.Servers.UpdateServer(serverParams, c.GuildConfigService.Auth)
	if srErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to update server name",
			Err:     srErr,
		})
		return
	}

	if serverResponse.Payload == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to update server name",
			Err:     errors.New("server update has nil payload"),
		})
		return
	}

	if serverResponse.Payload.Server == nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to update server name",
			Err:     errors.New("server update has nil server"),
		})
		return
	}

	newServer := serverResponse.Payload.Server

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField
	embeddableFields = append(embeddableFields, &NameServerOutput{
		ServerOld: *oldServer,
		ServerNew: *newServer,
	})

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

// parseNameServerCommand func
func parseNameServerCommand(command configs.Command, mc *discordgo.MessageCreate) (*NameServerCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	serverIDInt, sidErr := strconv.ParseInt(splitContent[1], 10, 64)
	if sidErr != nil {
		return nil, &Error{
			Message: "Invalid Server ID",
			Err:     sidErr,
		}
	}

	return &NameServerCommand{
		Params: NameServerParams{
			ServerID: serverIDInt,
			Name:     strings.Join(splitContent[2:], " "),
		},
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (nso *NameServerOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("```Old Server Name:\n\t%s\nNew Server Name:\n\t%s```", nso.ServerOld.Name, nso.ServerNew.Name)
	return &discordgo.MessageEmbedField{
		Name:   "Updated Server Name",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
