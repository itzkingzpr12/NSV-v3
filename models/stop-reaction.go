package models

import "fmt"

// StopReaction struct
type StopReaction struct {
	Servers   []Server   `json:"servers"`
	Reactions []Reaction `json:"reactions"`
	User      *User      `json:"user"`
}

// CacheKey func
func (cmr *StopReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
