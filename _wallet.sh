#!/bin/bash
# Multi-wallet management for LegacyCoin node
# Structure:
#   wallets/
#   ├── miner/      (base mining wallet)
#   ├── pool/       (pool rewards)
#   ├── exchange/   (exchange hot wallet)
#   └── cold/       (cold storage)

WALLET_DIR="/app/LegacyCore/wallets"
NODE_DIR="/home/coin/.legacycoin"

show_usage() {
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  init                Initialize wallet structure"
    echo "  list                List all wallets"
    echo "  create <name>       Create new wallet"
    echo "  export <name>       Export wallet keys"
    echo "  balance [name]      Show balance (all or specific wallet)"
    echo "  address <name>      Get wallet address"
    echo ""
    echo "Wallet names: miner, pool, exchange, cold, ..."
}

init_wallets() {
    echo "═══════════════════════════════════════════════════════════"
    echo "  LegacyCoin Multi-Wallet Initialization"
    echo "───────────────────────────────────────────────────────────"
    
    # Create directory structure
    mkdir -p "$WALLET_DIR"/{miner,pool,exchange,cold}
    
    # Create wallet metadata
    for wallet in miner pool exchange cold; do
        if [ ! -f "$WALLET_DIR/$wallet/wallet.json" ]; then
            cat > "$WALLET_DIR/$wallet/wallet.json" << EOF
{
  "name": "$wallet",
  "created": "$(date -Iseconds)",
  "addresses": [],
  "description": ""
}
EOF
            echo "  ✓ Created: wallets/$wallet/"
        else
            echo "  → Exists: wallets/$wallet/"
        fi
    done
    
    # Create main wallet config
    if [ ! -f "$WALLET_DIR/wallets.json" ]; then
        cat > "$WALLET_DIR/wallets.json" << EOF
{
  "version": 1,
  "wallets": {
    "miner": {
      "description": "Base mining wallet - receives block rewards",
      "path": "miner/"
    },
    "pool": {
      "description": "Pool rewards distribution",
      "path": "pool/"
    },
    "exchange": {
      "description": "Exchange hot wallet",
      "path": "exchange/"
    },
    "cold": {
      "description": "Cold storage - long-term holdings",
      "path": "cold/"
    }
  }
}
EOF
        echo "  ✓ Created: wallets/wallets.json"
    fi
    
    echo "───────────────────────────────────────────────────────────"
    echo "  Structure:"
    echo ""
    echo "  wallets/"
    echo "  ├── wallets.json      (master config)"
    echo "  ├── miner/            (base mining)"
    echo "  ├── pool/             (pool rewards)"
    echo "  ├── exchange/         (hot wallet)"
    echo "  └── cold/             (cold storage)"
    echo ""
    echo "═══════════════════════════════════════════════════════════"
}

list_wallets() {
    echo "═══════════════════════════════════════════════════════════"
    echo "  LegacyCoin Wallets"
    echo "───────────────────────────────────────────────────────────"
    
    if [ ! -d "$WALLET_DIR" ]; then
        echo "  ✗ No wallets found. Run: $0 init"
        exit 1
    fi
    
    for wallet in "$WALLET_DIR"/*/; do
        if [ -d "$wallet" ]; then
            name=$(basename "$wallet")
            if [ -f "$wallet/wallet.json" ]; then
                desc=$(grep -o '"description": *"[^"]*"' "$wallet/wallet.json" | head -1 | cut -d'"' -f4)
                addrs=$(grep -o '"addresses": *\[.*\]' "$wallet/wallet.json" | grep -o '"L[a-zA-Z0-9]*"' | wc -l)
                echo "  📁 $name"
                echo "     Description: ${desc:-No description}"
                echo "     Addresses: $addrs"
            fi
            echo ""
        fi
    done
    
    echo "═══════════════════════════════════════════════════════════"
}

create_wallet() {
    local name=$1
    
    if [ -z "$name" ]; then
        echo "Error: Wallet name required"
        echo "Usage: $0 create <name>"
        exit 1
    fi
    
    if [ -d "$WALLET_DIR/$name" ]; then
        echo "Error: Wallet '$name' already exists"
        exit 1
    fi
    
    mkdir -p "$WALLET_DIR/$name"
    
    cat > "$WALLET_DIR/$name/wallet.json" << EOF
{
  "name": "$name",
  "created": "$(date -Iseconds)",
  "addresses": [],
  "description": ""
}
EOF
    
    echo "✓ Wallet '$name' created at wallets/$name/"
}

export_wallet() {
    local name=$1
    
    if [ -z "$name" ]; then
        echo "Error: Wallet name required"
        echo "Usage: $0 export <name>"
        exit 1
    fi
    
    if [ ! -d "$WALLET_DIR/$name" ]; then
        echo "Error: Wallet '$name' not found"
        exit 1
    fi
    
    echo "═══════════════════════════════════════════════════════════"
    echo "  Wallet: $name"
    echo "───────────────────────────────────────────────────────────"
    echo ""
    echo "  ⚠  WARNING: This shows sensitive wallet data!"
    echo ""
    cat "$WALLET_DIR/$name/wallet.json"
    echo ""
    echo "═══════════════════════════════════════════════════════════"
}

show_balance() {
    local name=$1
    
    echo "═══════════════════════════════════════════════════════════"
    echo "  LegacyCoin Balances"
    echo "───────────────────────────────────────────────────────────"
    
    # Check if legacycoin-cli is available
    if ! command -v legacycoin-cli &> /dev/null; then
        if [ -f "/app/LegacyCore/legacycoin-cli" ]; then
            CLI="/app/LegacyCore/legacycoin-cli"
        else
            echo "  ✗ legacycoin-cli not found"
            exit 1
        fi
    else
        CLI="legacycoin-cli"
    fi
    
    if [ -n "$name" ]; then
        echo "  Wallet: $name"
        echo "  $($CLI getbalance 2>/dev/null || echo 'Error getting balance')"
    else
        echo "  Total: $($CLI getbalance 2>/dev/null || echo 'Error getting balance')"
    fi
    
    echo "═══════════════════════════════════════════════════════════"
}

get_address() {
    local name=$1
    
    if [ -z "$name" ]; then
        echo "Error: Wallet name required"
        echo "Usage: $0 address <name>"
        exit 1
    fi
    
    if [ ! -d "$WALLET_DIR/$name" ]; then
        echo "Error: Wallet '$name' not found"
        exit 1
    fi
    
    # Get or create address for wallet
    if [ -f "$WALLET_DIR/$name/address.txt" ]; then
        cat "$WALLET_DIR/$name/address.txt"
    else
        echo "No address found for wallet '$name'"
        echo "Use legacycoin-cli to generate one"
    fi
}

# Main
case "${1:-}" in
    init)
        init_wallets
        ;;
    list)
        list_wallets
        ;;
    create)
        create_wallet "$2"
        ;;
    export)
        export_wallet "$2"
        ;;
    balance)
        show_balance "$2"
        ;;
    address)
        get_address "$2"
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
