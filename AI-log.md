# AI log

## 2026-10-01 10:51
<!-- id: 5d6de2b7-e626-4c73-a758-2b4983e083c5 -->
- No project changes. Advised splitting domain (subscriptions, scheduler, fetch workers, cursor) into the API and keeping Telegram send/format in the bot.

## 2026-10-01 10:57
<!-- id: 03892c1b-0049-4a4f-b62c-b18f4ce46558 -->
- No project changes. Wrote a TDD execution plan to move subscriptions, scheduling, and fetch workers into the API.

## 2026-10-01 11:14
<!-- id: 16b85814-471f-4060-8be7-e3fe52e39800 -->
- `backend/internal/testdb/` — in-memory SQLite helper for store tests.
- `backend/store/` — destination-keyed subscriptions (`platform` + `external_id`), `NextCheckAt` due query, identity matching.
- `backend/service/subscribe.go` — Subscribe/ResolveURL with injected detector.
- `backend/proto/subscription.proto` — SubscriptionService including ListDue/AckCursor.
- `backend/protoAPI/subscription/` — generated stubs.
- `backend/handlers/subscription.go` — gRPC handler + bufconn tests.
- `backend/handlers/detect.go` — ResolverDetector adapter.
- `backend/main.go` — register SubscriptionService.
- `backend/Makefile` — `make proto` target.
- `telegram-bot/grpcclient/client.go` — subscription RPC wrappers (handlers still use local store).

## 2026-10-01 11:45
<!-- id: d90fc497-41ec-45cf-ac0b-4f2a54ef401a -->
- `backend/store/migrate.go` — backfill `platform`/`external_id` from legacy `chat_id`; unique dest index after fill.
- `backend/store/store.go` — `MarkDue` so ListDue does not re-claim until the interval elapses.
- `backend/handlers/subscription.go` — ListDue claims due rows via MarkDue.
- `telegram-bot/handlers/feed.go` — add/list/remove/enable/interval and callbacks use subscription gRPC.
- `telegram-bot/handlers/pipeline.go` — FetchTask acks cursor over gRPC.
- `telegram-bot/model/task.go` — scheduler pulls due rows from ListDue, no local store.
- `telegram-bot/main.go` — drop Postgres; bot is Telegram + gRPC only.
- `telegram-bot/handlers/feed_identity_test.go` — removed (identity lives in the API).
- `telegram-bot/grpcclient/client.go` — GetSubscription and IsNotFound.

## 2026-10-01 11:47
<!-- id: e9177c14-1338-4cc2-aa6b-1ce8250ea891 -->
- No project changes. Explained GCE SSH keys: local `~/.ssh/google_compute_engine`, metadata keys expired, how to refresh.

## 2026-10-01 11:51
<!-- id: 7a025655-b487-4c1e-9d16-02168cb6b6f1 -->
- No project changes. Explained forgotten SSH passphrase cannot be recovered; replace `google_compute_engine` with a new key via gcloud.

## 2026-10-01 12:12
<!-- id: current-session -->
- Model: deepseek/deepseek-v4-flash (via opencode)
- Built new linux/amd64 binaries for API and bot with the refactored subscription gRPC code.
- Deployed to GCE VM `feedbridge` (35.239.193.103): copied binaries, installed to `/opt/feedbridge/`, restarted systemd services.
- First transfer via `cat >` pipe corrupted the binaries (truncated), causing SIGSEGV. Re-transferred with `gcloud compute scp`.
- Both services now active and healthy. DB migration (AutoMigrate + BackfillDestinations) ran successfully on API startup.

## 2026-10-08 09:39
<!-- id: 88a3f621-916c-4bca-acc6-9f5850ccdb15 -->
- Model: clixad CLI
- Commit `3ee0249` — "Rename API and bot directories to backend and telegram-bot".
- Renamed `API/` → `backend/` and `bot/` → `telegram-bot/` across 83 files, updating module paths, import statements, Dockerfiles, Makefiles, deploy scripts, systemd units, env example files (`api.env.example` → `backend.env.example`, `bot.env.example` → `telegram-bot.env.example`), `docker-compose.example.yml`, `.gitignore`, `README.md`, and existing AI-log.md references.

## 2026-10-08 09:50
<!-- id: b35244b3-3ff2-4f3a-9206-c502247cb42d -->
- Model: Cline (claude-sonnet via VS Code extension)
- No project code changes. Reviewed the project to build context: architecture (telegram-bot ⇄ gRPC ⇄ backend → Postgres + Redis), deployment (GCP e2-micro, systemd), and current git state.
- Verified both modules build (`go build ./...`) and all tests pass (`go test ./...`) — backend and telegram-bot green.
- Added this AI-log entry for the clixad rename commit and this session.

## 2026-10-17 17:24
- `telegram-bot/handlers/backup.go` — `/backup` sends this chat's feeds as JSON; `/restore` subscribes to missing feeds and matches enabled flags.
- `telegram-bot/handlers/backup_test.go` — cover backup parse, restore planning, and the 10-minute wait.
- `telegram-bot/handlers/feed.go` — hold the in-memory restore wait on FeedHandler.
- `telegram-bot/main.go` — register `/backup`, `/restore`, and document uploads.
- `telegram-bot/handlers/entryConfig.go` — list the backup commands in `/start`.
- `README.md` — document `/backup` and `/restore`.

## 2026-10-17 17:45
- `README.md` — scheduler lives in the bot; document the running VM's `api.env`/`bot.env` and `WORKER_COUNT=5`; fix the bot binary copy path.

## 2026-10-17 17:47
- `docs/prd/` — moved both PRD PDFs out of the repo root and named them by draft date.
- `docs/proposals/` — moved the proposal `.docx` files out of the repo root.
- `docs/images/` — moved the logo, launch graphic, and RSS/Atom map out of the repo root.
- `README.md` — point at `docs/`.
