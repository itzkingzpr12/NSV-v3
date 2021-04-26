docker build -t nitrado-server-manager-v3 .
docker run -e "ENV_VARS=$(<./scripts/env_vars.json)" -e "LISTENER_PORT=8081" -p 8081:8081 nitrado-server-manager-v3
