#!/bin/bash
# Database migration using goose
set -e
DB_DSN="${PG_DSN:?PG_DSN env required (e.g. postgres://user:pass@host:5432/alfq?sslmode=disable)}"
goose -dir backend/go/migrations postgres "$DB_DSN" up
echo "Migrations applied successfully"
