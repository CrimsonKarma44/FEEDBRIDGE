# FeedBridge

Go service that detects RSS/Atom/JSON feeds for a URL and delivers new posts to Telegram chats. README.md has the full setup and deploy guide.

## Layout

Two Go modules, one repo:

- `backend/` — gRPC API (`127.0.0.1:50051` unless `LISTEN_ADDR` is set). Postgres via GORM, Redis cache. Owns all subscription state: `store/` (rows, migrations), `service/` (subscribe logic), `handlers/` (gRPC services + interceptors), `youtube/` (YouTube URL → feed), `utility/safehttp.go` (SSRF-safe HTTP client).
- `telegram-bot/` — Telebot v4 client. No database access; everything goes through `grpcclient/`. Runs the 30s scheduler and worker pool (`model/`): `ListDue` → fetch → dedupe → deliver → `AckCursor`.
- `browser-extension/` — WXT + TypeScript extension for Chrome and Firefox (MV3). Popup scans the active tab on click; talks to the API's HTTP gateway at `https://feedbridge.duckdns.org`. See its README.
- `deploy/` — systemd units, env examples, VM bootstrap and deploy scripts for one GCP e2-micro.

## Commands

Run from each module's directory:

```bash
go build ./... && go vet ./... && go test ./...   # backend/ and telegram-bot/
make -C backend proto                            # regenerate backend/protoAPI/ after editing backend/proto/* (needs protoc-gen-go, -go-grpc, -grpc-gateway)
make -C backend run                              # local API (needs backend/.env, Postgres, Redis)
cd telegram-bot && go run .                      # local bot (needs telegram-bot/.env)
```

In `browser-extension/`: `npm test`, `npm run check`, `npm run build` / `build:firefox`.

Tests use in-memory SQLite (`backend/internal/testdb`) and bufconn gRPC; they need no running services.

## Things that will bite

- Both `go.mod` files `replace` `github.com/CrimsonKarma44/rss_detector` with a local checkout at `/home/deus/Documents/code/project/rss_detector`. The bot also `replace`s the backend module with `../backend` and imports its generated stubs.
- Generated code in `backend/protoAPI/` is committed. Don't hand-edit it. `make proto` regenerates every file; older stubs then differ only in their `source:` header, so revert those and keep just the files you meant to change.
- `backend/gateway/` serves the extension's HTTP/JSON API (grpc-gateway over the local gRPC server, routes in `proto/extension_http.yaml`, generated into `protoAPI/extensiongw/` so the bot never imports grpc-gateway). It starts only when `HTTP_LISTEN_ADDR` is set.
- Subscriptions are keyed on the resolved feed URL plus `platform` + `external_id`, not on the URL the user typed.
- Telegram allows one long-poller per bot token. A local `go run` and the VM bot can't run at the same time (`409 Conflict`).
- Deploy by cross-compiling on the laptop (`make -C <module> build-linux` or `deploy/deploy-*.sh`). Never build on the VM, and never run Docker Compose there (1 GB RAM).
- Compiled binaries (`*-linux-amd64`) and `.env` files are gitignored. Never commit them.
- The API must stay bound to loopback on the VM; never open 50051, 5432 or 6379.

## Conventions

- Log every session that changes project files in `AI-log.md` using the `ai-log` skill (`.claude/skills/ai-log/`). Sessions that change nothing get no entry.
- Keep README.md in step with behavior changes (commands, env vars, deploy steps).
