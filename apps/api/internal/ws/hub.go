package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/habit-buddy/api/internal/logger"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // dev: allow all origins
	},
}

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID string
	send   chan []byte
}

// Hub manages channel-based WebSocket subscriptions.
//
// Supported channel patterns:
//   - "user:{id}"  — automatically subscribed on connect; delivers events for that user.
//   - "habit:{id}" — client-initiated via subscription message; delivers habit-scoped events.
//   - "team:{id}"  — client-initiated; reserved for future team-level fan-out.
//
// Clients are NOT broadcast to blindly — only channels they are subscribed to receive messages.
type Hub struct {
	// channels maps channel name → set of subscribed clients.
	channels map[string]map[*Client]bool
	// clientChans maps each client → its subscribed channels (for O(1) cleanup).
	clientChans map[*Client]map[string]bool

	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	log        *slog.Logger
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		channels:    make(map[string]map[*Client]bool),
		clientChans: make(map[*Client]map[string]bool),
		register:    make(chan *Client, 64),
		unregister:  make(chan *Client, 64),
		log:         logger.L.With("component", "ws_hub"),
	}
}

// Run processes register/unregister events. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if h.clientChans[c] == nil {
				h.clientChans[c] = make(map[string]bool)
			}
			h.mu.Unlock()
			h.log.Info("client registered", "user_id", c.userID)

		case c := <-h.unregister:
			h.mu.Lock()
			// Remove the client from every channel it is subscribed to.
			for ch := range h.clientChans[c] {
				if set := h.channels[ch]; set != nil {
					delete(set, c)
					if len(set) == 0 {
						delete(h.channels, ch)
					}
				}
			}
			delete(h.clientChans, c)
			h.mu.Unlock()
			close(c.send)
			h.log.Info("client unregistered", "user_id", c.userID)
		}
	}
}

// Subscribe adds c to the named channel.
// Safe to call concurrently with Run.
func (h *Hub) Subscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*Client]bool)
	}
	h.channels[channel][c] = true
	if h.clientChans[c] == nil {
		h.clientChans[c] = make(map[string]bool)
	}
	h.clientChans[c][channel] = true
	h.log.Info("client subscribed to channel", "user_id", c.userID, "channel", channel)
}

// Unsubscribe removes c from the named channel.
func (h *Hub) Unsubscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.channels[channel]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.channels, channel)
		}
	}
	if chans := h.clientChans[c]; chans != nil {
		delete(chans, channel)
	}
	h.log.Info("client unsubscribed from channel", "user_id", c.userID, "channel", channel)
}

// BroadcastToChannel sends msg to every client subscribed to channel.
// Clients whose send buffer is full are disconnected.
func (h *Hub) BroadcastToChannel(channel string, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("marshal error", "error", err, "channel", channel)
		return
	}

	h.mu.RLock()
	set := h.channels[channel]
	h.mu.RUnlock()

	for c := range set {
		select {
		case c.send <- data:
		default:
			h.unregister <- c
		}
	}
}

// BroadcastToUser is a convenience wrapper that targets the "user:{userID}" channel.
func (h *Hub) BroadcastToUser(userID string, msg any) {
	h.BroadcastToChannel("user:"+userID, msg)
}

// ServeWS upgrades the HTTP connection, registers the client, and auto-subscribes
// it to its "user:{userID}" channel.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("upgrade error", "error", err, "user_id", userID)
		return
	}

	c := &Client{
		hub:    h,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
	}
	h.register <- c
	// Auto-subscribe to the user's personal channel immediately.
	// This is the default delivery channel for all user-scoped events.
	h.Subscribe(c, "user:"+userID)

	go c.writePump()
	go c.readPump()
}

// subMessage is the JSON control message clients send to subscribe/unsubscribe.
//
//	{"action": "subscribe",   "channel": "habit:abc123"}
//	{"action": "unsubscribe", "channel": "habit:abc123"}
type subMessage struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

// isClientSubscribable returns true for channel patterns that clients may
// manage themselves. "user:{id}" channels are server-managed only.
func isClientSubscribable(channel string) bool {
	return strings.HasPrefix(channel, "habit:") ||
		strings.HasPrefix(channel, "team:")
}

// readPump reads control messages (subscribe/unsubscribe) and handles ping/pong.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg subMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue // not a control message — ignore
		}
		if !isClientSubscribable(msg.Channel) {
			continue // disallow user:{id} or unknown patterns
		}
		switch msg.Action {
		case "subscribe":
			c.hub.Subscribe(c, msg.Channel)
		case "unsubscribe":
			c.hub.Unsubscribe(c, msg.Channel)
		}
	}
}

// writePump sends messages from the send channel to the WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
