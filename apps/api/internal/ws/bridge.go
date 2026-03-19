package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/habit-buddy/api/internal/events"
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
// messages are silently dropped before reaching the router.
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

// seen returns true if id was already observed, otherwise records it and returns false.
func (c *seenCache) seen(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.ids[id]; ok {
		return true
	}
	if old := c.ring[c.head]; old != "" {
		delete(c.ids, old)
	}
	c.ids[id] = struct{}{}
	c.ring[c.head] = id
	c.head = (c.head + 1) % c.size
	return false
}

// EventBridge is the transport layer between Redis pub/sub and the event router.
//
// Responsibilities:
//   - Maintain two Redis connections: one for SUBSCRIBE, one for PUBLISH.
//   - Deduplicate events by event_id before forwarding.
//   - Decode the Redis envelope and pass an EventContext to the router.
//   - Expose Publish so HTTP handlers can emit events without knowing about the hub.
//
// The bridge is intentionally free of business logic — it delegates all
// routing and handler dispatch to the injected events.Router.
type EventBridge struct {
	router *events.Router
	sub    *redisclient.Subscriber
	pub    *redisclient.Client
	seen   *seenCache
	log    *slog.Logger
}

// NewEventBridge creates an EventBridge and starts the background listener.
// It opens two TCP connections to redisAddr: one for subscribing, one for publishing.
func NewEventBridge(redisAddr string, router *events.Router) (*EventBridge, error) {
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
		router: router,
		sub:    sub,
		pub:    pub,
		seen:   newSeenCache(1000),
		log:    logger.L.With("component", "event_bridge"),
	}
	go b.run()
	return b, nil
}

// Publish encodes event into an envelope and publishes it to the Redis channel.
// All API instances subscribed to hb:events will receive and route the message.
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

// run reads messages from the subscriber, deduplicates them, and forwards to the router.
func (b *EventBridge) run() {
	ctx := context.Background()

	for msg := range b.sub.Messages() {
		var env envelope
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			b.log.Error("envelope decode error", "error", err)
			continue
		}

		evt := env.Event

		if b.seen.seen(evt.EventID) {
			b.log.Info("duplicate event dropped",
				"event_id", evt.EventID,
				"event_type", evt.EventType,
			)
			continue
		}

		b.log.Info("event received from Redis",
			"event_id", evt.EventID,
			"event_type", evt.EventType,
			"user_id", env.UserID,
			"producer", evt.Producer,
		)

		b.router.Route(ctx, events.EventContext{
			UserID: env.UserID,
			Event:  evt,
		})
	}
}
