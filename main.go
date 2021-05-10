package main

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/caarlos0/env"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/controllers"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/interactions"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/routes"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/runners"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/guildconfigservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/services/nitradoservice"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/cache"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Environment struct
type Environment struct {
	Environment             string `env:"ENVIRONMENT,required"`
	DiscordToken            string `env:"DISCORD_TOKEN,required"`
	ListenerPort            string `env:"LISTENER_PORT,required"`
	ServiceToken            string `env:"SERVICE_TOKEN,required"`
	NitradoServiceToken     string `env:"NITRADO_SERVICE_TOKEN,required"`
	GuildConfigServiceToken string `env:"GUILD_CONFIG_SERVICE_TOKEN,required"`
	BasePath                string `env:"BASE_PATH"`
	Migrate                 bool   `env:"MIGRATE"`
}

func main() {
	ctx := context.Background()
	environment := Environment{}
	if err := env.Parse(&environment); err != nil {
		log.Fatal("FAILED TO LOAD CONFIG")
	}

	ctx = logging.AddValues(ctx,
		zap.String("scope", logging.GetFuncName()),
		zap.String("env", environment.Environment),
		zap.String("listener_port", environment.ListenerPort),
		zap.String("base_path", environment.BasePath),
	)

	config := configs.GetConfig(ctx, environment.Environment)
	cache := InitCache(ctx, config)
	guildConfigService := guildconfigservice.InitService(ctx, config, environment.GuildConfigServiceToken)
	nitradoService := nitradoservice.InitService(ctx, config, environment.NitradoServiceToken)

	// Instantiate Discord client
	dg, discErr := discordgo.New("Bot " + environment.DiscordToken)
	if discErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", discErr), zap.String("error_message", "Failed to create Discord client"))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	defer dg.Close()

	// Open a websocket connection to Discord and begin listening.
	openErr := dg.Open()
	if openErr != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", openErr), zap.String("error_message", "Failed to open Discord web socket"))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	comm := interactions.Interactions{
		Session:            dg,
		Config:             config,
		Cache:              cache,
		GuildConfigService: guildConfigService,
		NitradoService:     nitradoService,
	}

	comm.SetupHandlers()

	run := runners.Runners{
		Session:            dg,
		Config:             config,
		Cache:              cache,
		GuildConfigService: guildConfigService,
		NitradoService:     nitradoService,
	}

	run.StartRunners()

	router := routes.GetRouter(ctx)
	controller := controllers.Controller{
		Config:             config,
		Cache:              cache,
		DiscordSession:     dg,
		GuildConfigService: guildConfigService,
	}

	r := routes.Router{
		ServiceToken: environment.ServiceToken,
		Port:         environment.ListenerPort,
		BasePath:     environment.BasePath,
		Controller:   &controller,
	}

	routes.AddRoutes(ctx, router, r)

}

// InitCache initializes the Redis cache
func InitCache(ctx context.Context, config *configs.Config) *cache.Cache {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))
	pool, err := cache.GetClient(ctx, config.Redis.Host, config.Redis.Port, config.Redis.Pool)

	if err != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", err), zap.String("error_message", err.Message))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	return &cache.Cache{
		Client: pool,
	}
}
