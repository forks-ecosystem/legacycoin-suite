#!/bin/bash
# LegacyBTC Pool Management Script

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

POOL_DIR="$(cd "$(dirname "$0")" && pwd)"
PM2_NAME="legacybtc-pool"
export PATH="$HOME/.local/bin:$PATH"

cd "$POOL_DIR"

show_help() {
    echo -e "${YELLOW}Usage:${NC} $0 {install|start|stop|restart|status|logs|rebuild}"
    echo ""
    echo "  ${GREEN}install${NC}  - Install dependencies and push DB schema"
    echo "  ${GREEN}start${NC}    - Build & start the pool with PM2"
    echo "  ${GREEN}stop${NC}     - Stop the pool"
    echo "  ${GREEN}restart${NC}  - Restart the pool"
    echo "  ${GREEN}status${NC}   - Show pool status"
    echo "  ${GREEN}logs${NC}     - Show live logs"
    echo "  ${GREEN}rebuild${NC}  - Rebuild artifacts and restart"
    echo ""
}

case "$1" in
    install)
        echo -e "${YELLOW}Installing dependencies...${NC}"
        pnpm install --no-frozen-lockfile
        echo -e "${YELLOW}Pushing DB schema...${NC}"
        cd lib/db && DATABASE_URL="$DATABASE_URL" pnpm exec drizzle-kit push --config ./drizzle.config.ts && cd "$POOL_DIR"
        echo -e "${GREEN}✅ Done${NC}"
        ;;
    start)
        echo -e "${YELLOW}Building api-server...${NC}"
        pnpm --filter @workspace/api-server run build
        echo -e "${YELLOW}Starting with PM2...${NC}"
        if pm2 list 2>/dev/null | grep -q "$PM2_NAME"; then
            pm2 restart "$PM2_NAME"
        else
            pm2 start ecosystem.config.js
        fi
        pm2 save
        echo -e "${GREEN}✅ Pool started${NC}"
        ;;
    stop)
        pm2 stop "$PM2_NAME"
        echo -e "${GREEN}✅ Pool stopped${NC}"
        ;;
    restart)
        pm2 restart "$PM2_NAME"
        echo -e "${GREEN}✅ Pool restarted${NC}"
        ;;
    status)
        pm2 status "$PM2_NAME"
        pm2 info "$PM2_NAME" 2>/dev/null
        ;;
    logs)
        pm2 logs "$PM2_NAME" --lines 100
        ;;
    rebuild)
        echo -e "${YELLOW}Rebuilding...${NC}"
        pnpm --filter @workspace/api-server run build
        pm2 restart "$PM2_NAME"
        echo -e "${GREEN}✅ Rebuild complete${NC}"
        ;;
    *)
        show_help
        exit 1
        ;;
esac
