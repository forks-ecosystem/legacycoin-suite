#!/bin/bash

export HOME=/home/coin
export GOPATH=/home/coin/go
export GOCACHE=/home/coin/go/cache
export PATH=$PATH:/usr/local/go/bin

echo "📊 Monitoring..."
echo "------------------------------------------------------------"

# 1. Удаляем старую версию
echo "📁 Removing old installation..."
rm -rf /home/coin/LegacyCore

# 2. Клонируем
echo "📦 Cloning repository..."
cd /home/coin
git clone https://github.com/legacybtc/LegacyCore.git

cd /home/coin/LegacyCore

echo "🔨 Building legacycoind..."
go build -buildvcs=false -trimpath -ldflags "-s -w" -o legacycoind ./cmd/legacycoind

echo "🔨 Building legacycoin-cli..."
go build -buildvcs=false -trimpath -ldflags "-s -w" -o legacycoin-cli ./cmd/legacycoin-cli

if [ -f "legacycoind" ] && [ -f "legacycoin-cli" ]; then
    echo "✅ Build successful!"
    ls -la legacycoind legacycoin-cli
else
    echo "❌ Build failed"
    exit 1
fi

echo "------------------------------------------------------------"
echo "✅ Done"

