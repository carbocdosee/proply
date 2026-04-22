@echo off
:: test-run.bat — spin up test stack, run all tests, tear down.
::
:: Usage:
::   scripts\test-run.bat            — run everything
::   scripts\test-run.bat --go-only  — backend unit tests only
::   scripts\test-run.bat --e2e-only — Playwright E2E only
::   scripts\test-run.bat --no-down  — keep containers after run (debug)
::
:: Exit code: 0 = all passed, 1 = failure

setlocal EnableDelayedExpansion

pushd "%~dp0.."

set COMPOSE=docker compose -f docker-compose.test.yml --env-file envs/test/compose.env
set BACKEND_URL=http://localhost:8080
set FRONTEND_URL=http://localhost:3000
set HEALTH_TIMEOUT=120

:: ── Parse flags ───────────────────────────────────────────────────────────────

set RUN_GO=1
set RUN_E2E=1
set KEEP_CONTAINERS=0

for %%A in (%*) do (
    if "%%A"=="--go-only"  set RUN_E2E=0
    if "%%A"=="--e2e-only" set RUN_GO=0
    if "%%A"=="--no-down"  set KEEP_CONTAINERS=1
)

:: ── 1. Start containers ───────────────────────────────────────────────────────

echo [test-run] Building and starting test stack...
%COMPOSE% up --build -d
if %errorlevel% neq 0 (
    echo [test-run] ERROR: docker compose up failed.
    goto :teardown_fail
)

:: ── 2. Wait for backend ───────────────────────────────────────────────────────

echo [test-run] Waiting for backend (%BACKEND_URL%)...
call :wait_http "%BACKEND_URL%/health" %HEALTH_TIMEOUT%
if %errorlevel% neq 0 (
    echo [test-run] ERROR: backend did not become healthy.
    %COMPOSE% logs --tail=40
    goto :teardown_fail
)

:: ── 3. Wait for frontend ──────────────────────────────────────────────────────

echo [test-run] Waiting for frontend (%FRONTEND_URL%)...
call :wait_http "%FRONTEND_URL%" %HEALTH_TIMEOUT%
if %errorlevel% neq 0 (
    echo [test-run] ERROR: frontend did not become healthy.
    %COMPOSE% logs --tail=40
    goto :teardown_fail
)

:: ── 4. Go tests ───────────────────────────────────────────────────────────────

set GO_RESULT=SKIP
if %RUN_GO%==1 (
    echo [test-run] Running Go tests...
    pushd backend
    set DATABASE_URL=postgres://proply:proply@localhost:5432/proply?sslmode=disable
    go test ./... -count=1 -timeout 60s
    if !errorlevel!==0 (
        set GO_RESULT=PASS
    ) else (
        set GO_RESULT=FAIL
    )
    popd
)

:: ── 5. Playwright E2E ─────────────────────────────────────────────────────────

set E2E_RESULT=SKIP
if %RUN_E2E%==1 (
    echo [test-run] Running Playwright E2E tests...
    pushd frontend
    set PLAYWRIGHT_BASE_URL=%FRONTEND_URL%
    set VITE_API_URL=%BACKEND_URL%
    npx playwright test --reporter=list
    if !errorlevel!==0 (
        set E2E_RESULT=PASS
    ) else (
        set E2E_RESULT=FAIL
    )
    popd
)

:: ── 6. Teardown ───────────────────────────────────────────────────────────────

if %KEEP_CONTAINERS%==1 (
    echo [test-run] Skipping teardown (--no-down). Stop manually: scripts\test-down.bat
) else (
    echo [test-run] Tearing down containers...
    %COMPOSE% down -v
)

:: ── 7. Summary ────────────────────────────────────────────────────────────────

echo.
echo ------------------------------------
if not "%GO_RESULT%"=="SKIP"  echo   Go tests:  %GO_RESULT%
if not "%E2E_RESULT%"=="SKIP" echo   E2E tests: %E2E_RESULT%
echo ------------------------------------
echo.

if "%GO_RESULT%"=="FAIL"  goto :fail
if "%E2E_RESULT%"=="FAIL" goto :fail

popd
exit /b 0

:teardown_fail
if %KEEP_CONTAINERS%==0 (
    echo [test-run] Tearing down containers after error...
    %COMPOSE% down -v
)
popd
exit /b 1

:fail
popd
exit /b 1

:: ── Subroutine: wait_http <url> <timeout_seconds> ────────────────────────────
:wait_http
set _URL=%~1
set /a _DEADLINE=%~2
set /a _ELAPSED=0
:wait_loop
    curl -sf --max-time 2 "%_URL%" >nul 2>&1
    if %errorlevel%==0 (
        echo [test-run] OK.
        exit /b 0
    )
    if %_ELAPSED% geq %_DEADLINE% (
        exit /b 1
    )
    timeout /t 2 /nobreak >nul
    set /a _ELAPSED+=2
    goto :wait_loop
