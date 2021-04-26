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

// RemoveServerCommand struct
type RemoveServerCommand struct {
	Params RemoveServerParams
}

// RemoveServerParams struct
type RemoveServerParams struct {
	ServerID int64
}

// RemoveServerOutput struct
type RemoveServerOutput struct {
	Server gcscmodels.Server
}

// RemoveServer func
func (c *Commands) RemoveServer(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	removeServerCommand, rscErr := parseRemoveServerCommand(command, mc)
	if rscErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, *rscErr)
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
		if server.NitradoID == removeServerCommand.Params.ServerID {
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

	serverParams := servers.NewDeleteServerParamsWithTimeout(10)
	serverParams.Guild = mc.GuildID
	serverParams.ServerID = int64(oldServer.ID)
	serverParams.Context = context.Background()

	_, srErr := c.GuildConfigService.Client.Servers.DeleteServer(serverParams, c.GuildConfigService.Auth)
	if srErr != nil {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Failed to remove server",
			Err:     srErr,
		})
		return
	}

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField
	embeddableFields = append(embeddableFields, &RemoveServerOutput{
		Server: *oldServer,
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

// parseRemoveServerCommand func
func parseRemoveServerCommand(command configs.Command, mc *discordgo.MessageCreate) (*RemoveServerCommand, *Error) {
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

	return &RemoveServerCommand{
		Params: RemoveServerParams{
			ServerID: serverIDInt,
		},
	}, nil
}

// ConvertToEmbedField for NameServerOutput struct
func (rso *RemoveServerOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("```Removed Server Name:\n\t%s\nRemoved Server ID:\n\t%d```", rso.Server.Name, rso.Server.NitradoID)
	return &discordgo.MessageEmbedField{
		Name:   "Removed Server",
		Value:  fieldVal,
		Inline: false,
	}, nil
}
