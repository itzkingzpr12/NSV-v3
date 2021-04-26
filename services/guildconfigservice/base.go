package guildconfigservice

import (
	"context"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// GuildConfigService struct
type GuildConfigService struct {
	Client *gcsc.GuildConfigServiceClient
	Auth   runtime.ClientAuthInfoWriter
}

// Error struct
type Error struct {
	Message string `json:"message"`
	Err     error  `json:"error"`
}

// Error func
func (e *Error) Error() string {
	return e.Err.Error()
}

// InitService initializes the guild config service client
func InitService(ctx context.Context, config *configs.Config, serviceToken string) *GuildConfigService {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	transport := httptransport.New(config.GuildConfigService.Host, config.GuildConfigService.BasePath, gcsc.DefaultSchemes)
	client := gcsc.New(transport, strfmt.Default)
	apiKeyAuth := httptransport.APIKeyAuth("Service-Token", "header", serviceToken)
	// client.Guilds.DeleteGuild(&guilds.DeleteGuildParams{}, apiKeyAuth)
	// client.Guilds.CreateGuild(&guilds.CreateGuildParams{
	// 	Body: &models.Guild{},
	// }, apiKeyAuth)
	return &GuildConfigService{
		Client: client,
		Auth:   apiKeyAuth,
	}
}
