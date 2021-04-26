package models

import "fmt"

// OnlinePlayersOutputChannelMessages struct
type OnlinePlayersOutputChannelMessages struct {
	Messages []Message `json:"messages"`
	Channel  Channel   `json:"channel"`
}

// CacheKey func
func (cms *OnlinePlayersOutputChannelMessages) CacheKey(base, channelID string, serverID int64) string {
	return fmt.Sprintf("%s:%s:%d", base, channelID, serverID)
}
