package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 512
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

// Hub maintains the set of active clients grouped by userID.
type Hub struct {
	// clients maps userID → set of active clients
	clients map[string]map[*Client]bool
	mu      sync.RWMutex

	register   chan *Client
	unregister chan *Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

// Run processes register/unregister events. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if h.clients[c.userID] == nil {
				h.clients[c.userID] = make(map[*Client]bool)
			}
			h.clients[c.userID][c] = true
			h.mu.Unlock()
			log.Printf("ws: client registered userID=%s", c.userID)

		case c := <-h.unregister:
			h.mu.Lock()
			if set, ok := h.clients[c.userID]; ok {
				delete(set, c)
				if len(set) == 0 {
					delete(h.clients, c.userID)
				}
			}
			h.mu.Unlock()
			close(c.send)
			log.Printf("ws: client unregistered userID=%s", c.userID)
		}
	}
}

// BroadcastToUser sends a message to every WS client for a given userID.
func (h *Hub) BroadcastToUser(userID string, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}

	h.mu.RLock()
	set := h.clients[userID]
	h.mu.RUnlock()

	for c := range set {
		select {
		case c.send <- data:
		default:
			// Client's send buffer is full — drop and disconnect.
			h.unregister <- c
		}
	}
}

// ServeWS upgrades the HTTP connection and registers the client.
// The userID must be extracted from the JWT before calling this.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}

	c := &Client{
		hub:    h,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
	}
	h.register <- c

	go c.writePump()
	go c.readPump()
}

// readPump reads incoming messages (we ignore them, only used for ping/pong).
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
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
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
