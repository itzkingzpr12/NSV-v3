package models

import "fmt"

// CreateChannelsReaction struct
type CreateChannelsReaction struct {
	Name               string     `json:"name"`
	Server             Server     `json:"server"`
	Reactions          []Reaction `json:"reactions"`
	Category           Channel    `json:"category"`
	User               *User      `json:"user"`
	AdminChannelName   string     `json:"admin_channel_name"`
	ChatChannelName    string     `json:"chat_channel_name"`
	PlayersChannelName string     `json:"players_channel_name"`
	KillsChannelName   string     `json:"kills_channel_name"`
}

// CacheKey func
func (cmr *CreateChannelsReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
