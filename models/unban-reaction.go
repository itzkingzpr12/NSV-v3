package models

import "fmt"

// UnbanReaction struct
type UnbanReaction struct {
	PlayerName string     `json:"player_name"`
	Servers    []Server   `json:"servers"`
	Reactions  []Reaction `json:"reactions"`
	User       *User      `json:"user"`
}

// CacheKey func
func (cmr *UnbanReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
