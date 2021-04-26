package models

import "fmt"

// BanReaction struct
type BanReaction struct {
	PlayerName string     `json:"player_name"`
	Servers    []Server   `json:"servers"`
	Reactions  []Reaction `json:"reactions"`
	User       *User      `json:"user"`
}

// CacheKey func
func (cmr *BanReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
