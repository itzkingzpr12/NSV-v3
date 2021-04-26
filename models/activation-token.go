package models

import "fmt"

// ActivationToken struct
type ActivationToken struct {
	Token string `json:"token"`
}

// CacheKey func
func (cms *ActivationToken) CacheKey(base, token string) string {
	return fmt.Sprintf("%s:%s", base, token)
}
