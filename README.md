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
