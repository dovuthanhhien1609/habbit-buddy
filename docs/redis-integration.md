# Redis Integration Notes

## go-redis Overview

`services/go-redis` is a custom Redis-compatible server written in Go.

Module: `github.com/hiendvt/go-redis`

### Supported commands
- `PING [message]` — connectivity check
- `SET key value` — store key-value pair (persisted to AOF)
- `GET key` — retrieve value (nil if not found)
- `DEL key [key ...]` — delete keys (persisted to AOF)
- `EXISTS key [key ...]` — check key existence
- `KEYS pattern` — list keys matching glob pattern
- `COMMAND` — introspection

### Not yet supported
- `INCR / INCRBY` — emulated in the backend RESP client via GET+SET
- `EXPIRE / TTL` — not available; cache invalidation is manual DEL
- `SUBSCRIBE / PUBLISH` — planned; replaced by in-process Go channel bus

## Backend RESP Client

`apps/api/internal/redisclient/client.go` speaks RESP directly over TCP.

No Redis library is used. The client:
1. Opens a TCP connection to `REDIS_ADDR` (default `localhost:6379`)
2. Serialises commands as RESP arrays (`*N\r\n$len\r\narg\r\n...`)
3. Parses responses (simple string, error, integer, bulk string, array)
4. Uses a `sync.Mutex` to serialise commands on a single connection

```go
client, _ := redisclient.NewClient("localhost:6379")
client.Set("hb:habit:abc:streak", "7")
val, found, _ := client.Get("hb:habit:abc:streak")
client.Del("hb:user:xyz:habits")
n, _ := client.Incr("hb:user:xyz:total") // emulated with GET+SET
```

## Key Naming Convention

All keys are prefixed with `hb:` to namespace the application.

```
hb:user:{userId}:habits          → JSON array of Habit objects (cache)
hb:habit:{habitId}:streak        → integer string, current streak
hb:habit:{habitId}:last_date     → "YYYY-MM-DD" of last completion
hb:user:{userId}:daily:{date}    → integer count for that date
hb:user:{userId}:total           → lifetime completions integer
```

## Cache Invalidation Strategy

Since go-redis doesn't support TTL yet, invalidation is manual:
- On `CreateHabit`, `UpdateHabit`, `ArchiveHabit` → DEL `hb:user:{id}:habits`
- On `CompleteHabit`, `UndoCompletion` → DEL `hb:user:{id}:habits`

The cache is rebuilt on the next `GetDashboard` or `GetHabits` call.

## Pub/Sub Migration Path

When go-redis adds SUBSCRIBE/PUBLISH:
1. Replace the in-process Go channel bus in `internal/ws/hub.go` with a Redis subscriber
2. The WS hub opens a separate TCP connection and issues `SUBSCRIBE habit:completed:{userId}`
3. The API handler issues `PUBLISH habit:completed:{userId} <json-payload>` after each completion
4. No frontend or API interface changes needed
