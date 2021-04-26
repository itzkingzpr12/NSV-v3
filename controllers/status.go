package controllers

import (
	"net/http"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/viewmodels"
	"go.uber.org/zap"
)

// GetStatus responds with the availability status of this service
func (c *Controller) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	status := viewmodels.GetStatusResponse{
		Message: "Service is available",
	}

	Response(ctx, w, status, http.StatusOK)
}
