// Package scheduler provides time-based background jobs.
// The ReminderScheduler fires a 60-second ticker and, on each tick,
// loads all enabled reminders from the database, checks whether the
// current UTC time (hour + minute) and weekday match the reminder's
// configuration, and for each matching reminder creates a notification
// row and publishes a "reminder" event to the Redis event bus.
package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/habit-buddy/api/internal/logger"
	"github.com/habit-buddy/api/internal/model"
)

// Publisher is the subset of ws.EventBridge used by the scheduler.
// Declared here so the scheduler package does not import the ws package.
type Publisher interface {
	Publish(userID string, event model.Event) error
}

// ReminderRepository is the subset of repository.ReminderRepository needed here.
type ReminderRepository interface {
	GetEnabledReminders() ([]model.Reminder, error)
}

// NotificationRepository is the subset of repository.NotificationRepository needed here.
type NotificationRepository interface {
	CreateNotification(n *model.Notification) error
}

// HabitLookup returns the name of a habit by ID and owner — used to build
// human-readable notification bodies. It is satisfied by repository.HabitRepository.
type HabitLookup interface {
	GetByID(habitID, userID string) (*model.Habit, error)
}

// ReminderScheduler checks enabled reminders every minute and fires
// notifications + events for those that are due.
type ReminderScheduler struct {
	reminders     ReminderRepository
	notifications NotificationRepository
	habits        HabitLookup
	publisher     Publisher
	log           *slog.Logger
}

// NewReminderScheduler constructs the scheduler. All parameters are satisfied
// by the concrete repository types but declared as interfaces so tests can
// inject mocks without a real database.
func NewReminderScheduler(
	reminders ReminderRepository,
	notifications NotificationRepository,
	habits HabitLookup,
	publisher Publisher,
) *ReminderScheduler {
	return &ReminderScheduler{
		reminders:     reminders,
		notifications: notifications,
		habits:        habits,
		publisher:     publisher,
		log:           logger.L.With("component", "reminder_scheduler"),
	}
}

// Start launches the scheduler loop. It blocks until ctx is cancelled.
// Call it in a goroutine: go scheduler.Start(ctx).
func (s *ReminderScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	s.log.Info("reminder scheduler started")

	for {
		select {
		case <-ctx.Done():
			s.log.Info("reminder scheduler stopped")
			return
		case t := <-ticker.C:
			s.tick(ctx, t.UTC())
		}
	}
}

// Tick is an exported entry point for a single scheduler cycle at time t (UTC).
// It is used by tests to drive the scheduler deterministically without a running ticker.
func (s *ReminderScheduler) Tick(ctx context.Context, t time.Time) {
	s.tick(ctx, t)
}

// tick processes a single scheduler cycle at time t (UTC).
func (s *ReminderScheduler) tick(ctx context.Context, t time.Time) {
	reminders, err := s.reminders.GetEnabledReminders()
	if err != nil {
		s.log.Error("scheduler: failed to load reminders", "error", err)
		return
	}

	currentHour := t.Hour()
	currentMinute := t.Minute()
	currentWeekday := int(t.Weekday()) // Sunday = 0

	for _, rem := range reminders {
		if !reminderIsDue(rem, currentHour, currentMinute, currentWeekday) {
			continue
		}
		if err := s.fireReminder(ctx, rem); err != nil {
			s.log.Error("scheduler: failed to fire reminder",
				"reminder_id", rem.ID,
				"habit_id", rem.HabitID,
				"error", err,
			)
		}
	}
}

// reminderIsDue returns true when rem.RemindAt matches hour:minute and the
// current weekday is in rem.DaysOfWeek.
func reminderIsDue(rem model.Reminder, hour, minute, weekday int) bool {
	// RemindAt is stored/returned as "HH:MM:SS" (postgres TIME cast to text).
	// We accept both "HH:MM" and "HH:MM:SS".
	var rHour, rMinute int
	n, err1 := parseHHMM(rem.RemindAt, &rHour, &rMinute)
	if err1 != nil || n < 2 {
		return false
	}
	if rHour != hour || rMinute != minute {
		return false
	}
	for _, d := range rem.DaysOfWeek {
		if d == weekday {
			return true
		}
	}
	return false
}

// parseHHMM is a zero-allocation parser for "HH:MM" or "HH:MM:SS".
func parseHHMM(s string, hour, minute *int) (int, error) {
	if len(s) < 5 {
		return 0, errInvalidTime
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	if s[2] != ':' {
		return 0, errInvalidTime
	}
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	*hour = h
	*minute = m
	return 2, nil
}

type parseError string

func (e parseError) Error() string { return string(e) }

const errInvalidTime parseError = "invalid time format"

// fireReminder creates a notification row and publishes the reminder event.
func (s *ReminderScheduler) fireReminder(ctx context.Context, rem model.Reminder) error {
	// Resolve habit name for the notification body.
	habitName := "your habit"
	if h, err := s.habits.GetByID(rem.HabitID, rem.UserID); err == nil && h != nil {
		habitName = h.Name
	}

	title := "Time for your habit!"
	body := "Don't forget: " + habitName

	notifID := uuid.New().String()
	now := time.Now().UTC()

	notif := &model.Notification{
		ID:        notifID,
		UserID:    rem.UserID,
		Type:      model.EventReminder,
		Title:     title,
		Body:      body,
		Read:      false,
		CreatedAt: now,
	}
	if err := s.notifications.CreateNotification(notif); err != nil {
		return err
	}

	payload, err := json.Marshal(model.ReminderPayload{
		HabitID:        rem.HabitID,
		HabitName:      habitName,
		NotificationID: notifID,
		Title:          title,
		Body:           body,
	})
	if err != nil {
		return err
	}

	event := model.Event{
		EventID:   uuid.New().String(),
		EventType: model.EventReminder,
		Timestamp: now,
		Producer:  "scheduler",
		Payload:   payload,
	}

	s.log.Info("firing reminder",
		"reminder_id", rem.ID,
		"habit_id", rem.HabitID,
		"user_id", rem.UserID,
		"event_id", event.EventID,
	)

	return s.publisher.Publish(rem.UserID, event)
}

