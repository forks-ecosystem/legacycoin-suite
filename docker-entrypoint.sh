#!/bin/sh
set -e

if [ -z "$WALLET" ]; then
  echo "FATAL: WALLET environment variable is required"
  exit 1
fi

WEB="${WEB:-:3002}"

exec legacycoin-miner -web="$WEB"
