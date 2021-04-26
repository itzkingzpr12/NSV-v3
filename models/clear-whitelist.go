package models

import "fmt"

// ClearWhitelistReaction struct
type ClearWhitelistReaction struct {
	Servers   []Server   `json:"servers"`
	Reactions []Reaction `json:"reactions"`
	User      *User      `json:"user"`
}

// CacheKey func
func (cmr *ClearWhitelistReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
