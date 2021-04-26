package routes

import (
	"errors"
	"net/http"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/controllers"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Authentication struct
type Authentication struct {
	ServiceToken string
	BasePath     string
}

// AuthenticationMiddleware verifies the Service-Token header is set and authorized for access to the API
func (m Authentication) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

		serviceTokenHeader := r.Header.Get("Service-Token")
		if r.URL.Path == m.BasePath+"/status" {
			next.ServeHTTP(w, r)
		} else if serviceTokenHeader == "" {
			controllers.Error(ctx, w, "A Service-Token header must be set for all routes", errors.New("Missing Service-Token header"), http.StatusUnauthorized)
		} else if serviceTokenHeader == m.ServiceToken {
			next.ServeHTTP(w, r)
		} else {
			controllers.Error(ctx, w, "An invalid Service-Token header was sent with request", errors.New("Invalid Service-Token header"), http.StatusUnauthorized)
		}
	})
}
