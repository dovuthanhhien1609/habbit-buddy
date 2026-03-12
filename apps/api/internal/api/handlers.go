package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/habit-buddy/api/internal/middleware"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/service"
	"github.com/habit-buddy/api/internal/ws"
)

type HabitHandler struct {
	habitService *service.HabitService
	bridge       *ws.EventBridge
}

func NewHabitHandler(habitService *service.HabitService, bridge *ws.EventBridge) *HabitHandler {
	return &HabitHandler{habitService: habitService, bridge: bridge}
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
	writeJSON(w, http.StatusOK, map[string]interface{}{"habits": habits})
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

	// Get the habit name for the WS event.
	habits, _ := h.habitService.GetHabits(userID)
	habitName := ""
	for _, hb := range habits {
		if hb.ID == habitID {
			habitName = hb.Name
			break
		}
	}

	// Publish realtime event via Redis pub/sub so all API instances can
	// fan-out to their local WebSocket clients for this user.
	if err := h.bridge.Publish(userID, model.WSEvent{
		Type: "HABIT_COMPLETED",
		Payload: model.HabitCompletedPayload{
			HabitID:     habitID,
			HabitName:   habitName,
			Streak:      streak,
			CompletedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}); err != nil {
		log.Printf("ws: publish HABIT_COMPLETED: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
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

	if err := h.bridge.Publish(userID, model.WSEvent{
		Type: "HABIT_UNDONE",
		Payload: model.HabitUndonePayload{
			HabitID: habitID,
			Streak:  streak,
		},
	}); err != nil {
		log.Printf("ws: publish HABIT_UNDONE: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
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
		results = append(results, habitAnalytic{
			Habit:   hb,
			History: stats.History,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"habits": results})
}

// ---- shared helpers ----

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
