# AI log

## 2026-10-01 10:51
<!-- id: 5d6de2b7-e626-4c73-a758-2b4983e083c5 -->
- No project changes. Advised splitting domain (subscriptions, scheduler, fetch workers, cursor) into the API and keeping Telegram send/format in the bot.

## 2026-10-01 10:57
<!-- id: 03892c1b-0049-4a4f-b62c-b18f4ce46558 -->
- No project changes. Wrote a TDD execution plan to move subscriptions, scheduling, and fetch workers into the API.

## 2026-10-01 11:14
<!-- id: 16b85814-471f-4060-8be7-e3fe52e39800 -->
- `API/internal/testdb/` — in-memory SQLite helper for store tests.
- `API/store/` — destination-keyed subscriptions (`platform` + `external_id`), `NextCheckAt` due query, identity matching.
- `API/service/subscribe.go` — Subscribe/ResolveURL with injected detector.
- `API/proto/subscription.proto` — SubscriptionService including ListDue/AckCursor.
- `API/protoAPI/subscription/` — generated stubs.
- `API/handlers/subscription.go` — gRPC handler + bufconn tests.
- `API/handlers/detect.go` — ResolverDetector adapter.
- `API/main.go` — register SubscriptionService.
- `API/Makefile` — `make proto` target.
- `bot/grpcclient/client.go` — subscription RPC wrappers (handlers still use local store).

## 2026-10-01 11:45
<!-- id: d90fc497-41ec-45cf-ac0b-4f2a54ef401a -->
- `API/store/migrate.go` — backfill `platform`/`external_id` from legacy `chat_id`; unique dest index after fill.
- `API/store/store.go` — `MarkDue` so ListDue does not re-claim until the interval elapses.
- `API/handlers/subscription.go` — ListDue claims due rows via MarkDue.
- `bot/handlers/feed.go` — add/list/remove/enable/interval and callbacks use subscription gRPC.
- `bot/handlers/pipeline.go` — FetchTask acks cursor over gRPC.
- `bot/model/task.go` — scheduler pulls due rows from ListDue, no local store.
- `bot/main.go` — drop Postgres; bot is Telegram + gRPC only.
- `bot/handlers/feed_identity_test.go` — removed (identity lives in the API).
- `bot/grpcclient/client.go` — GetSubscription and IsNotFound.
