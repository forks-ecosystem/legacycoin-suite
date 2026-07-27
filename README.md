# LegacyCoin Miner

A CPU miner for LegacyCoin (LBTC) with Stratum v1 protocol support and embedded web dashboard.

## Features

- Stratum v1 mining protocol
- Multi-threaded CPU mining
- Yespower PoW algorithm
- Embedded web dashboard on port 3002
- Docker support

## Quick Start

### Docker

```bash
WALLET=Li8Q3GonH6iocGzfZLGxbpjazqnmez8zrJ.20 POOL=stratum+tcp://forkex.net:3032 WORKERS=4 docker compose up -d
```

Open http://localhost:3002 for the dashboard.

### Native

```bash
go build -o legacycoin-miner .
./legacycoin-miner -wallet=YOUR_WALLET -pool=stratum+tcp://127.0.0.1:3333 -workers=4
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-wallet` | — | Wallet address (required) |
| `-pool` | `stratum+tcp://127.0.0.1:3333` | Pool URL |
| `-workers` | CPU count | Mining threads |
| `-worker` | `cpu` | Worker name |
| `-web` | `:3002` | Web server address |

## Web Dashboard

The embedded web server provides:
- `/` — HTML dashboard with real-time stats
- `/api/stats` — JSON endpoint with hashrate, accepted shares, uptime
