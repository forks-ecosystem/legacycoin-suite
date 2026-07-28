# LegacyCoin Miner

A CPU miner for LegacyCoin (LBTC) with Stratum v1 protocol support and embedded web dashboard.

## Features

- Stratum v1 mining protocol
- Multi-threaded CPU mining
- Yespower PoW algorithm
- Embedded web dashboard on port 3002
- PM2 process management with log rotation

## Quick Start

### Build

```bash
go build -o legacycoin-miner .
```

### Run with PM2 (recommended)

```bash
# Edit wallet/pool in ecosystem.config.js, then:
pm2 start ecosystem.config.js
```

Open http://localhost:3002 for the dashboard.

### Run directly

```bash
./legacycoin-miner -wallet=YOUR_WALLET -pool=stratum+tcp://127.0.0.1:3333 -workers=4
```

## PM2 Management

```bash
pm2 status                  # check status
pm2 logs legacycoin-miner   # tail logs
pm2 restart legacycoin-miner
pm2 stop legacycoin-miner
pm2 delete legacycoin-miner
```

### Log Rotation

PM2 logs are written to `./logs/`. To enable automatic cleanup:

```bash
pm2 install pm2-logrotate
pm2 set pm2-logrotate:max_size 50M
pm2 set pm2-logrotate:retain 7
pm2 set pm2-logrotate:compress true
```

Or use system `logrotate` (`/etc/logrotate.d/legacycoin-miner`):

```
/path/to/legacycoin-miner/logs/*.log {
    daily
    rotate 7
    compress
    copytruncate
    missingok
    notifempty
}
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
