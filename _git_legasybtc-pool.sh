#!/bin/bash
set -e

export HOME=/home/coin
export GOPATH=/home/coin/go
export GOCACHE=/home/coin/go/cache
export PATH=$PATH:/usr/local/go/bin

INSTALL_DIR="/app/legacybtc-pool"

echo "============================================================"
echo "  LegacyBTC Pool"
echo "============================================================"

# 1. Удаляем старую версию
if [ -d "$INSTALL_DIR" ]; then
    echo "📁 Removing old installation..."
    rm -rf "$INSTALL_DIR"
fi

# 2. Клонируем
echo "📦 Cloning repository..."
git clone https://github.com/legacybtc/legacybtc-pool.git "$INSTALL_DIR"

cd "$INSTALL_DIR"

# 3. Устанавливаем зависимости
echo "📦 Installing dependencies..."
if command -v pnpm &> /dev/null; then
    pnpm install
elif command -v npm &> /dev/null; then
    npm install
fi

# 4. Проверяем
if [ -d "node_modules" ]; then
    echo ""
    echo "✅ Installation successful!"
    echo "   Location: $(pwd)"
    echo ""
    ls -la
else
    echo "❌ Installation failed"
    exit 1
fi

echo "============================================================"
echo "✅ Done"
