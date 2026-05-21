#!/usr/bin/env bash
# ALFQ Docker cleanup — prunes stale build cache and unused images.
# Run weekly via cron: deploy/scripts/install-cleanup-cron.sh
set -euo pipefail

echo "[$(date '+%Y-%m-%d %H:%M:%S')] docker-cleanup starting"

# Keep last 20 GB of build cache, drop the rest
echo "→ pruning build cache (keep ≤ 20 GB)..."
docker builder prune --keep-storage 20GB -f

# Remove images unused for > 7 days
echo "→ pruning images unused for ≥ 7 days..."
docker image prune -af --filter "until=168h"

echo "→ current docker disk usage:"
docker system df

echo "[$(date '+%Y-%m-%d %H:%M:%S')] docker-cleanup finished"
