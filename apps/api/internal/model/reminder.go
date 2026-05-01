package model

import "time"

// Reminder represents a scheduled reminder for a habit.
type Reminder struct {
	ID          string    `json:"id"`
	HabitID     string    `json:"habitId"`
	UserID      string    `json:"userId"`
	RemindAt    string    `json:"remindAt"`   // HH:MM (24-hour UTC)
	DaysOfWeek  []int     `json:"daysOfWeek"` // 0=Sunday … 6=Saturday
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Notification is a persisted in-app notification row.
type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReminderPayload is embedded in the Event.Payload for reminder events.
type ReminderPayload struct {
	HabitID        string `json:"habitId"`
	HabitName      string `json:"habitName"`
	NotificationID string `json:"notificationId"`
	Title          string `json:"title"`
	Body           string `json:"body"`
}

// Request / response helpers

type CreateReminderRequest struct {
	RemindAt   string `json:"remindAt"`   // "HH:MM"
	DaysOfWeek []int  `json:"daysOfWeek"` // [0..6]
}

type UpdateReminderRequest struct {
	RemindAt   *string `json:"remindAt"`
	DaysOfWeek []int   `json:"daysOfWeek"`
	Enabled    *bool   `json:"enabled"`
}

// Event type constant for reminder notifications.
const EventReminder = "reminder"
