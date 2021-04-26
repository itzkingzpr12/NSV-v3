package guildconfigservice

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guild_feeds"
)

// GetGuildFeed func
func GetGuildFeed(ctx context.Context, gcs *GuildConfigService, guildID string) (*guild_feeds.GetGuildFeedByIDOK, *Error) {
	guildFeedParams := guild_feeds.NewGetGuildFeedByIDParamsWithTimeout(30)
	guildFeedParams.Guild = guildID
	guildFeedParams.GuildID = guildID
	guildFeedParams.Context = context.Background()

	guildFeed, gfErr := gcs.Client.GuildFeeds.GetGuildFeedByID(guildFeedParams, gcs.Auth)
	if gfErr != nil {
		if val, ok := gfErr.(*guild_feeds.GetGuildFeedByIDNotFound); ok {
			return guildFeed, &Error{
				Message: "Discord server not set up",
				Err:     fmt.Errorf("%s\n%s", val.Payload.Message, val.Payload.Error),
			}
		}

		return guildFeed, &Error{
			Message: "Failed to get guild feed",
			Err:     gfErr,
		}
	}

	return guildFeed, nil
}

// ValidateGuildFeed func
func ValidateGuildFeed(guildFeed *guild_feeds.GetGuildFeedByIDOK, guildService string, validation string) *Error {
	if guildFeed.Payload == nil {
		return &Error{
			Message: "Failed to retrieve bot information",
			Err:     errors.New("guild feed has nil payload"),
		}
	}

	if validation == "Payload" {
		return nil
	}

	if guildFeed.Payload.Guild == nil {
		return &Error{
			Message: "Failed to retrieve bot information",
			Err:     errors.New("guild feed has nil guild"),
		}
	}

	if validation == "Guild" {
		return nil
	}

	if guildFeed.Payload.Guild.GuildServices == nil {
		return &Error{
			Message: "Bot has not been activated",
			Err:     errors.New("guild feed nil guild services"),
		}
	}

	guildServiceExists := false
	for _, aGuildService := range guildFeed.Payload.Guild.GuildServices {
		if aGuildService.Name == guildService {
			guildServiceExists = true
			if aGuildService.Enabled == false {
				return &Error{
					Message: "Discord server is not enabled to use this bot",
					Err:     errors.New("disabled guild service"),
				}
			}
		}
	}

	if !guildServiceExists {
		return &Error{
			Message: "Discord server is not enabled to use this bot",
			Err:     errors.New("no guild service"),
		}
	}

	if validation == "GuildServices" {
		return nil
	}

	if guildFeed.Payload.Guild.NitradoTokens == nil {
		return &Error{
			Message: "Failed to retrieve bot information",
			Err:     errors.New("guild feed has nil nitrado tokens"),
		}
	}

	if validation == "NitradoTokens" {
		return nil
	}

	if guildFeed.Payload.Guild.Servers == nil {
		return &Error{
			Message: "Failed to retrieve bot information",
			Err:     errors.New("guild feed has nil servers"),
		}
	}

	if validation == "Servers" {
		return nil
	}

	return nil
}
