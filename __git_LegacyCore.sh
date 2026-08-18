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

# 4. Проверяем
if [ -f "legacycoind" ] && [ -f "legacycoin-cli" ]; then
    echo ""
    echo "✅ Build successful!"
    echo "   legacycoind:  $(pwd)/legacycoind"
    echo "   legacycoin-cli: $(pwd)/legacycoin-cli"
    echo ""
    ls -la legacycoind legacycoin-cli
else
    echo "❌ Build failed"
    exit 1
fi

echo "============================================================"
echo "✅ Done"
