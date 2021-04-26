package models

import (
	"fmt"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
)

// RemoveRoleReaction struct
type RemoveRoleReaction struct {
	RoleID                  string                              `json:"role_id"`
	Reactions               []Reaction                          `json:"reactions"`
	User                    *User                               `json:"user"`
	GuildServicePermissions []gcscmodels.GuildServicePermission `json:"guild_service_permissions"`
}

// CacheKey func
func (cmr *RemoveRoleReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
