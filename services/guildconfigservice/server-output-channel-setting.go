package guildconfigservice

import (
	"context"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/server_output_channel_settings"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
)

// CreateServerOutputChannelSetting func
func CreateServerOutputChannelSetting(ctx context.Context, gcs *GuildConfigService, guildID string, serverOutputChannelID uint64, settingName string, settingValue string) (*gcscmodels.CreateServerOutputChannelSettingResponse, *Error) {
	body := gcscmodels.ServerOutputChannelSetting{
		ServerOutputChannelID: serverOutputChannelID,
		SettingName:           settingName,
		SettingValue:          settingValue,
		Enabled:               true,
	}

	outputChannelParams := server_output_channel_settings.NewCreateServerOutputChannelSettingParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetContext(context.Background())
	outputChannelParams.SetBody(&body)

	outputChannel, ocErr := gcs.Client.ServerOutputChannelSettings.CreateServerOutputChannelSetting(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to create server output channel setting",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// GetServerOutputChannelSetting func
func GetServerOutputChannelSetting(ctx context.Context, gcs *GuildConfigService, guildID string, serverOutputChannelSettingID uint64) (*gcscmodels.GetServerOutputChannelSettingByIDResponse, *Error) {
	outputChannelParams := server_output_channel_settings.NewGetServerOutputChannelSettingByIDParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetServerOutputChannelSettingID(int64(serverOutputChannelSettingID))
	outputChannelParams.SetContext(context.Background())

	outputChannel, ocErr := gcs.Client.ServerOutputChannelSettings.GetServerOutputChannelSettingByID(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to get server output channel setting",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// UpdateServerOutputChannelSetting func
func UpdateServerOutputChannelSetting(ctx context.Context, gcs *GuildConfigService, guildID string, serverOutputChannelSettingID uint64, serverOutputChannelID uint64, settingName string, settingValue string) (*gcscmodels.UpdateServerOutputChannelSettingResponse, *Error) {
	body := gcscmodels.UpdateServerOutputChannelSettingRequest{
		ServerOutputChannelID: serverOutputChannelID,
		SettingName:           settingName,
		SettingValue:          settingValue,
		Enabled:               true,
	}

	outputChannelParams := server_output_channel_settings.NewUpdateServerOutputChannelSettingParamsWithTimeout(30)
	outputChannelParams.SetServerOutputChannelSettingID(int64(serverOutputChannelSettingID))
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetContext(context.Background())
	outputChannelParams.SetBody(&body)

	outputChannel, ocErr := gcs.Client.ServerOutputChannelSettings.UpdateServerOutputChannelSetting(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to update server output channel setting",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// DeleteServerOutputChannelSetting func
func DeleteServerOutputChannelSetting(ctx context.Context, gcs *GuildConfigService, guildID string, outputChannelSettingID int64) (*gcscmodels.DeleteServerOutputChannelSettingResponse, *Error) {
	outputChannelParams := server_output_channel_settings.NewDeleteServerOutputChannelSettingParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetServerOutputChannelSettingID(outputChannelSettingID)
	outputChannelParams.SetContext(context.Background())

	outputChannel, ocErr := gcs.Client.ServerOutputChannelSettings.DeleteServerOutputChannelSetting(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to delete server output channel setting",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}
