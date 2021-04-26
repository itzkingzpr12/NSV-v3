package models

import "fmt"

// NitradoTokenGuild struct
type NitradoTokenGuild struct {
	Guild Guild `json:"guild"`
	User  User  `json:"user"`
}

// CacheKey func
func (cms *NitradoTokenGuild) CacheKey(base, userID string) string {
	return fmt.Sprintf("%s:%s", base, userID)
}
