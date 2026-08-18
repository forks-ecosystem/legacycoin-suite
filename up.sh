#!/bin/sh

# Load .env
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

docker compose up -d || exit 1

# Wait for container
sleep 2

# Check status
STATUS=$(docker inspect --format='{{.State.Status}}' legacycoin-suite 2>/dev/null || echo "not found")

if [ "$STATUS" = "running" ]; then
    STATUS_ICON="✓"
    STATUS_TEXT="RUNNING"
else
    STATUS_ICON="✗"
    STATUS_TEXT="$STATUS"
fi

# Get port
PORT=$(docker port legacycoin-suite 3002/tcp 2>/dev/null | head -1)
if [ -z "$PORT" ]; then
    PORT="3002 (host network)"
fi

echo
echo "════════════════════════════════════════"
echo " LegacyCoin Suite"
echo "────────────────────────────────────────"
echo " $STATUS_ICON Container: $STATUS_TEXT"
echo " $STATUS_ICON HTTP port: 3002"
echo " $STATUS_ICON Web: ${SUITE_URL:-http://localhost:3002}"
echo "════════════════════════════════════════"
