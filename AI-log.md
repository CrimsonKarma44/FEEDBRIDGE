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
