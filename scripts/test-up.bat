@echo off
pushd "%~dp0.."
echo [test] Building and starting test environment...
docker compose -f docker-compose.test.yml --env-file envs/test/compose.env up --build -d
if %errorlevel% neq 0 ( echo [test] ERROR: failed to start & popd & exit /b 1 )
echo [test] Running at http://localhost:3000  (frontend)
echo [test]              http://localhost:8080  (backend)
popd
