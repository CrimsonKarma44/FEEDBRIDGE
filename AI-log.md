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

## 2026-10-31 10:43
<!-- id: 566d8d40-8283-415e-a89c-c8c78a7c02b4 -->
- `.claude/skills/ai-log/SKILL.md` — `/ai-log` skill that appends or updates this session's entry in `AI-log.md`, following the format of past entries; writes an entry only when the session changed project files.
- `backend/feedbridge-linux-amd64`, `telegram-bot/feedbridge-bot-linux-amd64` — untracked (gitignored; deploy scripts rebuild them). Files kept on disk.
- `telegram-bot/store/store.go` — removed (unused since subscriptions moved to gRPC); `go mod tidy` dropped gorm and the Postgres driver from `telegram-bot/go.mod`.
- `backend/main.go` — removed the commented-out rssdetector demo and the hardcoded `Server running on :50051` log line.
- `README.md` — drop mentions of the deleted bot store.
- `backend/config/env.go`, `telegram-bot/model/ENV.go`, `deploy/setup-server.sh` — `api.env`/`bot.env` → `backend.env`/`telegram-bot.env`.
- `CLAUDE.md` — layout, commands, deploy cautions, and the AI-log rule.
- Commits `357b54b`, `7abb9c2`, `95e658c`, `371f29e`.
- Verified both modules build, vet, and pass tests (`go build ./... && go vet ./... && go test ./...`).
- `README.md` — replace the VM env-name caveat with the VM's current state.
- VM `feedbridge`: renamed `/etc/feedbridge/api.env` → `backend.env` and `bot.env` → `telegram-bot.env`, installed `deploy/feedbridge-*.service`, restarted both services. Both active; API on `127.0.0.1:50051`, bot `worker pool size 5`.
- `docs/plans/browser-extension.md` — new browser-extension plan replacing `.junie/plans/browser-extension-feedbridge.md`: click-only detection, grpc-gateway HTTP layer over the gRPC core, Redis link tokens for Telegram deep links, detection cached in Redis only.
- `deploy/backup-db.sh` — nightly cleanup after the dump: hard-delete `link_repositories` rows older than 30 days that no subscription uses (by `url` or a `feed_links` key).
- `README.md` — note the cleanup in the backups line.
- `backend/proto/extension.proto`, `backend/protoAPI/extension/` — `ExtensionService` (`Detect`, `CreateLinkToken`, `ResolveLinkToken`) and generated stubs; `backend/Makefile` builds it with `make proto`.
- `backend/handlers/extension.go` — `ExtensionHandler` and `CachedFinder` (Redis cache and negative cache only, no Postgres writes); tests in `extension_test.go`.
- `backend/handlers/detect.go`, `setUrl.go`, `cache.go` — `detectCached` shared by `SetUrl` and `Detect` with its own singleflight key; cache entries keep feed `title` and `source`; package-level negative-cache helpers.
- `backend/service/linktoken.go` — `LinkTokens`: 12-char single-use tokens in Redis, 15-minute TTL; tests in `linktoken_test.go`, also checked against a throwaway real Redis.
- `backend/config/redis_db.go` — `GetDel`.
- `backend/main.go` — register `ExtensionService`.
- `docs/plans/browser-extension.md` — step 1 marked done.
- `backend/gateway/` — grpc-gateway HTTP/JSON front for `ExtensionService` (`GET /v1/detect`, `POST /v1/link-tokens`, `GET /healthz`) with per-IP rate limit (30/min, burst 10), 4 KB body cap, and `X-Forwarded-For` trusted only from loopback; tests in `gateway_test.go`.
- `backend/proto/extension_http.yaml` — HTTP routes for the gateway, keeping `extension.proto` free of annotations; `ResolveLinkToken` has no route. `backend/Makefile` generates `protoAPI/extension/extension.pb.gw.go`.
- `backend/main.go` — start the gateway when `HTTP_LISTEN_ADDR` is set; shut it down before the gRPC server.
- `backend/go.mod`, `telegram-bot/go.mod` — add grpc-gateway v2.32.0 and `golang.org/x/time` v0.16.0; grpc v1.84.0, protobuf v1.36.12 pulled in by the gateway.
- `deploy/env/backend.env.example`, `backend/.env.example`, `README.md`, `CLAUDE.md` — document `HTTP_LISTEN_ADDR`, the HTTP routes, and the codegen plugins.
- Smoke-tested HTTP → gateway → `ExtensionHandler` → throwaway Redis with live detection of go.dev/blog (1.2 s first, 3 ms cached, title kept).
- `backend/protoAPI/extensiongw/` — gateway stubs moved out of `protoAPI/extension/` (grpc-gateway `standalone` mode) so the bot can import the gRPC stubs without pulling in grpc-gateway; `backend/Makefile` and `backend/gateway/gateway.go` updated.
- `telegram-bot/handlers/linkstart.go` — `/start add_<token>` from the extension: peeks the token, asks the chat to confirm with Subscribe/Cancel buttons (`lk_add`, `lk_no`, admins only in groups), consumes the token on press, then subscribes; tests in `linkstart_test.go`.
- `telegram-bot/handlers/feed.go` — `subscribeURL` extracted from `AddFeed` and shared with the deep-link flow.
- `telegram-bot/grpcclient/client.go` — `ResolveLinkToken` and `IsTokenGone`.
- `telegram-bot/main.go` — `/start` goes through `StartWithLink`; register the two buttons.
- `README.md`, `CLAUDE.md`, `docs/plans/browser-extension.md` — document the deep link and the gateway package; step 3 marked done.
- `deploy/Caddyfile` — Caddy site for `feedbridge.duckdns.org` proxying to `127.0.0.1:8080`.
- `deploy/duckdns-update.sh`, `deploy/env/duckdns.env.example` — DuckDNS IP updater; token read from `/etc/feedbridge/duckdns.env` and passed to curl on stdin.
- `deploy/feedbridge.cron` — `/etc/cron.d/feedbridge`: nightly `backup-db.sh` at 04:00 and the DuckDNS updater every 5 minutes.
- `deploy/setup-server.sh`, `deploy/gcp/create-vm.sh` — install Caddy, UFW 80/443, the scripts and cron; `feedbridge-web` GCP rule for tcp:80,443.
- `README.md` — "HTTPS for the browser extension" deploy section; backups now installed by `setup-server.sh`.
- VM `feedbridge`: deployed new API and bot binaries (checksums verified, old ones kept as `*.prev`), added `HTTP_LISTEN_ADDR=127.0.0.1:8080` to `backend.env`, installed Caddy 2.6.2 with `deploy/Caddyfile`, installed `backup-db.sh`, `duckdns-update.sh` and `/etc/cron.d/feedbridge`. Backups had never been scheduled before. First run wrote `/var/backups/feedbridge/feedbridge_2026-10-31_1238.sql.gz` and deleted 71 of 170 `link_repositories` rows.
- VM `feedbridge` and GCP: created firewall rule `feedbridge-web` (tcp:80,443 to tag `feedbridge`), UFW allow 80/443, DuckDNS token in `/etc/feedbridge/duckdns.env` (600) and first update OK. Caddy got a Let's Encrypt certificate for `feedbridge.duckdns.org`. Checked from outside: `/healthz`, `/v1/detect` (YouTube, go.dev), `/v1/link-tokens`, 400 on bad URL, 404 for token lookup, 429 after the burst; 8080, 50051, 5432, 6379 closed.
- `deploy/Caddyfile` — `acme_ca` set to Let's Encrypt; Ubuntu's Caddy 2.6.2 ZeroSSL fallback fails with `caddy_legacy_user_removed`.
- `backend/handlers/detect.go` — YouTube results carry `source` `platform:youtube`; redeployed the API.
- `docs/plans/browser-extension.md` — step 4 marked done.
- `browser-extension/` — new WXT 0.21.4 + TypeScript extension (Chrome and Firefox MV3): popup scans the active tab's `<link>` tags on click (`utils/scan.ts`), calls `/v1/detect` on YouTube/GitHub or on "Scan with FeedBridge", groups feeds by type, Copy/Open/Subscribe with `t.me/<bot>?start=add_<token>` or `?startgroup=`, `/addfeed` fallback; options page for server and bot; background only clears the per-tab badge. Icons cut from `docs/images/feedbridge-logo.png`. Firefox declares `data_collection_permissions: browsingActivity`, minimum 142.
- `browser-extension/tests/` — 24 Vitest tests (scan in jsdom, API client, links, settings). Also checked in Playwright's Chromium against a local test page and the live API (10/10, light/dark screenshots); `web-ext lint` on the Firefox build: 0 errors, 0 warnings.
- `README.md`, `CLAUDE.md`, `browser-extension/README.md`, `docs/plans/browser-extension.md` — document the extension; step 5 marked done.
- `browser-extension/wxt.config.ts` — `build.modulePreload: false`: Vite's `<link rel="modulepreload" crossorigin>` for the chunk shared by popup and options made Chrome log "cross-world extension resource mismatch" and "preloaded but not used" warnings. Browser check now also fails on console warnings from extension pages.
- `README.md` — bring the overview up to date: intro mentions the extension, architecture diagram shows extension → Caddy → HTTP gateway → gRPC, `docs/plans/` listed, Node 22+ prerequisite, deployment no longer says "nothing is public except SSH" (22/80/443 open), bootstrap and env tables cover Caddy, cron, `HTTP_LISTEN_ADDR` and `duckdns.env`.
- `telegram-bot/handlers/check.go` — `/check <url>`, and bare links in private chats: `NormalizeURL`, then the API's `Detect`; replies with the feeds (title, type, URL, up to 6) and a Subscribe button per feed carrying a single-use link token (`ck_add`, admins only in groups), or "No feeds found". Shows "Checking…" and edits it with the result. Tests in `check_test.go`.
- `telegram-bot/grpcclient/client.go` — `Detect` and `CreateLinkToken`.
- `telegram-bot/main.go`, `telegram-bot/handlers/entryConfig.go` — register `/check`, `OnText` and the button; `/start` texts mention `/check` and sending a link.
- `README.md` — `/check` in the bot commands.
