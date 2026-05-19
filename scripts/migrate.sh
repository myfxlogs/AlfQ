#!/bin/bash
# Database migration using goose
set -e
DB_DSN="${PG_DSN:-postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable}"
goose -dir backend/go/migrations postgres "$DB_DSN" up
echo "Migrations applied successfully"
