package configs

import (
	"context"
	"os"
	"time"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// Config struct that contians the structure of the config
type Config struct {
	Redis struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
		Pool int    `yaml:"pool"`
	} `yaml:"REDIS"`
	NitradoService struct {
		URL string `yaml:"url"`
	} `yaml:"NITRADO_SERVICE"`
	GuildConfigService struct {
		Host     string `yaml:"host"`
		BasePath string `yaml:"base_path"`
	} `yaml:"GUILD_CONFIG_SERVICE"`
	CacheSettings struct {
		ActivationToken                    CacheSetting `yaml:"activation_token"`
		CommandMessageReaction             CacheSetting `yaml:"command_message_reaction"`
		CommandMessageStep                 CacheSetting `yaml:"command_message_step"`
		NitradoTokenGuild                  CacheSetting `yaml:"nitrado_token_guild"`
		BanReaction                        CacheSetting `yaml:"ban_reaction"`
		UnbanReaction                      CacheSetting `yaml:"unban_reaction"`
		StopReaction                       CacheSetting `yaml:"stop_reaction"`
		RestartReaction                    CacheSetting `yaml:"restart_reaction"`
		WhitelistReaction                  CacheSetting `yaml:"whitelist_reaction"`
		UnwhitelistReaction                CacheSetting `yaml:"unwhitelist_reaction"`
		ClearWhitelistReaction             CacheSetting `yaml:"clear_whitelist_reaction"`
		CreateChannelsReaction             CacheSetting `yaml:"create_channels_reaction"`
		SetOutputReaction                  CacheSetting `yaml:"set_output_reaction"`
		AddRoleReaction                    CacheSetting `yaml:"add_role_reaction"`
		RemoveRoleReaction                 CacheSetting `yaml:"remove_role_reaction"`
		OnlinePlayersOutputChannelMessages CacheSetting `yaml:"online_players_output_channel_messages"`
		RefreshBansReaction                CacheSetting `yaml:"refresh_bans_reaction"`
		KillFeedSettingsReaction           CacheSetting `yaml:"kill_feed_reaction"`
	} `yaml:"CACHE_SETTINGS"`
	Bot struct {
		Prefix           string `yaml:"prefix"`
		OkColor          int    `yaml:"ok_color"`
		WarnColor        int    `yaml:"warn_color"`
		ErrorColor       int    `yaml:"error_color"`
		DocumentationURL string `yaml:"documentation_url"`
		GuildService     string `yaml:"guild_service"`
		WorkingThumbnail string `yaml:"working_thumbnail"`
		OkThumbnail      string `yaml:"ok_thumbnail"`
		WarnThumbnail    string `yaml:"warn_thumbnail"`
		ErrorThumbnail   string `yaml:"error_thumbnail"`
	} `yaml:"BOT"`
	Runners struct {
		Logs    Runner `yaml:"logs"`
		Players Runner `yaml:"players"`
	} `yaml:"RUNNERS"`
	Commands  []Command           `yaml:"COMMANDS"`
	Reactions map[string]Reaction `yaml:"REACTIONS"`
}

// CacheSetting struct
type CacheSetting struct {
	Base    string `yaml:"base"`
	TTL     string `yaml:"ttl"`
	Enabled bool   `yaml:"enabled"`
}

// Command struct
type Command struct {
	Name          string   `yaml:"name"`
	Long          string   `yaml:"long"`
	Short         string   `yaml:"short"`
	Description   string   `yaml:"description"`
	MinArgs       int      `yaml:"min_args"`
	MaxArgs       int      `yaml:"max_args"`
	Usage         []string `yaml:"usage"`
	Examples      []string `yaml:"examples"`
	Enabled       bool     `yaml:"enabled"`
	Workers       int      `yaml:"workers"`
	Category      string   `yaml:"category"`
	CategoryShort string   `yaml:"category_short"`
}

// Reaction struct
type Reaction struct {
	Icon      string `yaml:"icon"`
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Animated  bool   `yaml:"animated"`
	FullEmoji string `yaml:"full_emoji"`
}

// Runner struct
type Runner struct {
	Frequency time.Duration `yaml:"frequency"`
	Workers   int           `yaml:"workers"`
	Delay     time.Duration `yaml:"delay"`
	Enabled   bool          `yaml:"enabled"`
}

// GetConfig gets the config file and returns a Config struct
func GetConfig(ctx context.Context, env string) *Config {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	configFile := "./configs/conf-" + env + ".yml"
	f, err := os.Open(configFile)

	if err != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", err))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	defer f.Close()

	var config Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&config)

	if err != nil {
		ctx = logging.AddValues(ctx, zap.NamedError("error", err))
		logger := logging.Logger(ctx)
		logger.Fatal("error_log")
	}

	return &config
}
