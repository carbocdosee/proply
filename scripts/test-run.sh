#!/usr/bin/env bash
# test-run.sh — spin up test stack, run all tests, tear down.
#
# Usage:
#   bash scripts/test-run.sh            # run everything
#   bash scripts/test-run.sh --go-only  # backend unit tests only
#   bash scripts/test-run.sh --e2e-only # Playwright E2E only
#   bash scripts/test-run.sh --no-down  # keep containers after run (debug)
#
# Exit code:
#   0  all tests passed
#   1  one or more test suites failed or stack failed to start

set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."

COMPOSE="docker compose -f $ROOT/docker-compose.test.yml --env-file $ROOT/envs/test/compose.env"
BACKEND_DIR="$ROOT/backend"
FRONTEND_DIR="$ROOT/frontend"

BACKEND_URL="${VITE_API_URL:-http://localhost:8080}"
FRONTEND_URL="http://localhost:3000"

# Maximum seconds to wait for services to become healthy
HEALTH_TIMEOUT=120

# ── Flags ─────────────────────────────────────────────────────────────────────

RUN_GO=true
RUN_E2E=true
KEEP_CONTAINERS=false

for arg in "$@"; do
  case $arg in
    --go-only)   RUN_E2E=false ;;
    --e2e-only)  RUN_GO=false  ;;
    --no-down)   KEEP_CONTAINERS=true ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────

log()  { echo "[test-run] $*"; }
err()  { echo "[test-run] ERROR: $*" >&2; }
die()  { err "$*"; exit 1; }

# Wait until a URL returns HTTP 200, or timeout.
wait_http() {
  local url="$1" label="$2" deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  log "Waiting for $label ($url)…"
  until curl -sf --max-time 2 "$url/health" >/dev/null 2>&1; do
    [[ $(date +%s) -lt $deadline ]] || die "$label did not become healthy in ${HEALTH_TIMEOUT}s"
    sleep 2
  done
  log "$label is up."
}

wait_frontend() {
  local deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  log "Waiting for frontend ($FRONTEND_URL)…"
  until curl -sf --max-time 2 "$FRONTEND_URL" >/dev/null 2>&1; do
    [[ $(date +%s) -lt $deadline ]] || die "Frontend did not become healthy in ${HEALTH_TIMEOUT}s"
    sleep 2
  done
  log "Frontend is up."
}

# ── Teardown trap ─────────────────────────────────────────────────────────────

teardown() {
  if [[ "$KEEP_CONTAINERS" == "true" ]]; then
    log "Skipping teardown (--no-down). Stop manually with: make test-down"
    return
  fi
  log "Tearing down containers…"
  $COMPOSE down -v || true
  log "Done."
}
trap teardown EXIT

# ── 1. Start containers ───────────────────────────────────────────────────────

log "Building and starting test stack…"
$COMPOSE up --build -d

# Wait for compose health checks on postgres/minio first (up to HEALTH_TIMEOUT).
log "Waiting for compose health checks (postgres, minio)…"
deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
until [[ $($COMPOSE ps --format json 2>/dev/null | \
          grep -c '"Health":"healthy"' || echo 0) -ge 2 ]]; do
  [[ $(date +%s) -lt $deadline ]] || {
    err "Compose healthchecks did not pass in ${HEALTH_TIMEOUT}s"
    $COMPOSE logs --tail=40
    exit 1
  }
  sleep 3
done

wait_http    "$BACKEND_URL"  "backend"
wait_frontend

# ── 2. Run backend unit/integration tests ─────────────────────────────────────

GO_EXIT=0
if [[ "$RUN_GO" == "true" ]]; then
  log "Running Go tests…"
  (
    cd "$BACKEND_DIR"
    DATABASE_URL="postgres://proply:proply@localhost:5432/proply?sslmode=disable" \
      go test ./... -count=1 -timeout 60s
  ) && log "Go tests passed." || { err "Go tests FAILED."; GO_EXIT=1; }
fi

# ── 3. Run Playwright E2E tests ───────────────────────────────────────────────

E2E_EXIT=0
if [[ "$RUN_E2E" == "true" ]]; then
  log "Running Playwright E2E tests…"
  (
    cd "$FRONTEND_DIR"
    PLAYWRIGHT_BASE_URL="$FRONTEND_URL" \
    VITE_API_URL="$BACKEND_URL" \
      npx playwright test --reporter=list
  ) && log "E2E tests passed." || { err "E2E tests FAILED."; E2E_EXIT=1; }
fi

# ── 4. Summary ────────────────────────────────────────────────────────────────

echo ""
echo "────────────────────────────────────"
[[ "$RUN_GO"  == "true" ]] && echo "  Go tests:  $([ $GO_EXIT  -eq 0 ] && echo PASS || echo FAIL)"
[[ "$RUN_E2E" == "true" ]] && echo "  E2E tests: $([ $E2E_EXIT -eq 0 ] && echo PASS || echo FAIL)"
echo "────────────────────────────────────"
echo ""

# Exit non-zero if any suite failed (teardown still runs via trap).
[[ $GO_EXIT -eq 0 && $E2E_EXIT -eq 0 ]] || exit 1
