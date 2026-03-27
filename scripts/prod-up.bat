@echo off
pushd "%~dp0.."

if not exist "envs\prod\compose.env" (
    echo [prod] ERROR: envs\prod\compose.env not found.
    echo [prod]        Copy envs\prod\compose.env.example and fill in image names and URLs.
    popd & exit /b 1
)
if not exist "envs\prod\backend.env" (
    echo [prod] ERROR: envs\prod\backend.env not found.
    echo [prod]        Copy envs\prod\backend.env.example and fill in secrets.
    popd & exit /b 1
)
if not exist "envs\prod\frontend.env" (
    echo [prod] ERROR: envs\prod\frontend.env not found.
    echo [prod]        Copy envs\prod\frontend.env.example and fill in values.
    popd & exit /b 1
)

echo [prod] Pulling images and starting production environment...
docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env pull
docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env up -d
if %errorlevel% neq 0 ( echo [prod] ERROR: failed to start & popd & exit /b 1 )
echo [prod] Started.
popd
