#!/bin/sh
set -e

WALLET="${WALLET:?WALLET environment variable is required}"
POOL="${POOL:-stratum+tcp://127.0.0.1:3333}"
WORKERS="${WORKERS:-4}"
WEB="${WEB:-:3002}"

exec legacycoin-miner -wallet="$WALLET" -pool="$POOL" -workers="$WORKERS" -web="$WEB"
