# habit-buddy

A realtime habit tracking application that demonstrates modern fullstack architecture with a custom Redis-compatible server.

## Architecture

```
Browser (React) ──HTTP/WS──► Go Backend ──SQL──► PostgreSQL
                                    │
                                    ├──RESP──► go-redis (caching, counters)
                                    │
                                    └──RESP pub/sub──► go-redis (realtime events)
                                            │
                                    ◄───────┘ (hb:events channel → WS hub → Browser)
```

**Key points:**
- Realtime updates via WebSocket — completing a habit on Tab A updates Tab B instantly
- Events flow through Redis pub/sub (`hb:events`), enabling horizontal scaling without sticky sessions
- `go-redis` (git submodule at `services/go-redis`) handles caching, streak counters, daily analytics, and realtime event fan-out
- PostgreSQL is the source of truth; go-redis caches hot data and brokers events

## Quick Start

```bash
# Clone with submodules — required, go-redis is a git submodule
git clone --recurse-submodules <repo-url>
cd habit-buddy

# If you already cloned without --recurse-submodules:
git submodule update --init --recursive

# Start all four services
docker compose up --build

# App available at:
# Frontend  → http://localhost:3000
# API       → http://localhost:8080
# go-redis  → localhost:6379
```

## Services

| Service  | Port | Description                          |
|----------|------|--------------------------------------|
| frontend | 3000 | React + Tailwind UI (nginx)          |
| backend  | 8080 | Go REST API + WebSocket hub          |
| postgres | 5432 | PostgreSQL 16 database               |
| go-redis | 6379 | Custom Redis-compatible server       |

## Redis Keyspace

```
hb:user:{userId}:habits          → cached habit list (JSON)
hb:habit:{habitId}:streak        → current streak counter (int)
hb:habit:{habitId}:last_date     → last completion date (YYYY-MM-DD)
hb:user:{userId}:daily:{date}    → daily completion count
hb:user:{userId}:total           → lifetime completion count
hb:events                        → pub/sub channel for realtime WS events
```

## API Endpoints

```
POST   /api/auth/register
POST   /api/auth/login

GET    /api/dashboard
GET    /api/habits
POST   /api/habits
PATCH  /api/habits/:id
DELETE /api/habits/:id
POST   /api/habits/:id/complete
DELETE /api/habits/:id/complete
GET    /api/habits/:id/stats
GET    /api/analytics

WS     /ws?token=<jwt>
```

## Realtime Event Flow

```
HTTP handler (CompleteHabit)
    │
    ├─ writes to PostgreSQL
    ├─ updates Redis streak key
    └─ bridge.Publish("hb:events", {userID, type, payload})
                │
                ▼
        go-redis PUBLISH
                │
                ▼
        EventBridge.run()  ← subscribed to "hb:events" on startup
                │
                ▼
        hub.BroadcastToUser(userID, event)
                │
                ▼
        WebSocket clients (all open tabs for that user)
```

## Development (without Docker)

```bash
# Start infrastructure only
docker compose up postgres go-redis -d

# Backend
cd apps/api
DATABASE_URL="postgres://habit:habit@localhost:5432/habitbuddy?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
go run ./cmd/server

# Frontend (requires Node 20+)
cd apps/web
npm install
npm run dev
```

## go-redis Submodule

`services/go-redis` is a Git submodule pointing to a custom Redis-compatible server.
It is built and run as a Docker service — no separate installation needed.

**Implemented commands (subset):**

| Category   | Commands |
|------------|----------|
| Strings    | `SET`, `GET`, `DEL`, `EXISTS`, `KEYS`, `MSET`, `MGET`, `SETNX`, `SETEX`, `PSETEX`, `GETSET`, `GETDEL`, `APPEND`, `STRLEN` |
| Counters   | `INCR`, `INCRBY`, `DECR`, `DECRBY` |
| Expiry     | `EXPIRE`, `PEXPIRE`, `TTL`, `PTTL`, `PERSIST` |
| Hashes     | `HSET`, `HMSET`, `HGET`, `HDEL`, `HGETALL`, `HMGET`, `HLEN`, `HEXISTS`, `HKEYS`, `HVALS`, `HINCRBY` |
| Pub/Sub    | `PUBLISH`, `SUBSCRIBE`, `UNSUBSCRIBE`, `PSUBSCRIBE`, `PUNSUBSCRIBE` |
| Connection | `PING`, `SELECT` |
| Admin      | `INFO`, `DBSIZE`, `TYPE`, `RENAME`, `FLUSHDB`, `FLUSHALL`, `COMMAND` |

The backend communicates with it using a raw TCP RESP client (`apps/api/internal/redisclient`) — no Redis library dependency.

Full go-redis documentation: [`services/go-redis/README.md`](services/go-redis/README.md)
