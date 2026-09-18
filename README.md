# FEEDBRIDGE

Intelligent RSS-to-Chat Delivery Platform

FEEDBRIDGE detects syndication feeds (RSS / Atom / JSON Feed) for any site URL, caches them, and delivers new posts as Telegram messages to private chats, groups, and communities on a schedule.

## Architecture

```
Telegram  <-->  bot/  --gRPC-->  API/  -->  Postgres + Redis
                  |
                  +-- scheduler (30s tick) -> worker pool (WORKER_COUNT) -> dedupe -> delivery
```

| Component | What it does |
|---|---|
| `API/` | Go gRPC server on `LISTEN_ADDR` (default `127.0.0.1:50051`). Feed detection via [rss_detector](https://github.com/CrimsonKarma44/rss_detector), link registry in Postgres (GORM), Redis caching with TTLs (feed items 10 min, feed links 24 h, negative results 5 min), singleflight coalescing of identical requests, panic-recovery + logging interceptors, gRPC health checks & reflection |
| `bot/` | Telebot v4 client. Per-chat subscriptions stored in the same Postgres, admin-only management inside groups, scheduled polling, hybrid delivery: up to 5 new items as individual messages, larger batches collapsed into one digest. Worker pool size is `WORKER_COUNT` (default 10; use 3 on the 1 GB e2-micro). |

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
| `LISTEN_ADDR` | API | gRPC bind address (default `127.0.0.1:50051`) |
| `API_ADDR` | bot | gRPC endpoint of the API (default `localhost:50051`) |
| `TELEGRAM_BOT_TOKEN` | bot | Bot token from BotFather |
| `WORKER_COUNT` | bot | Fetch worker pool size (default 10) |
| `YOUTUBE_API_KEY` | API | Optional; prefers Data API v3 in the YouTube resolver |
| `FEEDBRIDGE_ALLOW_PRIVATE_FETCH` | API | Set to `1` to allow fetching LAN/loopback feed URLs (off by default) |

> Note: `.env` files are gitignored — never commit them.

### 2. Run the API

```bash
cd API
make run          # builds ./feedbridge then executes it
```

The server listens on `127.0.0.1:50051` unless `LISTEN_ADDR` is set. Schema is auto-migrated on startup. `.env` is optional when the same variables are already in the process environment (systemd `EnvironmentFile`).

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

Do **not** run Docker Compose on this machine (1 GB RAM). Do **not** compile on the VM (`rss_detector` is a local `replace` on your laptop). Cross-compile `linux/amd64` at home and copy the binaries.

Telegram allows only **one** long-poller per bot token. Stop the laptop `go run` before starting the cloud bot, or you will see `409 Conflict: terminated by other getUpdates`.

### 1. Account & VM

Always Free `e2-micro` is **one instance**, only in `us-central1`, `us-west1`, or `us-east1`. A billing account is still required.

```bash
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
gcloud services enable compute.googleapis.com
./deploy/gcp/create-vm.sh          # us-central1-a, Ubuntu 24.04 amd64, SSH-only firewall
```

Console equivalent: Compute Engine → Create instance — name `feedbridge`, zone `us-central1-a`, machine **e2-micro**, Ubuntu 24.04 LTS **amd64**, 20–30 GB standard disk, **HTTP/HTTPS off**.

### 2. Copy `deploy/` onto the VM

`gcloud compute scp` logs in as **your laptop username**. Google Console SSH often logs in as a **different** user. `~/deploy` is per-user; copy into the account you will actually use, or you will get `cd: ~/deploy: No such file or directory`.

From the **laptop**, repo root:

```bash
gcloud compute scp --recurse deploy feedbridge:~/deploy --zone=us-central1-a
```

If Console SSH is a different user (check `whoami` on the VM), copy into that home:

```bash
gcloud compute ssh feedbridge --zone=us-central1-a --command \
  'sudo cp -a ~/deploy /home/CONSOLE_USER/deploy && sudo chown -R CONSOLE_USER:CONSOLE_USER /home/CONSOLE_USER/deploy'
```

### 3. Bootstrap (on the VM)

In the SSH session where `ls ~/deploy` works:

```bash
cd ~/deploy
sudo ./setup-server.sh
```

The script is idempotent. It installs Postgres + Redis, creates system user `feedbridge`, applies 1 GB memory limits, enables UFW (SSH only), and prints `REDIS_PASSWORD`. UFW will ask whether to proceed; the script answers yes.

If the password banner scrolled past, recover it with:

```bash
sudo grep '^requirepass ' /etc/redis/redis.conf
```

Then create the two env files (these are the only files you author on the VM):

```bash
sudo cp ~/deploy/env/api.env.example /etc/feedbridge/api.env
sudo cp ~/deploy/env/bot.env.example /etc/feedbridge/bot.env
sudo chmod 600 /etc/feedbridge/*.env
sudoedit /etc/feedbridge/api.env /etc/feedbridge/bot.env
```

| File | Set these | Leave as in the example |
|---|---|---|
| `/etc/feedbridge/api.env` | `DB_PASSWORD` (the Postgres password you typed), `REDIS_PASSWORD` (from `requirepass`), optional `YOUTUBE_API_KEY` | `LISTEN_ADDR=127.0.0.1:50051`, loopback hosts |
| `/etc/feedbridge/bot.env` | `TELEGRAM_BOT_TOKEN`, same `DB_PASSWORD` | `API_ADDR=127.0.0.1:50051`, `WORKER_COUNT=3` |

Do **not** reuse the laptop Redis/Postgres passwords; the VM generated new ones.

```bash
sudo cp ~/deploy/feedbridge-*.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### 4. Ship binaries (from your laptop)

`make deploy` uses plain `ssh`/`scp`. `gcloud compute scp` is more reliable with OS Login keys:

```bash
# repo root
make -C API build-linux
make -C bot build-linux
gcloud compute scp API/feedbridge-linux-amd64 bot/feedbridge-bot-linux-amd64 \
  feedbridge:/tmp/ --zone=us-central1-a
gcloud compute ssh feedbridge --zone=us-central1-a --command \
  'sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-linux-amd64 /opt/feedbridge/feedbridge &&
   sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-bot-linux-amd64 /opt/feedbridge/feedbridge-bot'
```

Or, if your SSH user@IP works with the same keys: `make -C API deploy HOST=YOU@EXTERNAL_IP` and the same for `bot`.

### 5. Start

On the VM:

```bash
sudo systemctl enable --now feedbridge-api feedbridge-bot
sudo systemctl is-active feedbridge-api feedbridge-bot    # both should print: active
sudo journalctl -u feedbridge-api -u feedbridge-bot -n 30 --no-pager
ss -lntp | grep 50051    # must be 127.0.0.1:50051, not 0.0.0.0
```

Healthy logs: API `listening on 127.0.0.1:50051`, bot `Bot started` and `worker pool size 3`.

### 6. Migrate local subscriptions (optional)

Redis is a cache — do not copy it. Copy only Postgres `subscriptions` and `link_repositories`.

Local Postgres 17 `pg_dump` emits `\restrict`, `\unrestrict`, and `SET transaction_timeout`, which Ubuntu 24.04 Postgres **16** rejects. Strip those, restore data-only into the already-migrated schema, then restart:

```bash
# laptop
pg_dump -d feedbridge -t subscriptions -t link_repositories \
  --data-only --no-owner --no-acl -f /tmp/feedbridge-migrate.sql
sed -e '/^\\restrict /d' -e '/^\\unrestrict /d' -e '/^SET transaction_timeout /d' \
  /tmp/feedbridge-migrate.sql > /tmp/feedbridge-migrate-pg16.sql
gcloud compute scp /tmp/feedbridge-migrate-pg16.sql feedbridge:/tmp/feedbridge-migrate.sql \
  --zone=us-central1-a

# VM
sudo -u postgres psql -d feedbridge -v ON_ERROR_STOP=1 \
  -c "TRUNCATE TABLE public.subscriptions, public.link_repositories RESTART IDENTITY CASCADE;" \
  -f /tmp/feedbridge-migrate.sql
sudo -u postgres psql -d feedbridge \
  -c "SELECT count(*) FROM public.subscriptions;" \
  -c "SELECT count(*) FROM public.link_repositories;"
sudo systemctl restart feedbridge-api feedbridge-bot
```

Qualify tables as `public.…` after the dump: it sets `search_path` empty. Skipping this step leaves an empty bot; `/addfeed` still works.

### 7. Ops

- Cut over: stop the laptop bot, then `/listfeed` in Telegram.
- Backups: `sudo cp ~/deploy/backup-db.sh /opt/feedbridge/backup-db.sh` and root cron `0 4 * * * /opt/feedbridge/backup-db.sh >> /var/log/feedbridge-backup.log 2>&1` (keeps 7 dumps).
- Redeploy binaries: repeat step 4, then `sudo systemctl restart feedbridge-api feedbridge-bot`.
- RAM: `free -h`. Stay at `WORKER_COUNT=3` on the micro; move to a paid `e2-small` if you sit near 1 GB.
- Never open `50051`, `5432`, or `6379` on the GCP firewall or UFW.

## Docker (optional, local/dev only)

Not for the free-tier VM. Local full stack:

```bash
docker compose -f docker-compose.example.yml up -d
```

Set `LISTEN_ADDR=:50051` in the API container so gRPC is reachable from the bot service (already in the example file). Both modules `replace` a sibling `rss_detector` checkout outside this repo, so image builds need that source in the build context. The systemd path does not.

