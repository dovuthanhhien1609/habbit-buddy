# Demo Scenario

## Setup

```bash
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
redis-cli -p 6379 GET hb:user:<userId>:daily:2026-03-11
# Returns: "1"

# List all keys
redis-cli -p 6379 KEYS "hb:*"
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
> "The backend is Go with a Chi router. When a habit is completed, it writes to PostgreSQL for durability, then updates the streak counter in our custom go-redis server."

**go-redis:**
> "go-redis is a Redis-compatible server built from scratch in Go. It speaks the RESP protocol over TCP, has a thread-safe in-memory store, and persists commands to an append-only file."

**Realtime:**
> "The second tab just updated without any manual refresh. The backend emits an event to our WebSocket hub, which fans it out to all open connections for that user."

**Key design:**
> "The streak counter lives in go-redis as a simple string key: `hb:habit:{id}:streak`. It's O(1) to read, which is why dashboard loads are fast even with many habits."
