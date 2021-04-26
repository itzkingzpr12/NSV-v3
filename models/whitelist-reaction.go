package models

import "fmt"

// WhitelistReaction struct
type WhitelistReaction struct {
	PlayerName string     `json:"player_name"`
	Servers    []Server   `json:"servers"`
	Reactions  []Reaction `json:"reactions"`
	User       *User      `json:"user"`
}

// CacheKey func
func (cmr *WhitelistReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
