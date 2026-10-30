#!/bin/sh
# LegacyCoin Suite — rebuild script
# Rebuilds the container image from source and restarts the suite
# (miner + admin panel). Miner settings live in config/miner.json and are
# volume-mounted, so they survive a rebuild.

set -e

cd "$(dirname "$0")"
COMPOSE="docker compose"
COMPOSE_FILE="docker-compose.yml"
IMAGE_NAME="legacycoin-suite:latest"

echo "=== [1/4] Stopping and removing containers ==="
$COMPOSE -f "$COMPOSE_FILE" down || true
docker rm -f legacycoin-suite > /dev/null 2>&1 || true

echo "=== [2/4] Removing old image ==="
docker rmi -f "$IMAGE_NAME" > /dev/null 2>&1 || echo "Image $IMAGE_NAME not found or already removed"
docker image prune -f > /dev/null 2>&1 || true

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
echo " $STATUS_ICON Web: http://localhost:3002"
echo "════════════════════════════════════════"
