package models

import "fmt"

// SetOutputReaction struct
type SetOutputReaction struct {
	Server                       Server     `json:"server"`
	Reactions                    []Reaction `json:"reactions"`
	User                         *User      `json:"user"`
	NewChannel                   Channel    `json:"new_channel"`
	ServerOutputChannelIDAdmin   uint64     `json:"server_output_channel_id_admin"`
	ServerOutputChannelIDChat    uint64     `json:"server_output_channel_id_chat"`
	ServerOutputChannelIDKills   uint64     `json:"server_output_channel_id_kills"`
	ServerOutputChannelIDPlayers uint64     `json:"server_output_channel_id_players"`
}

// CacheKey func
func (cmr *SetOutputReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
