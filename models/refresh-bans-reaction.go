package models

import "fmt"

// RefreshBansReaction struct
type RefreshBansReaction struct {
	ServerBans map[uint64][]string `json:"server_bans"`
	Reactions  []Reaction          `json:"reactions"`
	User       *User               `json:"user"`
}

// CacheKey func
func (cmr *RefreshBansReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
