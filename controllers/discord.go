package controllers

import (
	"errors"
	"net/http"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/viewmodels"
	"go.uber.org/zap"
)

// GetAllGuilds responds with all connected guilds from the cached state
func (c *Controller) GetAllGuilds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if !c.DiscordSession.StateEnabled {
		Error(ctx, w, "Discord state is not enabled", errors.New("discord state not enabled"), http.StatusNotFound)
	}

	guilds := c.DiscordSession.State.Guilds
	if guilds == nil {
		Error(ctx, w, "No guilds in Discord state", errors.New("discord state has no guilds"), http.StatusNotFound)
	}

	var outputGuilds []*viewmodels.SmallGuild
	for _, aGuild := range guilds {
		var guild viewmodels.SmallGuild
		guild.ID = aGuild.ID
		guild.MemberCount = aGuild.MemberCount
		guild.OwnerID = aGuild.OwnerID

		dGuild, dgErr := c.DiscordSession.Guild(aGuild.ID)
		if dgErr == nil {
			guild.Name = dGuild.Name
		}

		outputGuilds = append(outputGuilds, &guild)
	}

	Response(ctx, w, viewmodels.GetAllGuildsResponse{
		Message: "Found guilds",
		Count:   len(guilds),
		Guilds:  outputGuilds,
	}, http.StatusOK)
}

// GetAllGuilds responds with all connected guilds from the cached state
func (c *Controller) VerifySubscriberGuilds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if !c.DiscordSession.StateEnabled {
		Error(ctx, w, "Discord state is not enabled", errors.New("discord state not enabled"), http.StatusNotFound)
	}

	guilds := c.DiscordSession.State.Guilds
	if guilds == nil {
		Error(ctx, w, "No guilds in Discord state", errors.New("discord state has no guilds"), http.StatusNotFound)
	}

	var outputGuilds []*viewmodels.VerifiedSmallGuild
	var countVerified int = 0
	for _, aGuild := range guilds {
		var guild viewmodels.VerifiedSmallGuild
		guild.ID = aGuild.ID
		guild.MemberCount = aGuild.MemberCount
		guild.OwnerID = aGuild.OwnerID

		dGuild, dgErr := c.DiscordSession.Guild(aGuild.ID)
		if dgErr == nil {
			guild.Name = dGuild.Name
		}

		guildFeedOK, gfErr := guildconfigservice.GetGuildFeed(ctx, c.GuildConfigService, aGuild.ID)
		if gfErr == nil {
			if guildFeedOK.Payload != nil {
				if guildFeedOK.Payload.Guild != nil {
					if guildFeedOK.Payload.Guild.GuildServices != nil {
						for _, gs := range guildFeedOK.Payload.Guild.GuildServices {
							if !gs.Enabled {
								continue
							}

							if gs.Name == c.Config.Bot.GuildService {
								guild.NSMSubscriber = true
								countVerified++
							}
						}
					}
				}
			}
		}
		outputGuilds = append(outputGuilds, &guild)
	}

	Response(ctx, w, viewmodels.VerifySubscriberGuildsResponse{
		Message:         "Found guilds",
		Count:           len(guilds),
		VerifiedCount:   countVerified,
		UnverifiedCount: len(guilds) - countVerified,
		Guilds:          outputGuilds,
	}, http.StatusOK)
}
