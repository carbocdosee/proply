@echo off
pushd "%~dp0.."
echo [prod] Stopping production environment...
docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env down
popd
