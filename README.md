# LegacyCoin Miner

A CPU miner for LegacyCoin (LBTC) with Stratum v1 protocol support and embedded web dashboard.

## Features

- Stratum v1 mining protocol
- Multi-threaded CPU mining
- Yespower PoW algorithm (CGO-optimized C implementation with AVX2 support)
- Embedded web dashboard on port 3002
- Configuration via `config.json` (or env vars)
- PM2 process management with automatic log rotation

## Quick Start

### 1. Configure

Edit `config.json`:

```json
{
  "wallet": "YOUR_WALLET_ADDRESS",
  "pool": "stratum+tcp://127.0.0.1:3333",
  "workers": 4,
  "worker": "cpu"
}
```

Environment variables (`WALLET`, `POOL`, `WORKERS`, `WORKER`) override `config.json` when set.

### 2. Build

```bash
# CGO is required for the optimized yespower implementation.
# Build with CPU-specific optimizations for best hashrate:
CGO_CFLAGS="-O3 -mavx2 -mavx -msse4.1" go build -o legacycoin-miner .
```

Plain `go build` also works but only uses SSE2; for max hashrate on x86-64 CPUs add the AVX2 flags above.

### 3. Run with PM2 (recommended)

```bash
pm2 start ecosystem.config.js
pm2 save
```

Open http://localhost:3002 for the dashboard.

### Run directly

```bash
./legacycoin-miner -web=:3002
```

## PM2 Management

```bash
pm2 status                  # check status
pm2 logs legacycoin-miner   # tail logs
pm2 restart legacycoin-miner
pm2 stop legacycoin-miner
pm2 delete legacycoin-miner
```

### Startup on boot

```bash
pm2 startup               # run the printed command once with sudo
pm2 save                  # persist the current process list
```

### Log Rotation

PM2 logs are written to `./logs/`. Automatic cleanup is configured with `pm2-logrotate`:

```bash
pm2 install pm2-logrotate
pm2 set pm2-logrotate:max_size 10M
pm2 set pm2-logrotate:retain 7
pm2 set pm2-logrotate:compress true
pm2 set pm2-logrotate:rotateInterval '0 0 * * *'
```

Logs are rotated at 10 MB (or daily), keeping 7 rotations, compressed with gzip.

## Flags

| Flag  | Default | Description          |
|-------|---------|----------------------|
| `-web`| `:3002` | Web server address   |

All mining settings (wallet, pool, workers) come from `config.json` or env vars.

## Web Dashboard

The embedded web server provides:
- `/` — HTML dashboard with real-time stats
- `/api/stats` — JSON endpoint with hashrate, accepted shares, uptime
- `/api/config` — GET/PUT the miner configuration
- `/api/miner/start|stop|restart` — control the miner
