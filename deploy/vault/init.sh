#!/bin/sh
# Vault initialization script for ALFQ.
# Run ONCE after first `docker compose up -d vault`.
# Stores unseal keys and root token in deploy/vault/init-output.json (ADD TO .gitignore).
#
# Usage:
#   chmod +x deploy/vault/init.sh
#   ./deploy/vault/init.sh

set -e

VAULT_ADDR="${VAULT_ADDR:-http://127.0.0.1:8200}"
OUTPUT_FILE="$(dirname "$0")/init-output.json"

echo "=== Initializing Vault at $VAULT_ADDR ==="

# Initialize with 1 key share, 1 threshold (single-host mode)
vault operator init -key-shares=1 -key-threshold=1 -format=json > "$OUTPUT_FILE"
echo "Init output saved to $OUTPUT_FILE"

UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' "$OUTPUT_FILE")
ROOT_TOKEN=$(jq -r '.root_token' "$OUTPUT_FILE")

echo "=== Unsealing Vault ==="
vault operator unseal "$UNSEAL_KEY"

echo "=== Logging in with root token ==="
vault login "$ROOT_TOKEN" > /dev/null

echo "=== Enabling kv-v2 secret engine ==="
vault secrets enable -path=secret kv-v2

echo "=== Writing application secrets ==="
# Load from .env and write to Vault.
# Each line format: KEY=VALUE
ENV_FILE="$(dirname "$0")/../.env"
if [ -f "$ENV_FILE" ]; then
  while IFS='=' read -r key value; do
    case "$key" in
      OPENAI_API_KEY|ANTHROPIC_API_KEY|NATS_PASSWORD|REDIS_PASSWORD|CLICKHOUSE_PASSWORD|DATABASE_URL)
        [ -n "$value" ] && vault kv put "secret/$key" value="$value"
        ;;
    esac
  done < "$ENV_FILE"
fi

echo ""
echo "=== Vault initialized and secrets loaded ==="
echo "Unseal key:  $UNSEAL_KEY"
echo "Root token:  $ROOT_TOKEN"
echo ""
echo "IMPORTANT: Save $OUTPUT_FILE securely. DO NOT commit to git."
echo "To unseal after restart: vault operator unseal <unseal_key>"
