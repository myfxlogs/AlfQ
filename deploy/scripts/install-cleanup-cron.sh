#!/usr/bin/env bash
# Install weekly docker-cleanup cron (Sunday 03:00).
# Run once per host: sudo bash deploy/scripts/install-cleanup-cron.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLEANUP="${SCRIPT_DIR}/docker-cleanup.sh"
CRON_LINE="0 3 * * 0 /bin/bash ${CLEANUP} >> /var/log/docker-cleanup.log 2>&1"

if [[ ! -x "$CLEANUP" ]]; then
    echo "ERROR: $CLEANUP not found or not executable" >&2
    exit 1
fi

# Avoid duplicate entries
if crontab -l 2>/dev/null | grep -qF "$CLEANUP"; then
    echo "cron entry already exists, skipping."
    exit 0
fi

(crontab -l 2>/dev/null; echo "$CRON_LINE") | crontab -
echo "cron installed: 每周日 03:00 执行 ${CLEANUP}"
