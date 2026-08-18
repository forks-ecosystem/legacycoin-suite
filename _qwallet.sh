#!/bin/bash
# Quick wallet operations via legacycoin-cli

CLI="/app/LegacyCore/legacycoin-cli"
COOKIE="/home/coin/.legacycoin/.cookie"

# Read credentials from cookie file
RPC_USER="__cookie__"
RPC_PASS=$(cat "$COOKIE" | cut -d: -f2)

RPC="$CLI -rpcuser=$RPC_USER -rpcpassword=$RPC_PASS"

case "${1:-}" in
    address|addr)
        echo "New address:"
        $RPC getnewaddress
        ;;
    balance|bal)
        echo "Balance:"
        $RPC getbalance
        ;;
    info)
        $RPC getwalletinfo
        ;;
    send)
        if [ -z "$2" ] || [ -z "$3" ]; then
            echo "Usage: _qwallet.sh send <address> <amount>"
            exit 1
        fi
        echo "Sending $3 LBTC to $2..."
        $RPC sendtoaddress "$2" "$3" --yes
        ;;
    list)
        $RPC listtransactions 20
        ;;
    unspent)
        $RPC listunspent
        ;;
    *)
        echo "LegacyCoin Quick Wallet"
        echo ""
        echo "Usage: _qwallet.sh <command>"
        echo ""
        echo "Commands:"
        echo "  address      New address"
        echo "  balance      Show balance"
        echo "  info         Wallet info"
        echo "  send <addr> <amount>  Send LBTC"
        echo "  list         Recent transactions"
        echo "  unspent      Unspent outputs"
        ;;
esac
