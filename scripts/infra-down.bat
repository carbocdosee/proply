@echo off
pushd "%~dp0.."
echo [infra] Stopping local infrastructure...
docker compose -f docker-compose.infra.yml down
popd
