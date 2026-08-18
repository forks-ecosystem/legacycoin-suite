#!/bin/bash
set -e

export HOME=/home/coin
export GOPATH=/home/coin/go
export GOCACHE=/home/coin/go/cache
export PATH=$PATH:/usr/local/go/bin

INSTALL_DIR="/app/legacycoin-explorer"

echo "============================================================"
echo "  LegacyCoin Explorer"
echo "============================================================"

# 1. Удаляем старую версию
if [ -d "$INSTALL_DIR" ]; then
    echo "📁 Removing old installation..."
    rm -rf "$INSTALL_DIR"
fi

# 2. Клонируем
echo "📦 Cloning repository..."
git clone https://github.com/legacybtc/legacycoin-explorer.git "$INSTALL_DIR"

cd "$INSTALL_DIR"

# 3. Билдим
echo "🔨 Building explorer..."
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w" -o explorer .

# 4. Проверяем
if [ -f "explorer" ]; then
    echo ""
    echo "✅ Build successful!"
    echo "   explorer: $(pwd)/explorer"
    echo ""
    ls -la explorer
else
    echo "❌ Build failed"
    exit 1
fi

echo "============================================================"
echo "✅ Done"
