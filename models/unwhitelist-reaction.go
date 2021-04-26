package models

import "fmt"

// UnwhitelistReaction struct
type UnwhitelistReaction struct {
	PlayerName string     `json:"player_name"`
	Servers    []Server   `json:"servers"`
	Reactions  []Reaction `json:"reactions"`
	User       *User      `json:"user"`
}

// CacheKey func
func (cmr *UnwhitelistReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
