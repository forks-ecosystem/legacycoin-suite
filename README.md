# LegacyCoin Suite

LegacyCoin Suite is the entry point and management console for the LBTC / ForkEX ecosystem. Suite does not store secrets.

The miner is the first and simplest component — a single step to start mining. Other components can be added as needed:

```text
LegacyCoin Suite
│
├── 1. Miner
│      └── first, simple entry
│
├── 2. Pool
│      └── setup / configuration / monitoring
│
├── 3. Node
│      └── optional node installation and management
│
└── 4. Explorer
       └── optional block explorer
```

## Miner

A CPU miner for LegacyCoin (LBTC) with Stratum v1 protocol support and embedded web dashboard.

## Features

- Stratum v1 mining protocol
- Multi-threaded CPU mining
- Yespower PoW algorithm (CGO-optimized C implementation with AVX2 support)
- Embedded web dashboard on port 3002
- Configuration via `config.json` (or env vars)
- Docker deployment with automatic restart

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
CGO_CFLAGS="-O3 -mavx2 -mavx -msse4.1" go build -o legacycoin-suite .
```

Plain `go build` also works but only uses SSE2; for max hashrate on x86-64 CPUs add the AVX2 flags above.

### 3. Run with Docker (recommended)

```bash
docker compose build suite
docker compose up -d suite
```

Open http://localhost:3002 for the dashboard.

### Run directly

```bash
./legacycoin-suite -web=:3002
```

## Docker Management

```bash
docker compose ps                    # check status
docker compose logs -f suite         # tail logs
docker compose restart suite
docker compose stop suite
docker compose down suite
```

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
- `/admin` — Admin panel (setup/login/config/wallet)

## Scripts

Scripts use underscore prefix to appear at the top of `ls` output.
Double `__` for essential management scripts, single `_` for utilities.

### Management (`__`)

| Script | Description |
|--------|-------------|
| `__up.sh` | Start container and show status |
| `__down.sh` | Stop container |

### Installation (`_`)

| Script | Description | Installs to |
|--------|-------------|-------------|
| `_git_LegacyCore.sh` | Build legacycoind + legacycoin-cli | `/app/LegacyCore` |
| `_git_legacycoin-explorer.sh` | Build block explorer | `/app/legacycoin-explorer` |
| `_git_legasybtc-pool.sh` | Install mining pool | `/app/legacybtc-pool` |

### Utility (`_`)

| Script | Description |
|--------|-------------|
| `_port_check.sh` | Check port availability |
| `_set_all.sh` | Set all environment variables |
