# Demo Scenario

## Setup

```bash
git clone --recurse-submodules <repo-url>
cd habit-buddy
docker compose up --build
# Navigate to http://localhost:3000
```

## Script

### 1. Register + Login (30s)
- Open `http://localhost:3000`
- Register a new account: `demo@example.com` / `demouser` / `password123`
- Get redirected to the dashboard

### 2. Create habits (60s)
Create three habits:
1. "Morning Run" — 🏃 icon — green color
2. "Read 20 pages" — 📚 icon — purple color
3. "Drink 8 glasses" — 💧 icon — blue color

### 3. Realtime demo (90s — the money shot)
- Open a **second browser tab** at `http://localhost:3000`
- Place both tabs side-by-side

**On Tab 1:**
- Click "Mark done" on "Morning Run"

**Observe on Tab 2 (no interaction):**
- The "Morning Run" card's button animates to a green checkmark
- Streak badge appears: "🔥 1 day"
- Toast notification slides in: "Morning Run completed! 🔥 1 day streak"

### 4. Show go-redis in action (30s)
In a terminal:
```bash
# Check the streak counter stored in go-redis
redis-cli -p 6379 GET hb:habit:<habitId>:streak
# Returns: "1"

# Check the daily counter
redis-cli -p 6379 GET hb:user:<userId>:daily:$(date +%Y-%m-%d)
# Returns: "1"

# List all keys
redis-cli -p 6379 KEYS "hb:*"

# Watch pub/sub events live (open before clicking "Mark done")
# Each message is a JSON envelope: {user_id, event{event_id, event_type, timestamp, producer, payload}}
redis-cli -p 6379 SUBSCRIBE hb:events
```

Or watch the go-redis logs:
```bash
docker compose logs go-redis -f
```

### 5. Analytics (30s)
- Navigate to the Analytics page
- Show the per-habit progress bars
- Show the contribution heatmap with today's cell filled

### 6. Complete all habits + celebration (30s)
- Complete the remaining two habits
- Dashboard shows "🎉 All done for today!" banner
- Progress bar fills to 100%

## Talking Points

**Architecture:**
> "The backend is Go with a Chi router. When a habit is completed, it writes to PostgreSQL for durability, updates the streak counter in our custom go-redis server, then publishes a structured event — with a UUID, type, timestamp, and producer — to a Redis pub/sub channel. All components emit structured JSON logs so you can trace an event from the HTTP request all the way to the WebSocket client."

**go-redis:**
> "go-redis is a Redis-compatible server built from scratch in Go. It speaks the RESP v2 protocol over TCP, supports strings, hashes, counters, key expiry, pub/sub, and an append-only file for persistence. It lives in the repo as a git submodule."

**Realtime:**
> "The second tab just updated without any manual refresh. When a habit is completed, the handler builds a canonical event — a UUID, event type like `habit.completed`, a UTC timestamp, and a payload — and publishes it to go-redis on the `hb:events` channel. An EventBridge goroutine subscribed to that channel deduplicates by event ID and fans it out to all open WebSocket connections for that user. Because the event goes through Redis, you could run multiple API instances and every instance would receive and deliver it exactly once."

**Key design:**
> "The streak counter lives in go-redis as a plain string key: `hb:habit:{id}:streak`. It's O(1) to read, which is why dashboard loads are fast even with many habits."
