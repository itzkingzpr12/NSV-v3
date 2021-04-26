package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gammazero/workerpool"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/discordapi"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	nsv2 "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

// SearchPlayersCommand struct
type SearchPlayersCommand struct {
	Params SearchPlayersCommandParams
}

// SearchPlayersCommandParams struct
type SearchPlayersCommandParams struct {
	PartialName string
}

// SearchPlayersSuccessOutput struct
type SearchPlayersSuccessOutput struct {
	Player  nsv2.Player
	Servers []gcscmodels.Server
}

// GetPlayersSuccess struct
type GetPlayersSuccess struct {
	Server  gcscmodels.Server
	Players []nsv2.Player
}

// GetPlayersError struct
type GetPlayersError struct {
	Message string
	Err     Error
}

// SearchPlayers func
func (c *Commands) SearchPlayers(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	parsedCommand, nscErr := parseSearchPlayersCommand(command, mc)
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
	for _, server := range guildFeed.Payload.Guild.Servers {
		if !server.Enabled {
			continue
		}

		aServer := *server
		servers = append(servers, aServer)
	}

	if len(servers) == 0 {
		c.ErrorOutput(ctx, command, mc.Content, mc.ChannelID, Error{
			Message: "Unable to find servers to search for player on",
			Err:     errors.New("no servers set up"),
		})
		return
	}

	wp := workerpool.New(command.Workers)
	defer wp.StopWait()

	successChannel := make(chan GetPlayersSuccess, len(guildFeed.Payload.Guild.Servers))
	errorChannel := make(chan GetPlayersError, len(guildFeed.Payload.Guild.Servers))

	go c.HandleGetPlayersResponses(ctx, s, mc, command, parsedCommand.Params.PartialName, len(servers), successChannel, errorChannel)

	for _, server := range servers {
		var aServer gcscmodels.Server = server
		wp.Submit(func() {
			c.GetPlayers(ctx, aServer, parsedCommand.Params.PartialName, successChannel, errorChannel)
		})
	}

	return
}

// parseSearchPlayersCommand func
func parseSearchPlayersCommand(command configs.Command, mc *discordgo.MessageCreate) (*SearchPlayersCommand, *Error) {
	splitContent := strings.Split(mc.Content, " ")

	if len(splitContent)-1 < command.MinArgs || len(splitContent)-1 > command.MaxArgs {
		return nil, &Error{
			Message: fmt.Sprintf("Command given %d arguments, expects %d to %d arguments.", len(splitContent)-1, command.MinArgs, command.MaxArgs),
			Err:     errors.New("invalid number of arguments"),
		}
	}

	playerName := strings.Join(splitContent[1:], " ")
	if len(playerName) < 3 {
		return nil, &Error{
			Message: "Player account name must be more than 3 characters long",
			Err:     errors.New("invalid player account name"),
		}
	}

	return &SearchPlayersCommand{
		Params: SearchPlayersCommandParams{
			PartialName: playerName,
		},
	}, nil
}

// HandleGetPlayersResponses func
func (c *Commands) HandleGetPlayersResponses(ctx context.Context, s *discordgo.Session, mc *discordgo.MessageCreate, command configs.Command, playerName string, servers int, getPlayersSuccess chan GetPlayersSuccess, getPlayersError chan GetPlayersError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	count := 0
	var successes []GetPlayersSuccess
	var errs []GetPlayersError

	var timer *time.Timer = time.NewTimer(120 * time.Second)

Loop:
	for {
		if count == servers {
			break
		}

		select {
		case success := <-getPlayersSuccess:
			count++
			successes = append(successes, success)
		case err := <-getPlayersError:
			count++
			errs = append(errs, err)
		case <-timer.C:
			break Loop
		}
	}

	var successOutputMap map[string]SearchPlayersSuccessOutput = make(map[string]SearchPlayersSuccessOutput, 0)

	for _, success := range successes {
		// FOR TESTING
		fmt.Println(success.Server.Name)
		for _, player := range success.Players {
			fmt.Println(player.Name)
		}
		fmt.Println("")

		aServer := success.Server
		for _, player := range success.Players {
			if player.Name == "" {
				continue
			}

			if val, ok := successOutputMap[player.Name]; ok {
				tempVal := val
				tempVal.Servers = append(tempVal.Servers, aServer)
				successOutputMap[player.Name] = tempVal
			} else {
				aPlayer := player
				successOutputMap[player.Name] = SearchPlayersSuccessOutput{
					Player: aPlayer,
					Servers: []gcscmodels.Server{
						aServer,
					},
				}
			}
		}
	}

	var keys []string
	for k := range successOutputMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var embeddableFields []discordapi.EmbeddableField
	var embeddableErrors []discordapi.EmbeddableField

	for _, k := range keys {
		output := successOutputMap[k]
		embeddableFields = append(embeddableFields, &output)
	}

	embedParams := discordapi.EmbeddableParams{
		Title:       fmt.Sprintf("Searched Players By Account: %s", playerName),
		Description: fmt.Sprintf("Found %d players across %d servers.", len(keys), len(successes)),
		TitleURL:    c.Config.Bot.DocumentationURL,
		Footer:      fmt.Sprintf("Executed by %s", mc.Author.Username),
	}

	c.Output(ctx, mc.ChannelID, embedParams, embeddableFields, embeddableErrors)
	return
}

// GetPlayers func
func (c *Commands) GetPlayers(ctx context.Context, server gcscmodels.Server, playerName string, getPlayersSuccess chan GetPlayersSuccess, getPlayersError chan GetPlayersError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	players, err := c.NitradoService.Client.SearchPlayers(server.NitradoToken.Token, fmt.Sprint(server.NitradoID), playerName, false, false, true)
	if err != nil {
		getPlayersError <- GetPlayersError{
			Message: err.Message(),
			Err: Error{
				Message: err.Message(),
				Err:     err,
			},
		}
		return
	}

	getPlayersSuccess <- GetPlayersSuccess{
		Server:  server,
		Players: players.Players,
	}
	return
}

// ConvertToEmbedField for NameServerOutput struct
func (out *SearchPlayersSuccessOutput) ConvertToEmbedField() (*discordgo.MessageEmbedField, *discordapi.Error) {
	name := "🔴 " + out.Player.Name

	if out.Player.Online {
		name = "🟢 " + out.Player.Name
	}

	if name == "" {
		name = "Unknown Player"
	}

	fieldVal := ""

	if !out.Player.Online {
		fieldVal += fmt.Sprintf("\n**Last Online:** %s", out.Player.LastOnline)
	}

	if len(out.Servers) > 0 {
		fieldVal += "\n**Server(s):**"
	}

	for _, server := range out.Servers {
		fieldVal += fmt.Sprintf("\n\t%s", server.Name)
	}

	if fieldVal == "" {
		fieldVal = "Information Unknown\n\u200b"
	} else {
		fieldVal += "\n\u200b"
	}

	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fieldVal,
		Inline: false,
	}, nil
}
