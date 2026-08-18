#!/bin/bash
set -e

export HOME=/home/coin
export GOPATH=/home/coin/go
export GOCACHE=/home/coin/go/cache
export PATH=$PATH:/usr/local/go/bin

INSTALL_DIR="/app/LegacyCore"

echo "============================================================"
echo "  LegacyCore (legacycoind + legacycoin-cli)"
echo "============================================================"

# 1. Удаляем старую версию
if [ -d "$INSTALL_DIR" ]; then
    echo "📁 Removing old installation..."
    rm -rf "$INSTALL_DIR"
fi

# 2. Клонируем
echo "📦 Cloning repository..."
git clone https://github.com/legacybtc/LegacyCore.git "$INSTALL_DIR"

cd "$INSTALL_DIR"

# 3. Билдим
echo "🔨 Building legacycoind..."
go build -buildvcs=false -trimpath -ldflags "-s -w" -o legacycoind ./cmd/legacycoind

echo "🔨 Building legacycoin-cli..."
go build -buildvcs=false -trimpath -ldflags "-s -w" -o legacycoin-cli ./cmd/legacycoin-cli

# 4. Инициализация мульти-кошельков
echo ""
echo "📂 Initializing multi-wallet structure..."
WALLET_DIR="$INSTALL_DIR/wallets"
mkdir -p "$WALLET_DIR"/{miner,pool,exchange,cold}

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
        echo "   ✓ wallets/$wallet/"
    fi
done

if [ ! -f "$WALLET_DIR/wallets.json" ]; then
    cat > "$WALLET_DIR/wallets.json" << EOF
{
  "version": 1,
  "wallets": {
    "miner": { "description": "Base mining wallet", "path": "miner/" },
    "pool": { "description": "Pool rewards", "path": "pool/" },
    "exchange": { "description": "Exchange hot wallet", "path": "exchange/" },
    "cold": { "description": "Cold storage", "path": "cold/" }
  }
}
EOF
    echo "   ✓ wallets/wallets.json"
fi

# 5. Проверяем
if [ -f "legacycoind" ] && [ -f "legacycoin-cli" ]; then
    echo ""
    echo "✅ Build successful!"
    echo "   legacycoind:  $(pwd)/legacycoind"
    echo "   legacycoin-cli: $(pwd)/legacycoin-cli"
    echo "   wallets:      $(pwd)/wallets/"
    echo ""
    ls -la legacycoind legacycoin-cli
else
    echo "❌ Build failed"
    exit 1
fi

echo "============================================================"
echo "✅ Done"
