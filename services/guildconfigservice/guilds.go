package guildconfigservice

import (
	"context"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guilds"
)

// GetAllGuilds func
func GetAllGuilds(ctx context.Context, gcs *GuildConfigService) (*guilds.GetAllGuildsOK, *Error) {
	guildParams := guilds.NewGetAllGuildsParamsWithTimeout(30)
	guildParams.Context = context.Background()

	guild, gfErr := gcs.Client.Guilds.GetAllGuilds(guildParams, gcs.Auth)
	if gfErr != nil {
		return guild, &Error{
			Message: "Failed to get all guilds",
			Err:     gfErr,
		}
	}

	return guild, nil
}
