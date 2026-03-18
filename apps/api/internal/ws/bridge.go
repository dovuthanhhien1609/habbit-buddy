package ws

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/habit-buddy/api/internal/logger"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/redisclient"
)

// eventChannel is the Redis pub/sub channel used for all realtime events.
const eventChannel = "hb:events"

// envelope is the JSON payload published to the Redis event channel.
// It wraps an Event with the target userID for routing.
type envelope struct {
	UserID string      `json:"user_id"`
	Event  model.Event `json:"event"`
}

// seenCache is a fixed-size in-memory deduplication ring buffer.
// It keeps the last `size` event IDs so that replayed or duplicate
// messages are silently dropped without hitting the database.
type seenCache struct {
	mu   sync.Mutex
	ids  map[string]struct{}
	ring []string
	head int
	size int
}

func newSeenCache(size int) *seenCache {
	return &seenCache{
		ids:  make(map[string]struct{}, size),
		ring: make([]string, size),
		size: size,
	}
}

// seen returns true if id was already observed, otherwise it records id and
// returns false.
func (c *seenCache) seen(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.ids[id]; ok {
		return true
	}
	// Evict the oldest entry when the ring is full.
	if old := c.ring[c.head]; old != "" {
		delete(c.ids, old)
	}
	c.ids[id] = struct{}{}
	c.ring[c.head] = id
	c.head = (c.head + 1) % c.size
	return false
}

// EventBridge subscribes to the Redis event channel and fans out events
// to the local WebSocket hub. It also exposes Publish so that HTTP handlers
// can emit events without directly coupling to the hub.
//
// Using Redis pub/sub as the transport means multiple API instances can each
// subscribe to the same channel and broadcast to their own set of WS clients,
// enabling horizontal scaling without sticky sessions.
type EventBridge struct {
	hub  *Hub
	sub  *redisclient.Subscriber
	pub  *redisclient.Client
	seen *seenCache
	log  *slog.Logger
}

// NewEventBridge creates an EventBridge and starts the background listener.
// It opens two connections to redisAddr: one for subscribing, one for publishing.
func NewEventBridge(redisAddr string, hub *Hub) (*EventBridge, error) {
	sub, err := redisclient.NewSubscriber(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("event bridge: subscriber: %w", err)
	}
	pub, err := redisclient.NewClient(redisAddr)
	if err != nil {
		sub.Close()
		return nil, fmt.Errorf("event bridge: publisher: %w", err)
	}
	if err := sub.Subscribe(eventChannel); err != nil {
		sub.Close()
		pub.Close()
		return nil, fmt.Errorf("event bridge: subscribe: %w", err)
	}
	b := &EventBridge{
		hub:  hub,
		sub:  sub,
		pub:  pub,
		seen: newSeenCache(1000),
		log:  logger.L.With("component", "event_bridge"),
	}
	go b.run()
	return b, nil
}

// Publish marshals event and publishes it to the Redis event channel for userID.
// All API instances (including this one) will receive it and broadcast to their
// local WebSocket clients for that user.
func (b *EventBridge) Publish(userID string, event model.Event) error {
	env := envelope{UserID: userID, Event: event}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	b.log.Info("event published to Redis",
		"event_id", event.EventID,
		"event_type", event.EventType,
		"user_id", userID,
		"producer", event.Producer,
	)

	_, err = b.pub.Publish(eventChannel, string(data))
	return err
}

// Close stops the bridge and releases both Redis connections.
func (b *EventBridge) Close() error {
	b.pub.Close()
	return b.sub.Close()
}

// run reads messages from the subscriber and broadcasts each event to the hub.
func (b *EventBridge) run() {
	for msg := range b.sub.Messages() {
		var env envelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			b.log.Error("event decode error", "error", err)
			continue
		}

		evt := env.Event

		// Idempotency: drop events whose ID has already been processed.
		if b.seen.seen(evt.EventID) {
			b.log.Info("duplicate event dropped",
				"event_id", evt.EventID,
				"event_type", evt.EventType,
			)
			continue
		}

		b.log.Info("event received from Redis subscriber",
			"event_id", evt.EventID,
			"event_type", evt.EventType,
			"user_id", env.UserID,
			"producer", evt.Producer,
		)

		// Build the WS wire message delivered to clients.
		wsEvent := model.WSEvent{
			EventID:   evt.EventID,
			Type:      evt.EventType,
			Timestamp: evt.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Producer:  evt.Producer,
			Payload:   evt.Payload,
		}

		b.log.Info("broadcasting event to WebSocket clients",
			"event_id", evt.EventID,
			"event_type", evt.EventType,
			"user_id", env.UserID,
		)

		b.hub.BroadcastToUser(env.UserID, wsEvent)
	}
}
