BACKEND_DIR  := ./backend
FRONTEND_DIR := ./frontend

.PHONY: local local-back local-front \
        infra-up infra-down \
        test-up test-down test-logs test-ps \
        prod-up prod-down \
        install tidy

# ── Local (native, no Docker) ─────────────────────────────────────────────────
# Start infra first: make infra-up
# Then run services natively.

local:
	$(MAKE) -j2 local-back local-front

local-back:
	cd $(BACKEND_DIR) && go run ./cmd/app/main.go

local-front:
	cd $(FRONTEND_DIR) && npm run dev

# ── Local infra (postgres + migrations only) ──────────────────────────────────
# DATABASE_URL=postgres://proply:proply@localhost:5432/proply?sslmode=disable

infra-up:
	docker compose -f docker-compose.infra.yml up -d

infra-down:
	docker compose -f docker-compose.infra.yml down

# ── Test (full stack in Docker) ───────────────────────────────────────────────

test-up:
	docker compose -f docker-compose.test.yml --env-file envs/test/compose.env up --build -d

test-down:
	docker compose -f docker-compose.test.yml --env-file envs/test/compose.env down -v

test-logs:
	docker compose -f docker-compose.test.yml --env-file envs/test/compose.env logs -f

test-ps:
	docker compose -f docker-compose.test.yml --env-file envs/test/compose.env ps

# ── Production ────────────────────────────────────────────────────────────────
# Requires: envs/prod/compose.env, envs/prod/backend.env, envs/prod/frontend.env

prod-up:
	docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env pull
	docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env up -d

prod-down:
	docker compose -f docker-compose.prod.yml --env-file envs/prod/compose.env down

# ── Utils ─────────────────────────────────────────────────────────────────────

install:
	cd $(FRONTEND_DIR) && npm install

tidy:
	cd $(BACKEND_DIR) && go mod tidy
