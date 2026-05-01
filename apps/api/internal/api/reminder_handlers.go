package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/habit-buddy/api/internal/logger"
	"github.com/habit-buddy/api/internal/middleware"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/repository"
)

// ReminderHandler serves CRUD endpoints for habit reminders and notifications.
type ReminderHandler struct {
	reminderRepo     *repository.ReminderRepository
	notificationRepo *repository.NotificationRepository
	log              *slog.Logger
}

func NewReminderHandler(
	reminderRepo *repository.ReminderRepository,
	notificationRepo *repository.NotificationRepository,
) *ReminderHandler {
	return &ReminderHandler{
		reminderRepo:     reminderRepo,
		notificationRepo: notificationRepo,
		log:              logger.L.With("component", "reminder_handler"),
	}
}

// ListReminders returns all reminders for the given habit, owned by the caller.
//
//	GET /api/habits/{habitId}/reminders
func (h *ReminderHandler) ListReminders(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "habitId")

	reminders, err := h.reminderRepo.GetRemindersByHabit(habitID, userID)
	if err != nil {
		h.log.Error("list reminders failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list reminders")
		return
	}
	if reminders == nil {
		reminders = []model.Reminder{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminders": reminders})
}

// CreateReminder adds a new reminder for a habit.
//
//	POST /api/habits/{habitId}/reminders
func (h *ReminderHandler) CreateReminder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "habitId")

	var req model.CreateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RemindAt == "" {
		writeError(w, http.StatusBadRequest, "remindAt is required (HH:MM)")
		return
	}
	if len(req.DaysOfWeek) == 0 {
		writeError(w, http.StatusBadRequest, "daysOfWeek must contain at least one day")
		return
	}

	now := time.Now().UTC()
	rem := &model.Reminder{
		ID:         uuid.New().String(),
		HabitID:    habitID,
		UserID:     userID,
		RemindAt:   req.RemindAt,
		DaysOfWeek: req.DaysOfWeek,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.reminderRepo.CreateReminder(rem); err != nil {
		h.log.Error("create reminder failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create reminder")
		return
	}
	writeJSON(w, http.StatusCreated, rem)
}

// UpdateReminder modifies an existing reminder.
//
//	PUT /api/habits/{habitId}/reminders/{reminderId}
func (h *ReminderHandler) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	reminderID := chi.URLParam(r, "reminderId")

	existing, err := h.reminderRepo.GetByID(reminderID, userID)
	if err != nil {
		h.log.Error("get reminder failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch reminder")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "reminder not found")
		return
	}

	var req model.UpdateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RemindAt != nil {
		existing.RemindAt = *req.RemindAt
	}
	if len(req.DaysOfWeek) > 0 {
		existing.DaysOfWeek = req.DaysOfWeek
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := h.reminderRepo.UpdateReminder(existing); err != nil {
		h.log.Error("update reminder failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update reminder")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

// DeleteReminder removes a reminder.
//
//	DELETE /api/habits/{habitId}/reminders/{reminderId}
func (h *ReminderHandler) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	reminderID := chi.URLParam(r, "reminderId")

	if err := h.reminderRepo.DeleteReminder(reminderID, userID); err != nil {
		h.log.Error("delete reminder failed", "error", err)
		writeError(w, http.StatusNotFound, "reminder not found or forbidden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListNotifications returns all unread notifications for the authenticated user.
//
//	GET /api/notifications
func (h *ReminderHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	notifications, err := h.notificationRepo.ListUnread(userID)
	if err != nil {
		h.log.Error("list notifications failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}
	if notifications == nil {
		notifications = []model.Notification{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

// MarkNotificationRead marks a single notification as read.
//
//	POST /api/notifications/{id}/read
func (h *ReminderHandler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	notifID := chi.URLParam(r, "id")

	if err := h.notificationRepo.MarkRead(notifID, userID); err != nil {
		h.log.Error("mark notification read failed", "error", err)
		writeError(w, http.StatusNotFound, "notification not found or forbidden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}
