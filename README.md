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

## Deployment (Google Cloud Always Free — e2-micro, $0)

One **amd64** VM: native Postgres + Redis, API and bot as systemd services. Nothing is public except SSH. The bot talks to Telegram outbound; the API listens on `127.0.0.1:50051` only.

Do **not** run Docker Compose on this machine (1 GB RAM). Do **not** compile on the VM (Go + `rss_detector` replace path live on your laptop). Cross-compile `linux/amd64` at home and `scp` the binaries.

### 1. Account & VM

Always Free `e2-micro` is **one instance**, only in `us-central1`, `us-west1`, or `us-east1`. A billing account is still required.

Console: Compute Engine → Create instance — name `feedbridge`, region `us-central1`, zone `us-central1-a`, machine **e2-micro**, Ubuntu 24.04 LTS **amd64**, 20–30 GB standard disk, **HTTP/HTTPS off**. Firewall: SSH (`tcp:22`) only.

Or from your laptop (gcloud SDK, project already selected):

```bash
./deploy/gcp/create-vm.sh
```

### 2. Bootstrap (on the VM)

```bash
# from your laptop
gcloud compute ssh feedbridge --zone=us-central1-a
# or: ssh YOU@EXTERNAL_IP

# copy the deploy/ tree once
# from laptop:  gcloud compute scp --recurse deploy feedbridge:~/deploy --zone=us-central1-a

cd ~/deploy && sudo ./setup-server.sh
```

The script installs Postgres + Redis, creates user `feedbridge`, tightens memory for 1 GB, enables UFW (SSH only), and prints `REDIS_PASSWORD`. Then:

```bash
sudo cp ~/deploy/env/api.env.example /etc/feedbridge/api.env
sudo cp ~/deploy/env/bot.env.example /etc/feedbridge/bot.env
sudo chmod 600 /etc/feedbridge/*.env
sudoedit /etc/feedbridge/api.env /etc/feedbridge/bot.env
```

Fill `DB_PASSWORD`, `REDIS_PASSWORD`, and `TELEGRAM_BOT_TOKEN`. Keep `WORKER_COUNT=3` and `LISTEN_ADDR=127.0.0.1:50051`.

```bash
sudo cp ~/deploy/feedbridge-*.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### 3. Ship binaries (from your laptop)

```bash
# repo root; HOST is the SSH user@external-ip of the VM
make -C API deploy HOST=YOU@EXTERNAL_IP
make -C bot deploy HOST=YOU@EXTERNAL_IP
```

### 4. Start + optional data migration

```bash
sudo systemctl enable --now feedbridge-api feedbridge-bot
sudo systemctl status feedbridge-api feedbridge-bot
journalctl -u feedbridge-api -u feedbridge-bot -f
```

Confirm gRPC is loopback-only: `ss -lntp | grep 50051` should show `127.0.0.1:50051`.

```bash
# optional: copy existing local subscriptions + detection cache
pg_dump -U deus -h localhost -d feedbridge -t subscriptions -t link_repositories | \
  ssh YOU@EXTERNAL_IP 'sudo -u postgres psql -d feedbridge'
```

### 5. Ops

- Backups: install `deploy/backup-db.sh` as root cron (`0 4 * * *`), keeps 7 dumps.
- Redeploy: same `make -C API deploy` / `make -C bot deploy` (units `Restart=always`).
- Watch RAM after the first feeds land: `free -h` and `ps aux --sort=-%mem`. If you sit near 1 GB, leave `WORKER_COUNT=3` or move to a paid `e2-small`.
- Never open `50051`, `5432`, or `6379` on the GCP firewall or UFW.

## Docker (optional, local/dev only)

Not for the free-tier VM. Local full stack:

```bash
docker compose -f docker-compose.example.yml up -d
```

Set `LISTEN_ADDR=:50051` in the API container so gRPC is reachable from the bot service (already in the example file). Both modules `replace` a sibling `rss_detector` checkout outside this repo, so image builds need that source in the build context. The systemd path does not.

