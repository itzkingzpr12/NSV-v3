package guildconfigservice

import (
	"context"

	"gitlab.com/BIC_Dev/guild-config-service-client/gcsc/guild_service_permissions"
	"gitlab.com/BIC_Dev/guild-config-service-client/gcscmodels"
)

// CreateGuildServicePermission func
func CreateGuildServicePermission(ctx context.Context, gcs *GuildConfigService, guildID string, guildServiceID uint64, roleID string, commandName string) (*gcscmodels.CreateGuildServicePermissionResponse, *Error) {
	body := gcscmodels.GuildServicePermission{
		GuildServiceID: guildServiceID,
		RoleID:         roleID,
		CommandName:    commandName,
		Enabled:        true,
	}

	params := guild_service_permissions.NewCreateGuildServicePermissionParamsWithTimeout(30)
	params.SetGuild(guildID)
	params.SetContext(context.Background())
	params.SetBody(&body)

	outputChannel, ocErr := gcs.Client.GuildServicePermissions.CreateGuildServicePermission(params, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to create guild service permissions",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}

// DeleteGuildServicePermission func
func DeleteGuildServicePermission(ctx context.Context, gcs *GuildConfigService, guildID string, guildServicePermissionsID int64) (*gcscmodels.DeleteGuildServicePermissionResponse, *Error) {
	params := guild_service_permissions.NewDeleteGuildServicePermissionParamsWithTimeout(30)
	params.SetGuildServicePermissionID(guildServicePermissionsID)
	params.SetGuild(guildID)
	params.SetContext(context.Background())

	outputChannel, ocErr := gcs.Client.GuildServicePermissions.DeleteGuildServicePermission(params, gcs.Auth)
	if ocErr != nil {
		return nil, &Error{
			Message: "Failed to delete guild service permissions",
			Err:     ocErr,
		}
	}

	return outputChannel.Payload, nil
}
