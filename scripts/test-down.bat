@echo off
pushd "%~dp0.."
echo [test] Stopping test environment (volumes will be removed)...
docker compose -f docker-compose.test.yml --env-file envs/test/compose.env down -v
popd
