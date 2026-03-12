package ws

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/redisclient"
)

// eventChannel is the Redis pub/sub channel used for all realtime events.
// All API instances subscribe to this channel and fan-out to their local WS clients.
const eventChannel = "hb:events"

// envelope is the JSON payload published to the Redis event channel.
// It wraps a WS event with the target userID for routing.
type envelope struct {
	UserID  string          `json:"userID"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// EventBridge subscribes to the Redis event channel and fans out events
// to the local WebSocket hub. It also exposes Publish so that HTTP handlers
// can emit events without directly coupling to the hub.
//
// Using Redis pub/sub as the transport means multiple API instances can each
// subscribe to the same channel and broadcast to their own set of WS clients,
// enabling horizontal scaling without sticky sessions.
type EventBridge struct {
	hub *Hub
	sub *redisclient.Subscriber
	pub *redisclient.Client
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
	b := &EventBridge{hub: hub, sub: sub, pub: pub}
	go b.run()
	return b, nil
}

// Publish encodes event and publishes it to the Redis event channel for userID.
// All API instances (including this one) will receive it and broadcast to their
// local WebSocket clients for that user.
func (b *EventBridge) Publish(userID string, event model.WSEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	env := envelope{
		UserID:  userID,
		Type:    event.Type,
		Payload: payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
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
			log.Printf("event bridge: decode error: %v", err)
			continue
		}
		// Reconstruct as WSEvent; Payload is json.RawMessage (implements interface{}).
		// BroadcastToUser will re-marshal it when writing to each WS client.
		b.hub.BroadcastToUser(env.UserID, model.WSEvent{
			Type:    env.Type,
			Payload: env.Payload,
		})
	}
}
