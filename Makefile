.PHONY: proto proto-lint proto-gen proto-mtapi-gen \
        go-lint go-test go-build go-test-integration \
        py-lint py-test \
        web-lint web-build \
        build test lint \
        dev-up dev-down dev-logs \
        sec-scan

# ============================================================
# Proto
# ============================================================
proto-lint:
	cd backend/proto && buf lint

proto-gen:
	cd backend/proto && buf generate

proto: proto-lint proto-gen

# 生成 mtapi MT4/MT5 Go gRPC 桩代码（不修改 /opt/alfq/gprc/ 官方 proto）
proto-mtapi-gen:
	bash backend/proto-mtapi/build.sh

# ============================================================
# Go
# ============================================================
go-lint:
	cd backend/go && GOTOOLCHAIN=local golangci-lint run ./...

go-test:
	cd backend/go && GOTOOLCHAIN=local go test -race -count=1 ./...

SVC ?= trading-core
CGO_ENABLED ?= 0
go-build: proto-gen
	docker build --build-arg SVC=$(SVC) --build-arg CGO_ENABLED=$(CGO_ENABLED) -f backend/go/Dockerfile.builder .

go-test-integration:
	cd backend/go && GOTOOLCHAIN=local go test -tags=integration -timeout=10m ./test/integration/...

# ============================================================
# Python (Research)
# ============================================================
py-lint:
	cd research && uv run --extra dev ruff check .

py-test:
	cd research && uv run --extra dev pytest

# ============================================================
# Frontend
# ============================================================
web-lint:
	cd frontend && pnpm lint

web-build:
	cd frontend && pnpm build

# ============================================================
# Aggregate
# ============================================================
build: go-build web-build

test: go-test py-test

lint: go-lint py-lint web-lint proto-lint

# ============================================================
# Dev Infrastructure
# ============================================================
dev-up:
	docker compose -f deploy/docker-compose.yml up -d --wait

dev-down:
	docker compose -f deploy/docker-compose.yml down -v

dev-logs:
	docker compose -f deploy/docker-compose.yml logs -f

# ============================================================
# Database
# ============================================================
# Requires PGPASSWORD env var (or ~/.pgpass). Never hardcode credentials.
db-migrate:
	@test -n "$$PGPASSWORD" || (echo "ERROR: PGPASSWORD env var required" && exit 1)
	@echo "Running migrations..."
	psql -h $${PGHOST:-localhost} -U $${PGUSER:-alfq} -d $${PGDATABASE:-alfq} -f backend/go/migrations/001_initial_schema.sql
	psql -h $${PGHOST:-localhost} -U $${PGUSER:-alfq} -d $${PGDATABASE:-alfq} -f backend/go/migrations/002_operational_tables.sql
	@echo "Migrations complete."

db-reset:
	@test -n "$$PGPASSWORD" || (echo "ERROR: PGPASSWORD env var required" && exit 1)
	@echo "Resetting database..."
	psql -h $${PGHOST:-localhost} -U $${PGUSER:-alfq} -d postgres -c "DROP DATABASE IF EXISTS alfq"
	psql -h $${PGHOST:-localhost} -U $${PGUSER:-alfq} -d postgres -c "CREATE DATABASE alfq"
	$(MAKE) db-migrate

# ============================================================
# Security
# ============================================================
sec-scan:
	govulncheck ./backend/go/...
