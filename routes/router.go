package routes

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/controllers"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Router struct
type Router struct {
	Controller   *controllers.Controller
	ServiceToken string
	Port         string
	BasePath     string
}

// GetRouter creates and returns a router
func GetRouter(ctx context.Context) *mux.Router {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))
	return mux.NewRouter().StrictSlash(true)
}

// AddRoutes adds all necessary routes to the router
func AddRoutes(ctx context.Context, router *mux.Router, r Router) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))
	auth := Authentication{
		ServiceToken: r.ServiceToken,
		BasePath:     r.BasePath,
	}

	// STATUS
	router.HandleFunc(r.BasePath+"/status", r.Controller.GetStatus).Methods("GET")
	router.HandleFunc(r.BasePath+"/activation-tokens", r.Controller.CreateActivationToken).Methods("POST")
	router.HandleFunc(r.BasePath+"/discord/all-guilds", r.Controller.GetAllGuilds).Methods("GET")
	router.HandleFunc(r.BasePath+"/discord/verify-subscriber-guilds", r.Controller.VerifySubscriberGuilds).Methods("GET")

	router.Use(auth.AuthenticationMiddleware)

	loggingMiddleware := LoggingMiddleware()
	loggedRouter := loggingMiddleware(router)

	logger := logging.Logger(ctx)
	logger.Info("Starting Listener", zap.String("port", r.Port))
	if err := http.ListenAndServe(":"+r.Port, loggedRouter); err != nil {
		logger.Fatal("error_log", zap.NamedError("err", err))
	}
}
