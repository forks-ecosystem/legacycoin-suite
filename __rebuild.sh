#!/bin/sh
# LegacyCoin Suite — rebuild script
# Rebuilds the container image from source and restarts the suite
# (miner + admin panel). Miner settings live in config/miner.json and are
# volume-mounted, so they survive a rebuild.

set -e

cd "$(dirname "$0")"

# Load .env
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

echo "=== [1/3] Building image ==="
docker compose build

echo "=== [2/3] Recreating container ==="
docker compose up -d

echo "=== [3/3] Status ==="
sleep 2

STATUS=$(docker inspect --format='{{.State.Status}}' legacycoin-suite 2>/dev/null || echo "not found")

if [ "$STATUS" = "running" ]; then
    STATUS_ICON="✓"
    STATUS_TEXT="RUNNING"
else
    STATUS_ICON="✗"
    STATUS_TEXT="$STATUS"
fi

echo
echo "════════════════════════════════════════"
echo " LegacyCoin Suite"
echo "────────────────────────────────────────"
echo " $STATUS_ICON Container: $STATUS_TEXT"
echo " $STATUS_ICON HTTP port: 3002"
echo " $STATUS_ICON Web: ${SUITE_URL:-http://localhost:3002}"
echo "════════════════════════════════════════"
