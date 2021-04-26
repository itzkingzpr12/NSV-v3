package viewmodels

import "gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"

// CreateActivationTokenResponse struct
type CreateActivationTokenResponse struct {
	Message    string                 `json:"message"`
	SetupToken models.ActivationToken `json:"setup_token"`
}
