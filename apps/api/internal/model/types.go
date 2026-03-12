package model

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Habit struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Color          string    `json:"color"`
	Icon           string    `json:"icon"`
	IsActive       bool      `json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
	ArchivedAt     *time.Time `json:"archivedAt,omitempty"`
	// Computed at read time
	Streak         int  `json:"streak"`
	CompletedToday bool `json:"completedToday"`
}

type HabitCompletion struct {
	ID            string    `json:"id"`
	HabitID       string    `json:"habitId"`
	UserID        string    `json:"userId"`
	CompletedDate string    `json:"completedDate"`
	CompletedAt   time.Time `json:"completedAt"`
}

type DashboardResponse struct {
	Date           string  `json:"date"`
	CompletedCount int     `json:"completedCount"`
	TotalCount     int     `json:"totalCount"`
	CompletionRate float64 `json:"completionRate"`
	Habits         []Habit `json:"habits"`
}

type HabitStats struct {
	HabitID        string   `json:"habitId"`
	HabitName      string   `json:"habitName"`
	Streak         int      `json:"streak"`
	LongestStreak  int      `json:"longestStreak"`
	TotalCompleted int      `json:"totalCompleted"`
	Rate30Day      float64  `json:"rate30Day"`
	History        []string `json:"history"` // dates completed in last 30 days
}

type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type HabitCompletedPayload struct {
	HabitID     string `json:"habitId"`
	HabitName   string `json:"habitName"`
	Streak      int    `json:"streak"`
	CompletedAt string `json:"completedAt"`
}

type HabitUndonePayload struct {
	HabitID string `json:"habitId"`
	Streak  int    `json:"streak"`
}

// Request bodies
type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateHabitRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
}

type UpdateHabitRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Icon        *string `json:"icon"`
}
