package discordapi

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// AddReaction func
func AddReaction(session *discordgo.Session, channelID string, messageID string, emoji string) *Error {
	err := session.MessageReactionAdd(channelID, messageID, emoji)

	if err != nil {
		return ParseDiscordError(err)
	}

	return nil
}

// GenerateEmojiID func
func GenerateEmojiID(emojiName string, emojiID string, animated bool) string {
	emoji := ""
	if animated {
		emoji += "a"
	}

	emoji += fmt.Sprintf(":%s:%s", emojiName, emojiID)

	return emoji
}
