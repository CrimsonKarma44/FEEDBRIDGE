# FEEDBRIDGE

Intelligent RSS-to-Chat Delivery Platform

FEEDBRIDGE detects syndication feeds (RSS / Atom / JSON Feed) for any site URL, caches them, and delivers new posts as Telegram messages to private chats, groups, and communities on a schedule.

## Architecture

```
Telegram  <-->  bot/  --gRPC-->  API/  -->  Postgres + Redis
                  |
                  +-- scheduler (30s tick) -> worker pool (10) -> dedupe -> delivery
```

| Component | What it does |
|---|---|
| `API/` | Go gRPC server on `:50051`. Feed detection via [rss_detector](https://github.com/CrimsonKarma44/rss_detector), link registry in Postgres (GORM), Redis caching with TTLs (feed items 10 min, feed links 24 h, negative results 5 min), singleflight coalescing of identical requests, panic-recovery + logging interceptors, gRPC health checks & reflection |
| `bot/` | Telebot v4 client. Per-chat subscriptions stored in the same Postgres, admin-only management inside groups, scheduled polling, hybrid delivery: up to 5 new items as individual messages, larger batches collapsed into one digest |

### API services (`handler` package)

| RPC | Description |
|---|---|
| `SetUrlHandler/SetUrl(url)` | Detects feeds for a URL, persists to Postgres, caches in Redis, returns detected feed links |
| `FeedHandlerService/GetFeed(url, from)` | Returns items published at/after `from` (undated items always included), newest first |

## Setup

### Prerequisites

- Go 1.26+
- PostgreSQL
- Redis
- A Telegram bot token from [@BotFather](https://t.me/BotFather)

### 1. Configure

```bash
cp API/.env.example API/.env      # fill in DB + Redis values
cp bot/.env.example bot/.env      # fill in TELEGRAM_BOT_TOKEN + DB values
```

| Variable | Used by | Purpose |
|---|---|---|
| `DB_HOST` `DB_USER` `DB_PASSWORD` `DB_NAME` `DB_PORT` | both | Shared PostgreSQL instance |
| `REDIS_ADDR` `REDIS_PASSWORD` `REDIS_DB` `REDIS_PROTOCOL` | API | Redis cache backend |
| `API_ADDR` | bot | gRPC endpoint of the API (default `localhost:50051`) |
| `TELEGRAM_BOT_TOKEN` | bot | Bot token from BotFather |
| `YOUTUBE_API_KEY` | API | Optional; prefers Data API v3 in the YouTube resolver |

> Note: `.env` files are gitignored — never commit them.

### 2. Run the API

```bash
cd API
make run          # builds ./feedbridge then executes it
```

The server listens on `:50051`. Schema is auto-migrated on startup.

### 3. Run the bot

```bash
cd bot
go run .
```

## Bot commands

| Command | Description |
|---|---|
| `/start` | Greet (adapts to private chat vs group/community) |
| `/configure` | Delivery target picker (this chat / group / community deep-links) |
| `/addfeed <url>` | Detect feeds for a URL and subscribe this chat |
| `/listfeed` | Inline list with Enable / Disable / Remove buttons per subscription |
| `/removefeed <url>` | Unsubscribe by URL |
| `/disablefeed <url>` / `/enablefeed <url>` | Pause / resume a subscription without deleting it |
| `/interval <duration>` | Refresh cadence for all feeds in this chat (`15m`, `1h`, ... clamped to 5 m - 24 h) |

Groups & communities: subscriptions are keyed by chat ID, so adding the bot via
`t.me/feed_bridge_bot?startgroup=true` and running `/addfeed` inside the group just works.
In groups only **admins** can manage feeds.

## Development notes

- Generated protobuf code lives in `API/protoAPI/` (committed). After editing `API/proto/*.proto`, regenerate with `protoc`.
- `bot/go.mod` uses local `replace` directives pointing at `../API` and the local `rss_detector` checkout — adjust paths if your layout differs.
- `API/youtube/` resolves any YouTube video/handle/channel URL to its Atom feed (Innertube first, Data API v3 when a key is set).
- Cache layers: repeat `GetFeed` calls within the item TTL are served entirely from Redis.
- Subscriptions are keyed on the **resolved feed URL** (not the input URL): adding any page of a site whose feed you already follow answers "already subscribed" instead of creating a duplicate.

## Deployment (Oracle Cloud Always Free — single VM, $0)

The whole stack runs on one always-free ARM VM: both binaries as systemd services, native Postgres + Redis. Nothing is exposed publicly — the bot polls Telegram outbound and talks to the API over loopback.

### 1. Account & VM
1. Sign up at oracle.com/cloud/free (card needed for identity only). Pick a **low-demand region** for ARM capacity — it's permanent.
2. Compute → Create Instance: Ubuntu 24.04 **aarch64**, shape `Ampere A1 Flex` (2 OCPU / 12 GB), your SSH key. Security list: port 22 only.

### 2. Bootstrap
```bash
scp -r deploy ubuntu@<IP>:~            # or git clone the repo on the VM
ssh ubuntu@<IP>
cd ~/deploy && sudo ./setup-server.sh  # packages, DB+user creation prompts, redis password, ufw
```
Copy envs into place:
```bash
sudo cp ~/deploy/env/api.env.example /etc/feedbridge/api.env
sudo cp ~/deploy/env/bot.env.example /etc/feedbridge/bot.env
sudo chmod 600 /etc/feedbridge/*.env && sudoedit each   # fill secrets; REDIS_PASSWORD printed by setup
```

### 3. Ship binaries
```bash
# from your machine, repo root:
API_HOST=... bot deploy targets:
make -C API deploy HOST=ubuntu@<IP>
make -C bot deploy HOST=ubuntu@<IP>
```

### 4. Services + data migration
```bash
sudo cp ~/deploy/feedbridge-*.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now feedbridge-api feedbridge-bot
journalctl -u feedbridge-api -f          # verify

# optional: migrate existing local data (subscriptions + detection cache)
pg_dump -U deus -h localhost -d feedbridge -t subscriptions -t link_repositories | \
  ssh ubuntu@<IP> 'sudo -u postgres psql -d feedbridge'
```

### 5. Ops
- Backups: install `deploy/backup-db.sh` as root cron (`0 4 * * *`), keeps 7 dumps.
- Redeploys after changes: same `make deploy` one-liners (Restart=always units).
- Gotchas: region is permanent; ARM capacity may require retries; consider upgrading to Pay-As-You-Go (still $0 within limits) to remove idle-reclaim risk; never open 50051/5432/6379 publicly.

## Docker (optional)

Both modules compile to fully static binaries, so containerization stays optional:

```bash
docker build -f API/Dockerfile . && docker build -f bot/Dockerfile .
docker compose -f docker-compose.example.yml up -d    # full parity stack incl. pg+redis
```

Caveat: both modules use local `replace` directives pointing at the sibling `rss_detector` checkout, which lives outside this repo. Container builds therefore need that source inside the build context (copy it in, or vendor/publish `rss_detector` — a good future cleanup). The systemd path has no such requirement.

