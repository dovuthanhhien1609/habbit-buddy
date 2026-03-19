// Package events provides the event routing layer between Redis pub/sub and
// downstream consumers (WebSocket hub, services, etc.).
//
// Architecture:
//
//	Redis Subscriber → EventBridge → seenCache (dedup) → Router.Route()
//	                                                           ↓
//	                                     map[event_type] → []Handler (+ retry)
//	                                                           ↓
//	                                          WSBroadcastHandler → Hub.BroadcastToChannel
package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/habit-buddy/api/internal/logger"
	"github.com/habit-buddy/api/internal/model"
)

// EventContext carries the routing metadata (UserID from the Redis envelope)
// alongside the domain event, so handlers can make channel-targeting decisions
// without embedding routing concerns in the event itself.
type EventContext struct {
	UserID string
	Event  model.Event
}

// Handler processes a single routed event.
type Handler interface {
	Handle(ctx context.Context, ec EventContext) error
}

// HandlerFunc adapts a plain function to the Handler interface.
type HandlerFunc func(ctx context.Context, ec EventContext) error

func (f HandlerFunc) Handle(ctx context.Context, ec EventContext) error {
	return f(ctx, ec)
}

// RetryPolicy defines how failed handlers are retried using exponential backoff.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

// DefaultRetryPolicy retries up to 3 times: 100 ms → 200 ms → 400 ms.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts: 3,
	BaseDelay:   100 * time.Millisecond,
}

// Router dispatches events to registered handlers by event_type.
// Multiple handlers may be registered for the same event type; each is called
// independently with its own retry budget.
type Router struct {
	handlers map[string][]Handler
	retry    RetryPolicy
	log      *slog.Logger
}

// NewRouter creates a Router with the given retry policy.
func NewRouter(retry RetryPolicy) *Router {
	return &Router{
		handlers: make(map[string][]Handler),
		retry:    retry,
		log:      logger.L.With("component", "event_router"),
	}
}

// On registers h to be called whenever an event of eventType is routed.
// Calling On multiple times for the same type appends the handler.
func (r *Router) On(eventType string, h Handler) {
	r.handlers[eventType] = append(r.handlers[eventType], h)
}

// Route dispatches ec to all handlers registered for ec.Event.EventType.
// Each handler is invoked with the configured retry policy. Errors are logged
// but do not block other registered handlers.
func (r *Router) Route(ctx context.Context, ec EventContext) {
	handlers, ok := r.handlers[ec.Event.EventType]
	if !ok {
		r.log.Warn("no handlers registered for event_type",
			"event_id", ec.Event.EventID,
			"event_type", ec.Event.EventType,
		)
		return
	}

	r.log.Info("routing event",
		"event_id", ec.Event.EventID,
		"event_type", ec.Event.EventType,
		"user_id", ec.UserID,
		"handler_count", len(handlers),
	)

	for _, h := range handlers {
		if err := r.withRetry(ctx, ec, h); err != nil {
			r.log.Error("handler failed after all retries",
				"event_id", ec.Event.EventID,
				"event_type", ec.Event.EventType,
				"error", err,
			)
		}
	}
}

// withRetry calls h.Handle with exponential backoff up to r.retry.MaxAttempts.
// The delay doubles each attempt: BaseDelay, 2×BaseDelay, 4×BaseDelay, …
func (r *Router) withRetry(ctx context.Context, ec EventContext, h Handler) error {
	var lastErr error
	for attempt := 1; attempt <= r.retry.MaxAttempts; attempt++ {
		if err := h.Handle(ctx, ec); err == nil {
			return nil
		} else {
			lastErr = err
			if attempt == r.retry.MaxAttempts {
				break
			}
			delay := r.retry.BaseDelay * time.Duration(1<<uint(attempt-1))
			r.log.Warn("handler error, will retry",
				"event_id", ec.Event.EventID,
				"event_type", ec.Event.EventType,
				"attempt", attempt,
				"max_attempts", r.retry.MaxAttempts,
				"retry_in", delay.String(),
				"error", lastErr,
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			}
		}
	}
	return lastErr
}
