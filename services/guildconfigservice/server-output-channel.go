package guildconfigservice

import (
	"context"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/server_output_channels"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
)

// CreateServerOutputChannel func
func CreateServerOutputChannel(ctx context.Context, gcs *GuildConfigService, guildID string, channelID string, serverID uint64, outputType string) (*gcscmodels.CreateServerOutputChannelResponse, *Error) {
	body := gcscmodels.ServerOutputChannel{
		ChannelID:           channelID,
		Enabled:             true,
		ServerID:            serverID,
		OutputChannelTypeID: outputType,
	}

	outputChannelParams := server_output_channels.NewCreateServerOutputChannelParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetContext(context.Background())
	outputChannelParams.SetBody(&body)

	outputChannel, ocErr := gcs.Client.ServerOutputChannels.CreateServerOutputChannel(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to create server output channel",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// GetServerOutputChannel func
func GetServerOutputChannel(ctx context.Context, gcs *GuildConfigService, guildID string, outputChannelID uint64) (*gcscmodels.GetServerOutputChannelByIDResponse, *Error) {
	outputChannelParams := server_output_channels.NewGetServerOutputChannelByIDParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetServerOutputChannelID(int64(outputChannelID))
	outputChannelParams.SetContext(context.Background())

	outputChannel, ocErr := gcs.Client.ServerOutputChannels.GetServerOutputChannelByID(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to get server output channel",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// UpdateServerOutputChannel func
func UpdateServerOutputChannel(ctx context.Context, gcs *GuildConfigService, guildID string, outputChannelID uint64, channelID string, serverID uint64, outputType string) (*gcscmodels.UpdateServerOutputChannelResponse, *Error) {
	body := gcscmodels.UpdateServerOutputChannelRequest{
		ChannelID:           channelID,
		Enabled:             true,
		ServerID:            serverID,
		OutputChannelTypeID: outputType,
	}

	outputChannelParams := server_output_channels.NewUpdateServerOutputChannelParamsWithTimeout(30)
	outputChannelParams.SetServerOutputChannelID(int64(outputChannelID))
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetContext(context.Background())
	outputChannelParams.SetBody(&body)

	outputChannel, ocErr := gcs.Client.ServerOutputChannels.UpdateServerOutputChannel(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to update server output channel",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// DeleteServerOutputChannel func
func DeleteServerOutputChannel(ctx context.Context, gcs *GuildConfigService, guildID string, outputChannelID int64) (*gcscmodels.DeleteServerOutputChannelResponse, *Error) {
	outputChannelParams := server_output_channels.NewDeleteServerOutputChannelParamsWithTimeout(30)
	outputChannelParams.SetGuild(guildID)
	outputChannelParams.SetServerOutputChannelID(outputChannelID)
	outputChannelParams.SetContext(context.Background())

	outputChannel, ocErr := gcs.Client.ServerOutputChannels.DeleteServerOutputChannel(outputChannelParams, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to delete server output channel",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}
