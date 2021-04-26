package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/viewmodels"
	"go.uber.org/zap"
)

// Controller struct
type Controller struct {
	Config *configs.Config
	Cache  *cache.Cache
}

// Response sends a response to the client
func Response(ctx context.Context, w http.ResponseWriter, response interface{}, status int) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(response)

	if err != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", err))
		logger := logging.Logger(ctx)
		logger.Error("error_log")
	}
}

// Error sends error response to the client
func Error(ctx context.Context, w http.ResponseWriter, message string, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encErr := json.NewEncoder(w).Encode(viewmodels.ErrorResponse{
		Message: message,
		Error:   err.Error(),
	})

	if encErr != nil {
		encCtx := logging.AddValues(ctx, zap.NamedError("error", encErr))
		logger := logging.Logger(encCtx)
		logger.Error("error_log")
	}

	ctx = logging.AddValues(ctx,
		zap.NamedError("error", err),
		zap.String("error_message", message),
	)

	if status >= 500 {
		ctx = logging.AddValues(ctx, zap.String("trace", string(debug.Stack())))
	}

	logger := logging.Logger(ctx)
	logger.Error("error_log")
}
