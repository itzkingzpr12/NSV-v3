package models

import "fmt"

type ServerOutputChannel struct {
	ID                          uint                          `json:"id"`
	ChannelID                   string                        `json:"channel_id"`
	OutputChannelTypeID         string                        `json:"output_channel_type_id"`
	ServerID                    uint                          `json:"server_id"`
	ServerOutputChannelSettings []*ServerOutputChannelSetting `json:"server_output_channel_settings"`
	Enabled                     bool                          `json:"enabled"`
}

type ServerOutputChannelSetting struct {
	ID                    uint   `json:"id"`
	ServerOutputChannelID uint   `json:"server_output_channel_id"`
	SettingName           string `json:"setting_name"`
	SettingValue          string `json:"setting_value"`
	Enabled               bool   `json:"enabled"`
}

// KillFeedServer struct
type KillFeedServer struct {
	Server        Server                `json:"server"`
	OutputChannel []ServerOutputChannel `json:"output_channel"`
}

// KillFeedSettingsReaction struct
type KillFeedSettingsReaction struct {
	Servers   []KillFeedServer `json:"servers"`
	Reactions []Reaction       `json:"reactions"`
	User      *User            `json:"user"`
}

// CacheKey func
func (cmr *KillFeedSettingsReaction) CacheKey(base, messageID string) string {
	return fmt.Sprintf("%s:%s", base, messageID)
}
