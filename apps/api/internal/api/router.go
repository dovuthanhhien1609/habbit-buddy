package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/habit-buddy/api/internal/middleware"
	"github.com/habit-buddy/api/internal/ws"
)

func NewRouter(
	authHandler *AuthHandler,
	habitHandler *HabitHandler,
	reminderHandler *ReminderHandler,
	hub *ws.Hub,
	jwtSecret string,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(corsMiddleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth routes (public)
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
	})

	// WebSocket endpoint
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query param for WS (headers not easily set in browser WS API)
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		userID := validateWSToken(token, jwtSecret)
		if userID == "" {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		hub.ServeWS(w, r, userID)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(appmiddleware.Authenticate(jwtSecret))

		r.Get("/api/dashboard", habitHandler.GetDashboard)
		r.Get("/api/analytics", habitHandler.GetAnalytics)

		r.Route("/api/habits", func(r chi.Router) {
			r.Get("/", habitHandler.ListHabits)
			r.Post("/", habitHandler.CreateHabit)

			r.Route("/{id}", func(r chi.Router) {
				r.Patch("/", habitHandler.UpdateHabit)
				r.Delete("/", habitHandler.ArchiveHabit)
				r.Post("/complete", habitHandler.CompleteHabit)
				r.Delete("/complete", habitHandler.UndoCompletion)
				r.Get("/stats", habitHandler.GetStats)
			})

			// Reminder endpoints — nested under habit
			r.Route("/{habitId}/reminders", func(r chi.Router) {
				r.Get("/", reminderHandler.ListReminders)
				r.Post("/", reminderHandler.CreateReminder)
				r.Put("/{reminderId}", reminderHandler.UpdateReminder)
				r.Delete("/{reminderId}", reminderHandler.DeleteReminder)
			})
		})

		// Notification endpoints
		r.Route("/api/notifications", func(r chi.Router) {
			r.Get("/", reminderHandler.ListNotifications)
			r.Post("/{id}/read", reminderHandler.MarkNotificationRead)
		})
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
