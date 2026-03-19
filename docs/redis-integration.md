# Redis Integration

## Overview

`services/go-redis` is a custom Redis-compatible server written in Go (git submodule).
It is the only Redis dependency — no external Redis installation is required.

Module: `github.com/hiendvt/go-redis`

The backend uses go-redis for three things:
1. **Caching** — habit lists are cached as JSON to avoid repeated DB queries
2. **Counters** — streak values and daily completion counts stored as integers
3. **Realtime events** — pub/sub on `hb:events` fans out WebSocket notifications across API instances

---

## Supported Commands

| Category   | Commands |
|------------|----------|
| Strings    | `SET`, `GET`, `DEL`, `EXISTS`, `KEYS`, `MSET`, `MGET`, `SETNX`, `SETEX`, `PSETEX`, `GETSET`, `GETDEL`, `APPEND`, `STRLEN` |
| Counters   | `INCR`, `INCRBY`, `DECR`, `DECRBY` |
| Expiry     | `EXPIRE`, `PEXPIRE`, `TTL`, `PTTL`, `PERSIST` |
| Hashes     | `HSET`, `HMSET`, `HGET`, `HDEL`, `HGETALL`, `HMGET`, `HLEN`, `HEXISTS`, `HKEYS`, `HVALS`, `HINCRBY` |
| Pub/Sub    | `PUBLISH`, `SUBSCRIBE`, `UNSUBSCRIBE`, `PSUBSCRIBE`, `PUNSUBSCRIBE` |
| Connection | `PING`, `SELECT` |
| Admin      | `INFO`, `DBSIZE`, `TYPE`, `RENAME`, `FLUSHDB`, `FLUSHALL`, `COMMAND` |

---

## Backend RESP Client

`apps/api/internal/redisclient/client.go` speaks RESP directly over TCP — no Redis library is used.

```go
client, _ := redisclient.NewClient("localhost:6379")

// String operations
client.Set("hb:habit:abc:streak", "7")
val, found, _ := client.Get("hb:habit:abc:streak")
client.Del("hb:user:xyz:habits")

// Native counter — uses INCR command
n, _ := client.Incr("hb:user:xyz:total")   // → 1, 2, 3, …

// Publish a realtime event (envelope JSON — see Event Schema below)
n, _ := client.Publish("hb:events", `{"user_id":"xyz","event":{...}}`)
```

For pub/sub subscriptions a **dedicated connection** is required (a subscribed connection cannot issue regular commands):

```go
sub, _ := redisclient.NewSubscriber("localhost:6379")
sub.Subscribe("hb:events")

for msg := range sub.Messages() {
    fmt.Println(msg.Channel, msg.Payload)
}
```

---

## Key Naming Convention

All keys are prefixed with `hb:` to namespace the application.

```
hb:user:{userId}:habits          → JSON array of Habit objects (cache)
hb:habit:{habitId}:streak        → integer string, current streak
hb:habit:{habitId}:last_date     → "YYYY-MM-DD" of last completion
hb:user:{userId}:daily:{date}    → integer count for that date
hb:user:{userId}:total           → lifetime completions integer
hb:events                        → pub/sub channel for WebSocket events
```

---

## Cache Invalidation Strategy

go-redis supports `EXPIRE` / `TTL`, but the habit-buddy cache uses manual invalidation for simplicity — a DEL on write, rebuild on the next read.

Invalidation points:
- `CreateHabit`, `UpdateHabit`, `ArchiveHabit` → `DEL hb:user:{id}:habits`
- `CompleteHabit`, `UndoCompletion` → `DEL hb:user:{id}:habits`

---

## Event Schema

All events published to Redis follow a canonical envelope defined in `internal/model/event.go`.

```go
type Event struct {
    EventID   string          `json:"event_id"`   // UUID v4
    EventType string          `json:"event_type"` // e.g. "habit.completed"
    Timestamp time.Time       `json:"timestamp"`  // UTC
    Producer  string          `json:"producer"`   // "api"
    Payload   json.RawMessage `json:"payload"`    // event-specific data
}
```

### Event types

| Constant | Value | Trigger |
|----------|-------|---------|
| `EventHabitCompleted` | `habit.completed` | `POST /api/habits/:id/complete` |
| `EventHabitUndone`    | `habit.undone`    | `DELETE /api/habits/:id/complete` |
| `EventHabitCreated`   | `habit.created`   | `POST /api/habits` |
| `EventHabitUpdated`   | `habit.updated`   | `PATCH /api/habits/:id` |
| `EventHabitArchived`  | `habit.archived`  | `DELETE /api/habits/:id` |

### Redis wire message (`hb:events` payload)

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "event": {
    "event_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "event_type": "habit.completed",
    "timestamp":  "2026-03-19T09:15:30Z",
    "producer":   "api",
    "payload": {
      "habitId":     "habit-xyz-789",
      "habitName":   "Morning Run",
      "streak":      7,
      "completedAt": "2026-03-19T09:15:30Z"
    }
  }
}
```

### WebSocket message delivered to clients

The `WSBroadcastHandler` converts the `Event` into a `WSEvent`. Clients receive:

```json
{
  "event_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "type":       "habit.completed",
  "timestamp":  "2026-03-19T09:15:30Z",
  "producer":   "api",
  "payload": {
    "habitId":     "habit-xyz-789",
    "habitName":   "Morning Run",
    "streak":      7,
    "completedAt": "2026-03-19T09:15:30Z"
  }
}
```

---

## Idempotency & Deduplication

`EventBridge` maintains an in-memory ring buffer (`seenCache`, capacity 1000) of recently processed `event_id` values. If the same event is delivered more than once — e.g. due to a Redis reconnect or a replayed message — it is silently dropped **before** reaching the router.

```
event received → seenCache.seen(event_id)
                      ├─ true  → drop, log "duplicate event dropped"
                      └─ false → record ID, forward to Router
```

The ring buffer evicts the oldest entry when full, so memory usage is bounded and requires no external TTL management.

---

## Realtime Architecture

### Layer diagram

```mermaid
flowchart TB
    subgraph Transport
        Handler["HTTP Handler\nbuild Event{event_id, …}"]
        Publish["redisclient.Publish\nhb:events"]
        Broker["go-redis\nRESP pub/sub broker"]
        Bridge["EventBridge\nsubscribe + dedup"]
    end

    subgraph Routing["Event Routing (internal/events)"]
        Router["Router\nmap[event_type] → []Handler\n+ exponential backoff retry"]
        WSHandler["WSBroadcastHandler\nconverts Event → WSEvent"]
    end

    subgraph Hub["WS Hub (internal/ws)"]
        Channels["channel map\nuser:{id} · habit:{id} · team:{id}"]
        Clients["WebSocket Clients"]
    end

    Handler -->|"bridge.Publish(userID, Event)"| Publish
    Publish --> Broker
    Broker -->|"hb:events push"| Bridge
    Bridge -->|"drop if duplicate"| Bridge
    Bridge -->|"Router.Route(EventContext)"| Router
    Router -->|"Handler.Handle (+ retry)"| WSHandler
    WSHandler -->|"BroadcastToChannel(user:{id})"| Channels
    WSHandler -->|"BroadcastToChannel(habit:{id})"| Channels
    Channels --> Clients
```

### Separation of concerns

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Transport | `internal/ws/bridge.go` | Redis subscribe/publish, envelope decode, deduplication |
| Routing | `internal/events/router.go` | Dispatch by event_type, retry on failure |
| Business logic | `internal/events/handlers.go` | Decide target channels, build WSEvent |
| Fan-out | `internal/ws/hub.go` | Channel subscriptions, write to WS connections |

### Key files

| File | Role |
|------|------|
| `internal/model/event.go` | `Event` struct + event type constants |
| `internal/model/types.go` | `WSEvent` — wire format sent to WS clients |
| `internal/redisclient/client.go` | `Publish(channel, message)` |
| `internal/redisclient/subscriber.go` | Dedicated pub/sub TCP connection |
| `internal/ws/bridge.go` | Transport layer — Redis → Router |
| `internal/events/router.go` | `Router` + `RetryPolicy` |
| `internal/events/handlers.go` | `WSBroadcastHandler`, `Broadcaster` interface |
| `internal/ws/hub.go` | Channel-based WS fan-out |
| `internal/api/handlers.go` | Builds `Event`, calls `bridge.Publish` |

---

## WebSocket Channel Subscriptions

The Hub uses named channels instead of broadcasting to all clients.

### Channel patterns

| Channel | Managed by | Purpose |
|---------|-----------|---------|
| `user:{id}` | Server (auto on connect) | All events for a user |
| `habit:{id}` | Client (opt-in) | Events scoped to one habit |
| `team:{id}` | Client (opt-in) | Reserved for team-level fan-out |

### Client subscription protocol

After connecting, a client is automatically subscribed to `user:{their-id}`. To opt into habit-scoped updates, send a JSON control message over the WebSocket:

```json
{ "action": "subscribe",   "channel": "habit:abc123" }
{ "action": "unsubscribe", "channel": "habit:abc123" }
```

Clients cannot subscribe to `user:{id}` channels — those are server-managed only.

---

## Retry Mechanism

`Router.withRetry` wraps every handler call with exponential backoff:

```
attempt 1 → error → wait 100 ms
attempt 2 → error → wait 200 ms
attempt 3 → error → log "handler failed after all retries"
```

The policy is configurable:

```go
events.NewRouter(events.RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond})
```

---

## Structured Logging

All components use `log/slog` JSON output (initialized in `internal/logger/logger.go`).
Every stage of the event lifecycle emits a structured log line:

```json
{"level":"INFO","msg":"publishing event",                    "component":"habit_handler",       "event_id":"…","event_type":"habit.completed","habit_id":"…","user_id":"…"}
{"level":"INFO","msg":"event published to Redis",            "component":"event_bridge",        "event_id":"…","event_type":"habit.completed","user_id":"…","producer":"api"}
{"level":"INFO","msg":"event received from Redis",           "component":"event_bridge",        "event_id":"…","event_type":"habit.completed","user_id":"…","producer":"api"}
{"level":"INFO","msg":"routing event",                       "component":"event_router",        "event_id":"…","event_type":"habit.completed","user_id":"…","handler_count":1}
{"level":"INFO","msg":"broadcast to user channel",           "component":"ws_broadcast_handler","event_id":"…","channel":"user:…"}
{"level":"INFO","msg":"broadcast to habit channel",          "component":"ws_broadcast_handler","event_id":"…","channel":"habit:…"}
```

Retry warnings:
```json
{"level":"WARN","msg":"handler error, will retry","component":"event_router","event_id":"…","attempt":1,"max_attempts":3,"retry_in":"100ms","error":"…"}
```

Duplicate events:
```json
{"level":"INFO","msg":"duplicate event dropped","component":"event_bridge","event_id":"…","event_type":"habit.completed"}
```
