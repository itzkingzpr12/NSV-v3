package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// ListServersCommand struct
type ListServersCommand struct {
	Params ListServersCommandParams
}

// ListServersCommandParams struct
type ListServersCommandParams struct{}

// ListServersOutput struct
type ListServersOutput struct {
	Server gcscmodels.Server
}

// ListServers func
func (c *Commands) ListServers(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	_, err := parseListServersCommand(command, mc)
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

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	for _, server := range guildFeed.Payload.Guild.Servers {
		var aServer = *server
		embeddableFields = append(embeddableFields, &ListServersOutput{
			Server: aServer,
		})
	}

	embedParams := discordapi.EmbeddableParams{
		Title:       command.Name,
		Description: command.Description,
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)
}

// parseListServersCommand func
func parseListServersCommand(command configs.Command, mc *discordgo.MessageCreate) (*ListServersCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	return &ListServersCommand{
		Params: ListServersCommandParams{},
	}, nil
}

// ConvertToEmbedField for ListServersOutput struct
func (lso *ListServersOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	fieldVal := fmt.Sprintf("**ID:** %d", lso.Server.NitradoID)

	var adminOutputChannel *gcscmodels.ServerOutputChannel
	var chatOutputChannel *gcscmodels.ServerOutputChannel
	var playersOutputChannel *gcscmodels.ServerOutputChannel
	if lso.Server.ServerOutputChannels != nil {
		for _, outputChannel := range lso.Server.ServerOutputChannels {
			if outputChannel.OutputChannelType == nil {
				continue
			}

			switch outputChannel.OutputChannelTypeID {
			case "admin":
				temp := *outputChannel
				adminOutputChannel = &temp
			case "chat":
				temp := *outputChannel
				chatOutputChannel = &temp
			case "players":
				temp := *outputChannel
				playersOutputChannel = &temp
			}
		}
	}

	if adminOutputChannel != nil {
		fieldVal += fmt.Sprintf("\n**Admin Logs:** <#%s>", adminOutputChannel.ChannelID)
	} else {
		fieldVal += "\n**Admin Logs:** Not Set"
	}

	if chatOutputChannel != nil {
		fieldVal += fmt.Sprintf("\n**Chat Logs:** <#%s>", chatOutputChannel.ChannelID)
	} else {
		fieldVal += "\n**Chat Logs:** Not Set"
	}

	if playersOutputChannel != nil {
		fieldVal += fmt.Sprintf("\n**Online Players:** <#%s>", playersOutputChannel.ChannelID)
	} else {
		fieldVal += "\n**Online Players:** Not Set"
	}

	return &discordgo.MessageEmbedField{
		Name:   lso.Server.Name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
