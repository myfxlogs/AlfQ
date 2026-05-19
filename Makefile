.PHONY: proto proto-lint proto-gen proto-mtapi-gen \
        go-lint go-test go-build \
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

go-build: proto-gen
	cd backend/go && GOTOOLCHAIN=local go build ./cmd/trading-core ./cmd/md-gateway ./cmd/quant-engine ./cmd/assistant-svc

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
db-migrate:
	@echo "Running migrations..."
	PGPASSWORD=alfq_dev psql -h localhost -U alfq -d alfq -f backend/go/migrations/001_initial_schema.sql
	PGPASSWORD=alfq_dev psql -h localhost -U alfq -d alfq -f backend/go/migrations/002_operational_tables.sql
	@echo "Migrations complete."

db-reset:
	@echo "Resetting database..."
	PGPASSWORD=alfq_dev psql -h localhost -U alfq -d postgres -c "DROP DATABASE IF EXISTS alfq"
	PGPASSWORD=alfq_dev psql -h localhost -U alfq -d postgres -c "CREATE DATABASE alfq"
	$(MAKE) db-migrate

# ============================================================
# Security
# ============================================================
sec-scan:
	govulncheck ./backend/go/... || true
