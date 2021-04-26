package discordapi

import "github.com/bwmarrin/discordgo"

// GetGuildRoles func
func GetGuildRoles(session *discordgo.Session, guildID string) ([]*discordgo.Role, *Error) {
	roles, rErr := session.GuildRoles(guildID)

	if rErr != nil {
		return nil, ParseDiscordError(rErr)
	}

	return roles, nil
}
