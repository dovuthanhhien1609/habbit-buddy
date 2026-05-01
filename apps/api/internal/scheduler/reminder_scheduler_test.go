package scheduler_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/scheduler"
)

// ---------------------------------------------------------------------------
// Mocks satisfying scheduler interfaces
// ---------------------------------------------------------------------------

type mockPublisher struct {
	mu        sync.Mutex
	published []publishedEvent
}

type publishedEvent struct {
	UserID string
	Event  model.Event
}

func (m *mockPublisher) Publish(userID string, event model.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, publishedEvent{UserID: userID, Event: event})
	return nil
}

func (m *mockPublisher) snapshot() []publishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]publishedEvent, len(m.published))
	copy(out, m.published)
	return out
}

type mockReminderRepo struct {
	mu        sync.Mutex
	reminders []model.Reminder
}

func (r *mockReminderRepo) GetEnabledReminders() ([]model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.Reminder, 0, len(r.reminders))
	for _, rem := range r.reminders {
		if rem.Enabled {
			out = append(out, rem)
		}
	}
	return out, nil
}

type mockNotificationRepo struct {
	mu            sync.Mutex
	notifications []*model.Notification
}

func (r *mockNotificationRepo) CreateNotification(n *model.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications = append(r.notifications, n)
	return nil
}

func (r *mockNotificationRepo) snapshot() []*model.Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Notification, len(r.notifications))
	copy(out, r.notifications)
	return out
}

type mockHabitLookup struct{}

func (h *mockHabitLookup) GetByID(habitID, userID string) (*model.Habit, error) {
	return &model.Habit{ID: habitID, UserID: userID, Name: "Morning Run"}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeReminder(remindAt time.Time, weekday int, enabled bool) model.Reminder {
	return model.Reminder{
		ID:         uuid.New().String(),
		HabitID:    uuid.New().String(),
		UserID:     uuid.New().String(),
		RemindAt:   remindAt.UTC().Format("15:04"),
		DaysOfWeek: []int{weekday},
		Enabled:    enabled,
	}
}

func newSched(reminders []model.Reminder) (*scheduler.ReminderScheduler, *mockPublisher, *mockNotificationRepo) {
	pub := &mockPublisher{}
	notifRepo := &mockNotificationRepo{}
	remRepo := &mockReminderRepo{reminders: reminders}
	sched := scheduler.NewReminderScheduler(remRepo, notifRepo, &mockHabitLookup{}, pub)
	return sched, pub, notifRepo
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestScheduler_DueReminder_Fires(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC) // Friday = 5
	rem := makeReminder(now, int(now.Weekday()), true)

	sched, pub, notifRepo := newSched([]model.Reminder{rem})
	sched.Tick(context.Background(), now)

	events := pub.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(events))
	}
	if events[0].Event.EventType != model.EventReminder {
		t.Errorf("expected event_type %q, got %q", model.EventReminder, events[0].Event.EventType)
	}
	if events[0].UserID != rem.UserID {
		t.Errorf("published to wrong user: got %q", events[0].UserID)
	}

	var payload model.ReminderPayload
	if err := json.Unmarshal(events[0].Event.Payload, &payload); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if payload.HabitID != rem.HabitID {
		t.Errorf("payload habitId mismatch: got %q", payload.HabitID)
	}

	if len(notifRepo.snapshot()) != 1 {
		t.Errorf("expected 1 notification row, got %d", len(notifRepo.snapshot()))
	}
}

func TestScheduler_DisabledReminder_DoesNotFire(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	rem := makeReminder(now, int(now.Weekday()), false)

	sched, pub, notifRepo := newSched([]model.Reminder{rem})
	sched.Tick(context.Background(), now)

	if n := len(pub.snapshot()); n != 0 {
		t.Errorf("expected 0 events for disabled reminder, got %d", n)
	}
	if n := len(notifRepo.snapshot()); n != 0 {
		t.Errorf("expected 0 notifications for disabled reminder, got %d", n)
	}
}

func TestScheduler_WrongWeekday_DoesNotFire(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC) // Friday = 5
	rem := makeReminder(now, 6 /* Saturday */, true)

	sched, pub, notifRepo := newSched([]model.Reminder{rem})
	sched.Tick(context.Background(), now)

	if n := len(pub.snapshot()); n != 0 {
		t.Errorf("expected 0 events for wrong weekday, got %d", n)
	}
	if n := len(notifRepo.snapshot()); n != 0 {
		t.Errorf("expected 0 notifications for wrong weekday, got %d", n)
	}
}

func TestScheduler_WrongTime_DoesNotFire(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	remindAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC) // different hour
	rem := makeReminder(remindAt, int(now.Weekday()), true)

	sched, pub, notifRepo := newSched([]model.Reminder{rem})
	sched.Tick(context.Background(), now)

	if n := len(pub.snapshot()); n != 0 {
		t.Errorf("expected 0 events for wrong time, got %d", n)
	}
	if n := len(notifRepo.snapshot()); n != 0 {
		t.Errorf("expected 0 notifications for wrong time, got %d", n)
	}
}

func TestScheduler_MultipleReminders_OnlyDueFires(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC) // Friday = 5
	dueRem := makeReminder(now, int(now.Weekday()), true)
	wrongTime := makeReminder(time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC), int(now.Weekday()), true)
	wrongDay := makeReminder(now, 6 /* Saturday */, true)
	disabled := makeReminder(now, int(now.Weekday()), false)

	sched, pub, notifRepo := newSched([]model.Reminder{dueRem, wrongTime, wrongDay, disabled})
	sched.Tick(context.Background(), now)

	if n := len(pub.snapshot()); n != 1 {
		t.Errorf("expected exactly 1 event, got %d", n)
	}
	if n := len(notifRepo.snapshot()); n != 1 {
		t.Errorf("expected exactly 1 notification, got %d", n)
	}
}
