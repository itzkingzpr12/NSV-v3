package nitradoservice

import (
	"context"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	nsv2 "gitlab.com/BIC_Dev/nitrado-service-v2-client"
	"go.uber.org/zap"
)

// NitradoService struct
type NitradoService struct {
	Client *nsv2.Client
}

// InitService func
func InitService(ctx context.Context, config *configs.Config, serviceToken string) *NitradoService {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	client, cErr := nsv2.NewClient(nsv2.ClientConfig{
		BasePath:     config.NitradoService.URL,
		ServiceToken: serviceToken,
	})

	if cErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", cErr), zap.String("error_message", "Failed to instantiate Nitrado Service V2 client"))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	return &NitradoService{
		Client: client,
	}
}
