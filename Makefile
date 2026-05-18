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

go-build:
	cd backend/go && GOTOOLCHAIN=local go build ./cmd/admin-api

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
# Security
# ============================================================
sec-scan:
	govulncheck ./backend/go/... || true
