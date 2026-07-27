#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PM2_NAME="api-server"
MINER_NAME="legacycoin-miner"
MINER_WALLET="Li8Q3GonH6iocGzfZLGxbpjazqnmez8zrJ.20"
MINER_POOL="stratum+tcp://forkex.net:3032"
MINER_WORKERS=4

echo -e "${YELLOW}Building api-server...${NC}"
pnpm --filter workspace/api-server run build

echo -e "${YELLOW}Starting with PM2...${NC}"

# Проверка и запуск api-server
if pm2 list 2>/dev/null | grep -q "$PM2_NAME"; then
    echo "  Restarting $PM2_NAME..."
    pm2 restart "$PM2_NAME"
else
    echo "  Starting $PM2_NAME..."
    pm2 start ecosystem.config.js
fi

# Запуск майнера через PM2
if pm2 list 2>/dev/null | grep -q "$MINER_NAME"; then
    echo "  Restarting $MINER_NAME..."
    pm2 restart "$MINER_NAME"
else
    echo "  Starting $MINER_NAME..."
    pm2 start ./legacycoin-miner \
        --name "$MINER_NAME" \
        -- -wallet="$MINER_WALLET" \
           -pool="$MINER_POOL" \
           -workers="$MINER_WORKERS"
fi

pm2 save

echo -e "${GREEN} Miner and API server started${NC}"
echo ""
echo " Commands:"
echo "   pm2 status     - check status"
echo "   pm2 logs       - view all logs"
echo "   pm2 logs $MINER_NAME - view miner logs"
echo "   pm2 monit      - monitor processes"