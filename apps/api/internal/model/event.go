package model

import (
	"encoding/json"
	"time"
)

// Event is the canonical event envelope published on every state change.
// All inter-service and pub/sub messages use this schema.
type Event struct {
	EventID   string          `json:"event_id"`  // UUID v4
	EventType string          `json:"event_type"` // e.g. "habit.completed"
	Timestamp time.Time       `json:"timestamp"`  // UTC wall time
	Producer  string          `json:"producer"`   // service that emitted the event
	Payload   json.RawMessage `json:"payload"`    // event-specific data
}

// Event type constants — dot-namespaced, lowercase.
const (
	EventHabitCompleted = "habit.completed"
	EventHabitUndone    = "habit.undone"
	EventHabitCreated   = "habit.created"
	EventHabitUpdated   = "habit.updated"
	EventHabitArchived  = "habit.archived"
)
