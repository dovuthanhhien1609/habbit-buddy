# habit-buddy

A realtime habit tracking application that demonstrates modern fullstack architecture with a custom Redis-compatible server.

## Architecture

```
Browser (React) ──HTTP/WS──► Go Backend ──SQL──► PostgreSQL
                                    │
                                    └──RESP──► go-redis (custom Redis clone)
```

**Key points:**
- Realtime updates via WebSocket — completing a habit on Tab A updates Tab B instantly
- `go-redis` (git submodule) handles caching, streak counters, and daily analytics
- PostgreSQL is the source of truth; go-redis caches hot data

## Quick Start

```bash
# Clone with submodules
git clone --recurse-submodules <repo-url>
cd habit-buddy

# Start everything
docker compose up --build

# App available at:
# Frontend  → http://localhost:3000
# API       → http://localhost:8080
# go-redis  → localhost:6379
```

## Services

| Service    | Port | Description |
|------------|------|-------------|
| frontend   | 3000 | React + Tailwind UI (nginx) |
| backend    | 8080 | Go REST API + WebSocket hub |
| postgres   | 5432 | PostgreSQL 16 database |
| go-redis   | 6379 | Custom Redis-compatible server |

## Redis Keyspace

```
hb:user:{userId}:habits          → cached habit list (JSON)
hb:habit:{habitId}:streak        → current streak counter (int)
hb:habit:{habitId}:last_date     → last completion date
hb:user:{userId}:daily:{date}    → daily completion count
hb:user:{userId}:total           → lifetime completion count
```

## API Endpoints

```
POST /api/auth/register
POST /api/auth/login

GET  /api/dashboard
GET  /api/habits
POST /api/habits
PATCH /api/habits/:id
DELETE /api/habits/:id
POST  /api/habits/:id/complete
DELETE /api/habits/:id/complete
GET   /api/habits/:id/stats
GET   /api/analytics

WS   /ws?token=<jwt>
```

## Development (without Docker)

```bash
# Start PostgreSQL + go-redis
docker compose up postgres go-redis -d

# Backend
cd apps/api
DATABASE_URL="postgres://habit:habit@localhost:5432/habitbuddy?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
go run ./cmd/server

# Frontend (requires Node 20)
cd apps/web
npm install
npm run dev
```

## go-redis Submodule

The `services/go-redis` directory is a Git submodule pointing to:
https://github.com/dovuthanhhien1609/go-redis

It is a custom Redis-compatible server written in Go that implements:
- RESP v2 protocol over TCP
- SET, GET, DEL, EXISTS, KEYS, PING commands
- Thread-safe in-memory store
- Append-only file (AOF) persistence

The backend communicates with it using a raw TCP RESP client (no Redis library dependency).
