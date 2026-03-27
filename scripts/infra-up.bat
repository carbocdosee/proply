@echo off
pushd "%~dp0.."
echo [infra] Starting local infrastructure (postgres + migrations)...
docker compose -f docker-compose.infra.yml up -d
if %errorlevel% neq 0 ( echo [infra] ERROR: failed to start & popd & exit /b 1 )
echo [infra] Ready. Set DATABASE_URL=postgres://proply:proply@localhost:5432/proply?sslmode=disable in backend/.env
popd
