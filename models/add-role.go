package models

import "fmt"

// AddRoleReaction struct
type AddRoleReaction struct {
	RoleID         string     `json:"role_id"`
	Commands       []Command  `json:"commands"`
	Reactions      []Reaction `json:"reactions"`
	User           *User      `json:"user"`
	GuildServiceID uint64     `json:"guild_service_id"`
}

// CacheKey func
func (cmr *AddRoleReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
