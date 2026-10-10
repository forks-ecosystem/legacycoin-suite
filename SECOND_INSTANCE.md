# Second instance — provisioning checklist

How to stand up a second, independent LegacyCoin / ForkEX instance on a new
server: bring the code from git, then move the pieces that are **not** in git
(secrets, configs, databases, live state, TLS).

> This is a second **instance**, not a mirror: give it its own wallet keys,
> miner wallet, DB passwords and domain. Do **not** reuse production secrets.

## 1. Code from git

All projects live in the `forks-ecosystem` org and are cloned under `/app`:

| Repo | Path | Role |
|------|------|------|
| legacycoin-suite | /app/legacycoin-suite | miner + admin console (:3002) |
| forkex | /app/forkex | exchange API / web / ws / plugins / bot |
| legacybtc-pool | /app/legacybtc-pool | mining pool |
| legacycoin-explorer | /app/legacycoin-explorer | block explorer |
| legacycore | /app/legacycore | node / wallet (`legacycoind`) |
| legacycoreagent | /app/legacycoreagent | node agent |
| forks-agent | /app/forks-agent | |
| forks-life-node | /app/forks-life-node | |
| forks-life-navigator | /app/forks-life-navigator | |
| forks-life-supervisor | /app/forks-life-supervisor | |
| server-coin | /app/server-coin | |

Remotes use the SSH alias `github-forks:` — install the deploy key and an
`~/.ssh/config` entry on the new host (or add a `github.com` remote).

Later updates: `cd /app/legacycoin-suite && ./__update_from_git.sh`.

## 2. Secrets & configs NOT in git (copy by hand)

These paths are gitignored and must be transferred (or recreated):

| Project | Gitignored files |
|---------|------------------|
| legacycoin-suite | `.env`, `config/admin.json`, `config/miner.json` |
| forkex | `.env`, `server/tools/hollaex-kit.env.local`, `templates/local/*.env.local`, `plugins/config/`, `plugins/json/`, `*.pem`, `*.key`, `backups/` |
| legacybtc-pool | `config/admin.json`, `config/pool.env`, `artifacts/api-server/.env`, `data/` |
| legacycoin-explorer | `.env`, `data/` |
| legacycore | `data/`, node wallets (`/home/coin/.legacycoin/wallets/`) |
| server-coin / forks-agent / forks-life-navigator / forks-life-supervisor | `.env` |
| forks-life-node | `.env` / `credentials.env`, `/var/www/html/fork/db/`, `/var/www/utx/` |

## 3. Databases (Postgres / Redis)

Running containers:

- `forkex-db` — postgres:14.9-alpine, `127.0.0.1:5434` → exchange DB (`hollaex`, `admin` user)
- `wallet-postgres` — postgres:15
- `legacybtc-pool-postgres` — postgres:15-alpine
- `forkex-redis` — redis:8.0.2-alpine

Dump on source, restore on target, e.g.:

```sh
docker exec forkex-db pg_dumpall -U admin > forkex-all.sql
# on target:
docker exec -i forkex-db psql -U admin < forkex-all.sql
```

## 4. Live state files (resume position)

- `/app/forkex/wallet-scanner/scanner-state.json` — wallet-scanner position
- `/app/forkex/server/scripts/lbtc-monitor-state.json` — lbtc-deposit-monitor position
- `/home/coin/.legacycoin/` — node data dir, incl. `wallets/`

Copy them, or reset to the current chain height on a fresh instance.

## 5. systemd units + ops env

- `/etc/systemd/system/`: `wallet-scanner.service`, `lbtc-deposit-monitor.service`,
  `legacycoind.service`, `lbtc-legacycoind.service`, `legacycoin-explorer.service`,
  `legacybtc-pool-reload.path`, `legacybtc-pool-reload.service`,
  `miningcore-lbtc.service`, `forkex-bot.service`, `forkex-db-proxy.service`
- `/etc/lbtc-ops/`: `lbtc-ops.env`, `wallet-scanner.env`, `order-tracker.env`,
  `state-anchor.env`, `run-forkex-api-v16.sh`

On target: `systemctl daemon-reload && systemctl enable --now <unit>`.

## 6. nginx + TLS + DNS

- `/etc/nginx/sites-enabled/`: `default`, `explorer.lbtc.forkex.life`,
  `hollaex.forkex.life`, `lbtc.forkex.life`, `my.forkex.life`,
  `pool.lbtc.forkex.life`, `wallet.forkex.life`
- `/etc/letsencrypt/` — re-issue via certbot or copy the certs
- DNS records for the chosen domains

## 7. Non-git directories (copy manually)

`/app/185.253.219.51/`, `/app/bin/`, `/app/legacymobile/`, `/app/ping-agent/`.

## 8. Order of operations

1. Install deploy key; `git clone` all repos under `/app`.
2. Install systemd units and `/etc/lbtc-ops/`.
3. Start Postgres / Redis containers; restore DB dumps.
4. Copy secrets & configs (section 2).
5. Copy state files or reset to current height (section 4).
6. Configure nginx + TLS + DNS.
7. Build & start: `cd /app/legacycoin-suite && ./__rebuild.sh`, then each other project's own build/start.
