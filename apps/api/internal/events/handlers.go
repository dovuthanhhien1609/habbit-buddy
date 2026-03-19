package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/habit-buddy/api/internal/logger"
	"github.com/habit-buddy/api/internal/model"
)

// Broadcaster is the subset of ws.Hub needed by handlers.
// Using an interface here avoids an import cycle between the events and ws packages.
type Broadcaster interface {
	BroadcastToChannel(channel string, msg any)
}

// WSBroadcastHandler fans a routed event out to WebSocket clients via the Hub.
//
// Channel targeting:
//   - "user:{userID}"   — always; delivers to all tabs open for that user.
//   - "habit:{habitID}" — when the payload contains a habitId field; delivers
//     to any client that explicitly subscribed to that habit's channel.
type WSBroadcastHandler struct {
	hub Broadcaster
	log *slog.Logger
}

// NewWSBroadcastHandler returns a handler that broadcasts to the given hub.
func NewWSBroadcastHandler(hub Broadcaster) *WSBroadcastHandler {
	return &WSBroadcastHandler{
		hub: hub,
		log: logger.L.With("component", "ws_broadcast_handler"),
	}
}

// Handle converts ec into a WSEvent and broadcasts it to the appropriate channels.
func (h *WSBroadcastHandler) Handle(_ context.Context, ec EventContext) error {
	wsEvent := model.WSEvent{
		EventID:   ec.Event.EventID,
		Type:      ec.Event.EventType,
		Timestamp: ec.Event.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Producer:  ec.Event.Producer,
		Payload:   ec.Event.Payload,
	}

	// Always deliver to the owning user.
	userChannel := "user:" + ec.UserID
	h.hub.BroadcastToChannel(userChannel, wsEvent)
	h.log.Info("broadcast to user channel",
		"event_id", ec.Event.EventID,
		"channel", userChannel,
	)

	// Additionally deliver to the habit channel when the payload carries a habitId.
	// This lets any client that subscribed to "habit:{id}" receive the update —
	// useful for shared/team habit dashboards.
	if habitID := extractHabitID(ec.Event.Payload); habitID != "" {
		habitChannel := "habit:" + habitID
		h.hub.BroadcastToChannel(habitChannel, wsEvent)
		h.log.Info("broadcast to habit channel",
			"event_id", ec.Event.EventID,
			"channel", habitChannel,
		)
	}

	return nil
}

// extractHabitID parses the habitId field from a JSON payload.
// Returns an empty string if the field is absent or the payload cannot be parsed.
func extractHabitID(payload json.RawMessage) string {
	var p struct {
		HabitID string `json:"habitId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return p.HabitID
}
