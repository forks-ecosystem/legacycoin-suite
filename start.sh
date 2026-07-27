#!/bin/bash

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Building api-server...${NC}"
pnpm --filter workspace/api-server run build

echo -e "${YELLOW}Starting with PM2...${NC}"
pm2 start ecosystem.config.js

pm2 save

echo -e "${GREEN} All processes started${NC}"
echo ""
echo " Management:"
echo "   pm2 status"
echo "   pm2 logs"
echo "   pm2 stop all"
echo "   pm2 restart all"