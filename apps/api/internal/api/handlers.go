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
	"github.com/habit-buddy/api/internal/service"
	"github.com/habit-buddy/api/internal/ws"
)

const producer = "api"

type HabitHandler struct {
	habitService *service.HabitService
	bridge       *ws.EventBridge
	log          *slog.Logger
}

func NewHabitHandler(habitService *service.HabitService, bridge *ws.EventBridge) *HabitHandler {
	return &HabitHandler{
		habitService: habitService,
		bridge:       bridge,
		log:          logger.L.With("component", "habit_handler"),
	}
}

func (h *HabitHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	dashboard, err := h.habitService.GetDashboard(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (h *HabitHandler) ListHabits(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habits, err := h.habitService.GetHabits(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list habits")
		return
	}
	if habits == nil {
		habits = []model.Habit{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"habits": habits})
}

func (h *HabitHandler) CreateHabit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req model.CreateHabitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	habit, err := h.habitService.CreateHabit(userID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create habit")
		return
	}
	writeJSON(w, http.StatusCreated, habit)
}

func (h *HabitHandler) UpdateHabit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "id")

	var req model.UpdateHabitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	habit, err := h.habitService.UpdateHabit(habitID, userID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update habit")
		return
	}
	if habit == nil {
		writeError(w, http.StatusNotFound, "habit not found")
		return
	}
	writeJSON(w, http.StatusOK, habit)
}

func (h *HabitHandler) ArchiveHabit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "id")

	if err := h.habitService.ArchiveHabit(habitID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive habit")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (h *HabitHandler) CompleteHabit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "id")

	streak, milestone, err := h.habitService.CompleteHabit(habitID, userID)
	if err != nil {
		if err.Error() == "habit not found" {
			writeError(w, http.StatusNotFound, "habit not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to complete habit")
		return
	}

	// Get the habit name for the event payload.
	habits, _ := h.habitService.GetHabits(userID)
	habitName := ""
	for _, hb := range habits {
		if hb.ID == habitID {
			habitName = hb.Name
			break
		}
	}

	now := time.Now().UTC()
	eventID := uuid.New().String()

	h.log.Info("publishing event",
		"event_id", eventID,
		"event_type", model.EventHabitCompleted,
		"habit_id", habitID,
		"user_id", userID,
	)

	payload, _ := json.Marshal(model.HabitCompletedPayload{
		HabitID:     habitID,
		HabitName:   habitName,
		Streak:      streak,
		CompletedAt: now.Format(time.RFC3339),
	})

	if err := h.bridge.Publish(userID, model.Event{
		EventID:   eventID,
		EventType: model.EventHabitCompleted,
		Timestamp: now,
		Producer:  producer,
		Payload:   payload,
	}); err != nil {
		h.log.Error("publish failed", "event_id", eventID, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"habitId":        habitID,
		"streak":         streak,
		"completedToday": true,
		"milestone":      milestone,
	})
}

func (h *HabitHandler) UndoCompletion(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "id")

	streak, err := h.habitService.UndoCompletion(habitID, userID)
	if err != nil {
		if err.Error() == "habit not found" {
			writeError(w, http.StatusNotFound, "habit not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to undo completion")
		return
	}

	eventID := uuid.New().String()

	h.log.Info("publishing event",
		"event_id", eventID,
		"event_type", model.EventHabitUndone,
		"habit_id", habitID,
		"user_id", userID,
	)

	payload, _ := json.Marshal(model.HabitUndonePayload{
		HabitID: habitID,
		Streak:  streak,
	})

	if err := h.bridge.Publish(userID, model.Event{
		EventID:   eventID,
		EventType: model.EventHabitUndone,
		Timestamp: time.Now().UTC(),
		Producer:  producer,
		Payload:   payload,
	}); err != nil {
		h.log.Error("publish failed", "event_id", eventID, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"habitId":        habitID,
		"streak":         streak,
		"completedToday": false,
	})
}

func (h *HabitHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habitID := chi.URLParam(r, "id")

	stats, err := h.habitService.GetHabitStats(habitID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	if stats == nil {
		writeError(w, http.StatusNotFound, "habit not found")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *HabitHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	habits, err := h.habitService.GetHabits(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}

	type habitAnalytic struct {
		model.Habit
		History []string `json:"history"`
	}

	results := make([]habitAnalytic, 0, len(habits))
	for _, hb := range habits {
		stats, err := h.habitService.GetHabitStats(hb.ID, userID)
		if err != nil {
			continue
		}
		history := stats.History
		if history == nil {
			history = []string{}
		}
		results = append(results, habitAnalytic{
			Habit:   hb,
			History: history,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"habits": results})
}

// ---- shared helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
