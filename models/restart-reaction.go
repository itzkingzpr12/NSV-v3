package models

import "fmt"

// RestartReaction struct
type RestartReaction struct {
	Servers   []Server   `json:"servers"`
	Reactions []Reaction `json:"reactions"`
	User      *User      `json:"user"`
	Message   string     `json:"message"`
}

// CacheKey func
func (cmr *RestartReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
