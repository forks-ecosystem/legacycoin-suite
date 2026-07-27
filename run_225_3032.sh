#!/bin/bash
# miner-control.sh

MINER_NAME="legacycoin-miner"
MINER_CMD="./legacycoin-miner -wallet=Li8Q3GonH6iocGzfZLGxbpjazqnmez8zrJ.20 -pool=stratum+tcp://forkex.net:3032 -workers=4"

case "$1" in
    start)
        pm2 start "$MINER_CMD" --name "$MINER_NAME"
        pm2 save
        echo " Miner started"
        ;;
    stop)
        pm2 stop "$MINER_NAME"
        echo " Miner stopped"
        ;;
    restart)
        pm2 restart "$MINER_NAME"
        echo " Miner restarted"
        ;;
    status)
        pm2 show "$MINER_NAME"
        ;;
    logs)
        pm2 logs "$MINER_NAME"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        exit 1
        ;;
esac