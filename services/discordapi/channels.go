package discordapi

import (
	"github.com/bwmarrin/discordgo"
)

// CreateDMChannel func
func CreateDMChannel(session *discordgo.Session, userID string) (*discordgo.Channel, *Error) {
	dmChannel, dmErr := session.UserChannelCreate(userID)
	if dmErr != nil {
		return nil, ParseDiscordError(dmErr)
	}

	return dmChannel, nil
}

// GetChannel func
func GetChannel(session *discordgo.Session, channelID string) (*discordgo.Channel, *Error) {
	channel, cErr := session.Channel(channelID)
	if cErr != nil {
		return nil, ParseDiscordError(cErr)
	}

	return channel, nil
}

// UpdateChannel func
func UpdateChannel(session *discordgo.Session, channelID string, data *discordgo.ChannelEdit) (*discordgo.Channel, *Error) {
	channel, cErr := session.ChannelEditComplex(channelID, data)

	if cErr != nil {
		return nil, ParseDiscordError(cErr)
	}

	return channel, nil
}

// DeleteChannel func
func DeleteChannel(session *discordgo.Session, channelID string) (*discordgo.Channel, *Error) {
	channel, cErr := session.ChannelDelete(channelID)

	if cErr != nil {
		return nil, ParseDiscordError(cErr)
	}

	return channel, nil
}

// CreateChannel func
func CreateChannel(session *discordgo.Session, guildID string, channelCreateData discordgo.GuildChannelCreateData) (*discordgo.Channel, *Error) {
	channel, err := session.GuildChannelCreateComplex(guildID, channelCreateData)
	if err != nil {
		return nil, ParseDiscordError(err)
	}

	return channel, nil
}
