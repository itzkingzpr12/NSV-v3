package discordapi

import (
	"github.com/bwmarrin/discordgo"
)

// SendMessage func
func SendMessage(session *discordgo.Session, channelID string, content *string, embed *discordgo.MessageEmbed) (*discordgo.Message, *Error) {
	messageSend := &discordgo.MessageSend{
		Embed: embed,
	}
	if content != nil {
		messageSend.Content = *content
	}

	message, err := session.ChannelMessageSendComplex(channelID, messageSend)

	if err != nil {
		return nil, ParseDiscordError(err)
	}

	return message, nil
}

// EditMessage func
func EditMessage(session *discordgo.Session, channelID string, messageID string, content *string, embed *discordgo.MessageEmbed) (*discordgo.Message, *Error) {
	message, err := session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Content: content,
		Embed:   embed,
		ID:      messageID,
		Channel: channelID,
	})

	if err != nil {
		return nil, ParseDiscordError(err)
	}

	return message, nil
}

// DeleteMessage func
func DeleteMessage(session *discordgo.Session, channelID string, messageID string) *Error {
	err := session.ChannelMessageDelete(channelID, messageID)
	if err != nil {
		return ParseDiscordError(err)
	}

	return nil
}
