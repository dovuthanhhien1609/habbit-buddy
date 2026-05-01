package api_test

// Handler integration tests for the Reminder & Notification endpoints.
//
// These tests use httptest.NewRecorder() and a locally-built Chi router.
// All service/repo layers are mocked — no real database or Redis is required.
//
// Endpoints under test:
//   POST /api/habits/{habitId}/reminders → 201 with reminder JSON
//   GET  /api/notifications             → 200 with authed user's notifications only
//
// Auth: a real JWT is minted using the test secret so the existing
// internal/middleware.Authenticate middleware passes without modification.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	appmiddleware "github.com/habit-buddy/api/internal/middleware"
)

// ---------------------------------------------------------------------------
// Domain types (local to test — replace with domain package import once live)
// ---------------------------------------------------------------------------

// testReminder is the reminder response shape the handler should return.
type testReminder struct {
	ID         string  `json:"id"`
	HabitID    string  `json:"habitId"`
	UserID     string  `json:"userId"`
	RemindAt   string  `json:"remindAt"`
	DaysOfWeek []int16 `json:"daysOfWeek"`
	Enabled    bool    `json:"enabled"`
}

// testNotification is the notification response shape the handler should return.
type testNotification struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

// createReminderRequest mirrors the expected POST body.
type createReminderRequest struct {
	RemindAt   string  `json:"remindAt"`   // RFC3339 or HH:MM
	DaysOfWeek []int16 `json:"daysOfWeek"` // 0=Sun..6=Sat
}

// ---------------------------------------------------------------------------
// Mock service layer
// ---------------------------------------------------------------------------

// reminderService is the interface the handler depends on.
// The real implementation lives in internal/service — mirrored here so the
// test compiles without that package.
type reminderService interface {
	CreateReminder(userID, habitID string, remindAt time.Time, daysOfWeek []int16) (testReminder, error)
	ListNotifications(userID string) ([]testNotification, error)
}

// mockReminderService is an in-memory stub.
type mockReminderService struct {
	mu            sync.Mutex
	reminders     []testReminder
	notifications []testNotification
}

func (m *mockReminderService) CreateReminder(
	userID, habitID string,
	remindAt time.Time,
	daysOfWeek []int16,
) (testReminder, error) {
	rem := testReminder{
		ID:         uuid.New().String(),
		HabitID:    habitID,
		UserID:     userID,
		RemindAt:   remindAt.Format(time.RFC3339),
		DaysOfWeek: daysOfWeek,
		Enabled:    true,
	}
	m.mu.Lock()
	m.reminders = append(m.reminders, rem)
	m.mu.Unlock()
	return rem, nil
}

func (m *mockReminderService) ListNotifications(userID string) ([]testNotification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []testNotification
	for _, n := range m.notifications {
		if n.UserID == userID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *mockReminderService) seedNotification(n testNotification) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, n)
}

// ---------------------------------------------------------------------------
// Handler (local stub — replace with real api.ReminderHandler import once live)
//
// This stub satisfies the handler contract so the test file compiles and
// exercises the HTTP plumbing before the real handler is written.
// ---------------------------------------------------------------------------

type reminderHandler struct {
	svc reminderService
}

func newReminderHandler(svc reminderService) *reminderHandler {
	return &reminderHandler{svc: svc}
}

func (h *reminderHandler) CreateReminder(w http.ResponseWriter, r *http.Request) {
	habitID := chi.URLParam(r, "habitId")
	userID := appmiddleware.UserIDFromContext(r.Context())

	var req createReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		testWriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RemindAt == "" {
		testWriteError(w, http.StatusBadRequest, "remindAt is required")
		return
	}

	remindAt, err := time.Parse(time.RFC3339, req.RemindAt)
	if err != nil {
		// Try HH:MM fallback.
		remindAt, err = time.Parse("15:04", req.RemindAt)
		if err != nil {
			testWriteError(w, http.StatusBadRequest, "remindAt must be RFC3339 or HH:MM")
			return
		}
	}

	rem, err := h.svc.CreateReminder(userID, habitID, remindAt, req.DaysOfWeek)
	if err != nil {
		testWriteError(w, http.StatusInternalServerError, "failed to create reminder")
		return
	}
	testWriteJSON(w, http.StatusCreated, rem)
}

func (h *reminderHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.UserIDFromContext(r.Context())
	notifications, err := h.svc.ListNotifications(userID)
	if err != nil {
		testWriteError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}
	if notifications == nil {
		notifications = []testNotification{}
	}
	testWriteJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

func testWriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func testWriteError(w http.ResponseWriter, status int, msg string) {
	testWriteJSON(w, status, map[string]string{"error": msg})
}

// ---------------------------------------------------------------------------
// Test router builder
// ---------------------------------------------------------------------------

const testJWTSecret = "test-secret-key-for-tests-only"

// buildTestRouter wires the Chi router with auth middleware and the reminder
// handler endpoints — mirrors the pattern in internal/api/router.go.
func buildTestRouter(h *reminderHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(appmiddleware.Authenticate(testJWTSecret))

		r.Post("/api/habits/{habitId}/reminders", h.CreateReminder)
		r.Get("/api/notifications", h.ListNotifications)
	})

	return r
}

// mintJWT creates a signed JWT for the given userID using the test secret.
// This replicates the token shape produced by internal/api.AuthHandler.generateToken.
func mintJWT(userID string) string {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		panic("mintJWT: " + err.Error())
	}
	return signed
}

// authHeader returns the Bearer header value for the given userID.
func authHeader(userID string) string {
	return "Bearer " + mintJWT(userID)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCreateReminder_201 verifies that a valid POST request returns 201 with
// the created reminder JSON, including the correct habitId and userId.
func TestCreateReminder_201(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	userID := uuid.New().String()
	habitID := uuid.New().String()

	body := createReminderRequest{
		RemindAt:   "2026-05-01T09:00:00Z",
		DaysOfWeek: []int16{1, 3, 5},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/habits/"+habitID+"/reminders",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(userID))

	// Chi needs URL params embedded — use WithContext to add chi route params.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("habitId", habitID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got testReminder
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if got.ID == "" {
		t.Error("expected non-empty ID in response")
	}
	if got.HabitID != habitID {
		t.Errorf("expected habitId %q, got %q", habitID, got.HabitID)
	}
	if got.UserID != userID {
		t.Errorf("expected userId %q, got %q", userID, got.UserID)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true for newly created reminder")
	}
}

// TestCreateReminder_MissingRemindAt_400 verifies that omitting remindAt
// returns a 400 Bad Request.
func TestCreateReminder_MissingRemindAt_400(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	userID := uuid.New().String()
	habitID := uuid.New().String()

	body := map[string]any{"daysOfWeek": []int{1, 3}} // no remindAt
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/habits/"+habitID+"/reminders",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(userID))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("habitId", habitID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestCreateReminder_Unauthenticated_401 verifies that requests without a
// valid JWT are rejected with 401.
func TestCreateReminder_Unauthenticated_401(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	habitID := uuid.New().String()
	body := createReminderRequest{RemindAt: "2026-05-01T09:00:00Z", DaysOfWeek: []int16{1}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/habits/"+habitID+"/reminders",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestGetNotifications_200_OnlyAuthedUser verifies that GET /api/notifications
// returns 200 with only the authenticated user's notifications.
func TestGetNotifications_200_OnlyAuthedUser(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	userA := uuid.New().String()
	userB := uuid.New().String()

	// Seed notifications for two users.
	svc.seedNotification(testNotification{
		ID:     uuid.New().String(),
		UserID: userA,
		Type:   "reminder",
		Title:  "Wake up",
		Body:   "Time for your morning habit",
		Read:   false,
	})
	svc.seedNotification(testNotification{
		ID:     uuid.New().String(),
		UserID: userA,
		Type:   "reminder",
		Title:  "Evening",
		Body:   "Evening habit reminder",
		Read:   false,
	})
	svc.seedNotification(testNotification{
		ID:     uuid.New().String(),
		UserID: userB, // different user — must NOT appear in userA's response
		Type:   "reminder",
		Title:  "Other user",
		Body:   "Should not be visible",
		Read:   false,
	})

	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	req.Header.Set("Authorization", authHeader(userA))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Notifications []testNotification `json:"notifications"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(resp.Notifications) != 2 {
		t.Errorf("expected 2 notifications for userA, got %d", len(resp.Notifications))
	}
	for _, n := range resp.Notifications {
		if n.UserID != userA {
			t.Errorf("notification belongs to wrong user %q (expected %q)", n.UserID, userA)
		}
	}
}

// TestGetNotifications_200_EmptySliceForNewUser verifies that a user with no
// notifications receives 200 with an empty array (not null or 404).
func TestGetNotifications_200_EmptySliceForNewUser(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	userID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	req.Header.Set("Authorization", authHeader(userID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Notifications []testNotification `json:"notifications"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Notifications == nil {
		t.Error("expected non-nil (empty) notifications slice, got null")
	}
	if len(resp.Notifications) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(resp.Notifications))
	}
}

// TestGetNotifications_Unauthenticated_401 verifies that an unauthenticated
// request to the notifications endpoint is rejected with 401.
func TestGetNotifications_Unauthenticated_401(t *testing.T) {
	t.Parallel()

	svc := &mockReminderService{}
	handler := newReminderHandler(svc)
	router := buildTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	// No Authorization header.

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
