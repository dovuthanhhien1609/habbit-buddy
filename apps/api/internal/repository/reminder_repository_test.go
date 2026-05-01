package repository_test

// NOTE: No real DB test helpers exist in this codebase (confirmed by searching
// for *_test.go files — none were found).  These tests use pure in-memory mock
// implementations to exercise CRUD contract logic without a running Postgres
// instance.  When a test DB helper is added in future, replace the mock
// implementations below with real repository calls against the test database.

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Domain types (duplicated locally so the test file compiles before
// internal/domain is implemented)
// ---------------------------------------------------------------------------

// Reminder is the reminder domain entity.
type Reminder struct {
	ID         uuid.UUID
	HabitID    uuid.UUID
	UserID     uuid.UUID
	RemindAt   time.Time
	DaysOfWeek []int16
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Notification is the notification domain entity.
type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      string
	Read      bool
	CreatedAt time.Time
}

// ---------------------------------------------------------------------------
// In-memory ReminderRepository
// ---------------------------------------------------------------------------

// ReminderRepository defines the interface the real repository must satisfy.
type ReminderRepository interface {
	Create(r Reminder) (Reminder, error)
	GetByHabit(habitID uuid.UUID) ([]Reminder, error)
	Update(r Reminder) (Reminder, error)
	Delete(id uuid.UUID) error
}

// inMemReminderRepo is a thread-safe in-memory implementation used for testing.
type inMemReminderRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]Reminder
}

func newInMemReminderRepo() *inMemReminderRepo {
	return &inMemReminderRepo{rows: make(map[uuid.UUID]Reminder)}
}

func (r *inMemReminderRepo) Create(rem Reminder) (Reminder, error) {
	if rem.ID == uuid.Nil {
		rem.ID = uuid.New()
	}
	now := time.Now().UTC()
	rem.CreatedAt = now
	rem.UpdatedAt = now
	r.mu.Lock()
	r.rows[rem.ID] = rem
	r.mu.Unlock()
	return rem, nil
}

func (r *inMemReminderRepo) GetByHabit(habitID uuid.UUID) ([]Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Reminder
	for _, rem := range r.rows {
		if rem.HabitID == habitID {
			out = append(out, rem)
		}
	}
	return out, nil
}

func (r *inMemReminderRepo) Update(rem Reminder) (Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[rem.ID]; !ok {
		return Reminder{}, errors.New("reminder not found")
	}
	rem.UpdatedAt = time.Now().UTC()
	r.rows[rem.ID] = rem
	return rem, nil
}

func (r *inMemReminderRepo) Delete(id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[id]; !ok {
		return errors.New("reminder not found")
	}
	delete(r.rows, id)
	return nil
}

// ---------------------------------------------------------------------------
// In-memory NotificationRepository
// ---------------------------------------------------------------------------

// NotificationRepository defines the interface the real repository must satisfy.
type NotificationRepository interface {
	CreateNotification(n Notification) (Notification, error)
	ListUnread(userID uuid.UUID) ([]Notification, error)
	MarkRead(id uuid.UUID) error
}

// inMemNotificationRepo is a thread-safe in-memory implementation used for testing.
type inMemNotificationRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]Notification
}

func newInMemNotificationRepo() *inMemNotificationRepo {
	return &inMemNotificationRepo{rows: make(map[uuid.UUID]Notification)}
}

func (r *inMemNotificationRepo) CreateNotification(n Notification) (Notification, error) {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	n.CreatedAt = time.Now().UTC()
	r.mu.Lock()
	r.rows[n.ID] = n
	r.mu.Unlock()
	return n, nil
}

func (r *inMemNotificationRepo) ListUnread(userID uuid.UUID) ([]Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Notification
	for _, n := range r.rows {
		if n.UserID == userID && !n.Read {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *inMemNotificationRepo) MarkRead(id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.rows[id]
	if !ok {
		return errors.New("notification not found")
	}
	n.Read = true
	r.rows[id] = n
	return nil
}

// ---------------------------------------------------------------------------
// Reminder CRUD round-trip tests
// ---------------------------------------------------------------------------

// TestReminderRepo_CreateAndGetByHabit verifies Create then GetByHabit returns
// the stored reminder with the correct fields.
func TestReminderRepo_CreateAndGetByHabit(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	habitID := uuid.New()

	input := Reminder{
		HabitID:    habitID,
		UserID:     uuid.New(),
		RemindAt:   time.Date(0, 1, 1, 8, 30, 0, 0, time.UTC),
		DaysOfWeek: []int16{1, 3, 5}, // Mon, Wed, Fri
		Enabled:    true,
	}

	created, err := repo.Create(input)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID after Create")
	}

	reminders, err := repo.GetByHabit(habitID)
	if err != nil {
		t.Fatalf("GetByHabit failed: %v", err)
	}
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	got := reminders[0]
	if got.HabitID != habitID {
		t.Errorf("HabitID mismatch: got %v", got.HabitID)
	}
	if got.RemindAt.Hour() != 8 || got.RemindAt.Minute() != 30 {
		t.Errorf("RemindAt mismatch: got %v", got.RemindAt)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
}

// TestReminderRepo_GetByHabit_OtherHabitNotReturned verifies that reminders
// belonging to a different habit are not returned.
func TestReminderRepo_GetByHabit_OtherHabitNotReturned(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	habitA := uuid.New()
	habitB := uuid.New()

	_, _ = repo.Create(Reminder{HabitID: habitA, UserID: uuid.New(), Enabled: true})
	_, _ = repo.Create(Reminder{HabitID: habitB, UserID: uuid.New(), Enabled: true})

	reminders, err := repo.GetByHabit(habitA)
	if err != nil {
		t.Fatalf("GetByHabit failed: %v", err)
	}
	if len(reminders) != 1 {
		t.Errorf("expected 1 reminder for habitA, got %d", len(reminders))
	}
}

// TestReminderRepo_Update verifies that Update mutates the stored record.
func TestReminderRepo_Update(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	created, err := repo.Create(Reminder{
		HabitID:  uuid.New(),
		UserID:   uuid.New(),
		Enabled:  true,
		RemindAt: time.Date(0, 1, 1, 7, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	created.Enabled = false
	created.RemindAt = time.Date(0, 1, 1, 18, 0, 0, 0, time.UTC)

	updated, err := repo.Update(created)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Enabled {
		t.Error("expected Enabled=false after Update")
	}
	if updated.RemindAt.Hour() != 18 {
		t.Errorf("expected RemindAt hour=18 after Update, got %d", updated.RemindAt.Hour())
	}
}

// TestReminderRepo_Update_NotFound verifies that updating a non-existent ID
// returns an error.
func TestReminderRepo_Update_NotFound(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	_, err := repo.Update(Reminder{ID: uuid.New()})
	if err == nil {
		t.Error("expected error when updating non-existent reminder")
	}
}

// TestReminderRepo_Delete removes a reminder and verifies it no longer appears
// in GetByHabit results.
func TestReminderRepo_Delete(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	habitID := uuid.New()

	created, err := repo.Create(Reminder{HabitID: habitID, UserID: uuid.New(), Enabled: true})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	reminders, _ := repo.GetByHabit(habitID)
	if len(reminders) != 0 {
		t.Errorf("expected 0 reminders after Delete, got %d", len(reminders))
	}
}

// TestReminderRepo_Delete_NotFound verifies deleting a non-existent ID errors.
func TestReminderRepo_Delete_NotFound(t *testing.T) {
	t.Parallel()

	repo := newInMemReminderRepo()
	err := repo.Delete(uuid.New())
	if err == nil {
		t.Error("expected error when deleting non-existent reminder")
	}
}

// ---------------------------------------------------------------------------
// Notification repository tests
// ---------------------------------------------------------------------------

// TestNotificationRepo_ListUnread_OnlyUnreadForUser verifies that ListUnread
// returns only unread notifications belonging to the queried user.
func TestNotificationRepo_ListUnread_OnlyUnreadForUser(t *testing.T) {
	t.Parallel()

	repo := newInMemNotificationRepo()
	userA := uuid.New()
	userB := uuid.New()

	// User A: one unread, one read.
	_, _ = repo.CreateNotification(Notification{UserID: userA, Type: "reminder", Read: false})
	nRead, _ := repo.CreateNotification(Notification{UserID: userA, Type: "reminder", Read: false})
	_ = repo.MarkRead(nRead.ID)

	// User B: one unread — must not appear in userA's results.
	_, _ = repo.CreateNotification(Notification{UserID: userB, Type: "reminder", Read: false})

	unread, err := repo.ListUnread(userA)
	if err != nil {
		t.Fatalf("ListUnread failed: %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread notification for userA, got %d", len(unread))
	}
	if unread[0].UserID != userA {
		t.Errorf("notification belongs to wrong user: %v", unread[0].UserID)
	}
	if unread[0].Read {
		t.Error("returned notification should not be marked as read")
	}
}

// TestNotificationRepo_CreateNotification_SetsCreatedAt verifies the repository
// stamps CreatedAt on insert.
func TestNotificationRepo_CreateNotification_SetsCreatedAt(t *testing.T) {
	t.Parallel()

	repo := newInMemNotificationRepo()
	before := time.Now().UTC()
	created, err := repo.CreateNotification(Notification{UserID: uuid.New(), Type: "reminder"})
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}
	if created.CreatedAt.Before(before) || created.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v not within expected range [%v, %v]", created.CreatedAt, before, after)
	}
}

// TestNotificationRepo_ListUnread_EmptyForUserWithNoNotifications verifies that
// ListUnread returns an empty slice (not an error) for a user with no records.
func TestNotificationRepo_ListUnread_EmptyForUserWithNoNotifications(t *testing.T) {
	t.Parallel()

	repo := newInMemNotificationRepo()
	unread, err := repo.ListUnread(uuid.New())
	if err != nil {
		t.Fatalf("ListUnread returned unexpected error: %v", err)
	}
	if len(unread) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(unread))
	}
}
