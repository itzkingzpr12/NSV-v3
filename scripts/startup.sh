export ENVIRONMENT=$(echo ${ENV_VARS} | jq -r '.ENVIRONMENT')
export DISCORD_TOKEN=$(echo ${ENV_VARS} | jq -r '.DISCORD_TOKEN')
export SERVICE_TOKEN=$(echo ${ENV_VARS} | jq -r '.SERVICE_TOKEN')
export NITRADO_SERVICE_TOKEN=$(echo ${ENV_VARS} | jq -r '.NITRADO_SERVICE_TOKEN')
export GUILD_CONFIG_SERVICE_TOKEN=$(echo ${ENV_VARS} | jq -r '.GUILD_CONFIG_SERVICE_TOKEN')
export BASE_PATH=$(echo ${ENV_VARS} | jq -r '.BASE_PATH')

/main