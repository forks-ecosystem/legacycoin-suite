#!/bin/bash
# LegacyCoin Miner Management Script

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

POOL_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVICE="legacycoin-miner"

cd "$POOL_DIR"

print() {
  local color="$1"
  local msg="$2"
  printf "${color}%s${NC}\n" "$msg"
}

build_miner() {
  print "$YELLOW" "Building legacycoin-miner..."
  go build -o legacycoin-miner .
  print "$GREEN" "Build complete"
}

build_all() {
  build_miner
}

show_help() {
  printf "${YELLOW}Usage:${NC} %s {install|build|start|stop|restart|status|logs|rebuild|config|edit|logrot-setup|logrot-clean}\n\n" "$0"
  printf "  ${GREEN}install${NC}        - Install dependencies (go mod download)\n"
  printf "  ${GREEN}build${NC}          - Compile miner binary\n"
  printf "  ${GREEN}start${NC}          - Build & start via PM2\n"
  printf "  ${GREEN}stop${NC}           - Stop via PM2\n"
  printf "  ${GREEN}restart${NC}        - Restart via PM2\n"
  printf "  ${GREEN}status${NC}         - Show PM2 status\n"
  printf "  ${GREEN}logs${NC}           - Show live logs (pm2 logs)\n"
  printf "  ${GREEN}rebuild${NC}        - Rebuild binary and restart\n"
  printf "  ${GREEN}config${NC}         - Show current config\n"
  printf "  ${GREEN}edit${NC}           - Edit config.json\n"
  printf "  ${GREEN}logrot-setup${NC}   - Install pm2-logrotate for auto log cleanup\n"
  printf "  ${GREEN}logrot-clean${NC}   - Manually clean old logs\n\n"
}

logrot_setup() {
  print "$YELLOW" "Installing pm2-logrotate..."
  pm2 install pm2-logrotate
  pm2 set pm2-logrotate:max_size 50M
  pm2 set pm2-logrotate:retain 7
  pm2 set pm2-logrotate:compress true
  pm2 set pm2-logrotate:workerInterval 3600
  pm2 set pm2-logrotate:rotateInterval '0 0 * * *'
  print "$GREEN" "pm2-logrotate installed: max_size=50M, retain=7 days, compress=true"
}

logrot_clean() {
  print "$YELLOW" "Cleaning logs older than 7 days..."
  find "$POOL_DIR/logs" -type f -name "*.log" -mtime +7 -delete 2>/dev/null
  find "$POOL_DIR/logs" -type f -name "*.gz" -mtime +7 -delete 2>/dev/null
  print "$GREEN" "Old logs cleaned"
}

show_config() {
  if [ -f config.json ]; then
    print "$BLUE" "Current config:"
    cat config.json
  else
    print "$RED" "config.json not found"
  fi
}

case "$1" in
  install)
    print "$YELLOW" "Installing dependencies..."
    go mod download
    print "$GREEN" "Done"
    ;;
  build)
    build_all
    print "$GREEN" "Build complete"
    ;;
  start)
    build_all
    print "$YELLOW" "Starting via PM2..."
    pm2 start ecosystem.config.js
    pm2 save
    print "$GREEN" "Service started via PM2"
    ;;
  stop)
    pm2 stop "$SERVICE"
    print "$GREEN" "Service stopped"
    ;;
  restart)
    pm2 restart ecosystem.config.js
    print "$GREEN" "Service restarted"
    ;;
  status)
    pm2 status "$SERVICE"
    ;;
  logs)
    pm2 logs "$SERVICE" --lines 100
    ;;
  rebuild)
    print "$YELLOW" "Rebuilding..."
    build_all
    pm2 restart ecosystem.config.js
    print "$GREEN" "Rebuild complete"
    ;;
  config)
    show_config
    ;;
  edit)
    show_config
    print "$YELLOW" "Opening config.json for editing..."
    ${EDITOR:-vi} config.json
    print "$GREEN" "To apply changes, restart: $0 restart"
    ;;
  logrot-setup)
    logrot_setup
    ;;
  logrot-clean)
    logrot_clean
    ;;
  *)
    show_help
    exit 1
    ;;
esac
