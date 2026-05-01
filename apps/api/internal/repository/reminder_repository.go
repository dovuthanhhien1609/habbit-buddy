package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/habit-buddy/api/internal/model"
	"github.com/lib/pq"
)

// ReminderRepository handles persistence for habit_reminders.
type ReminderRepository struct {
	db *sql.DB
}

func NewReminderRepository(db *sql.DB) *ReminderRepository {
	return &ReminderRepository{db: db}
}

// CreateReminder inserts a new reminder for the given habit.
func (r *ReminderRepository) CreateReminder(rem *model.Reminder) error {
	query := `
		INSERT INTO habit_reminders (id, habit_id, user_id, remind_at, days_of_week, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4::time, $5, $6, $7, $8)`
	_, err := r.db.Exec(query,
		rem.ID,
		rem.HabitID,
		rem.UserID,
		rem.RemindAt,
		pq.Array(rem.DaysOfWeek),
		rem.Enabled,
		rem.CreatedAt,
		rem.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("reminder.Create: %w", err)
	}
	return nil
}

// GetRemindersByHabit returns all reminders for a habit, checking ownership.
func (r *ReminderRepository) GetRemindersByHabit(habitID, userID string) ([]model.Reminder, error) {
	query := `
		SELECT id, habit_id, user_id, remind_at::text, days_of_week, enabled, created_at, updated_at
		FROM habit_reminders
		WHERE habit_id = $1 AND user_id = $2
		ORDER BY created_at ASC`
	rows, err := r.db.Query(query, habitID, userID)
	if err != nil {
		return nil, fmt.Errorf("reminder.GetByHabit: %w", err)
	}
	defer rows.Close()
	return scanReminders(rows)
}

// GetRemindersByUser returns all reminders for a user.
func (r *ReminderRepository) GetRemindersByUser(userID string) ([]model.Reminder, error) {
	query := `
		SELECT id, habit_id, user_id, remind_at::text, days_of_week, enabled, created_at, updated_at
		FROM habit_reminders
		WHERE user_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("reminder.GetByUser: %w", err)
	}
	defer rows.Close()
	return scanReminders(rows)
}

// GetEnabledReminders returns all enabled reminders (used by the scheduler).
func (r *ReminderRepository) GetEnabledReminders() ([]model.Reminder, error) {
	query := `
		SELECT id, habit_id, user_id, remind_at::text, days_of_week, enabled, created_at, updated_at
		FROM habit_reminders
		WHERE enabled = TRUE
		ORDER BY user_id, habit_id`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("reminder.GetEnabled: %w", err)
	}
	defer rows.Close()
	return scanReminders(rows)
}

// UpdateReminder updates a reminder's fields, verifying ownership.
func (r *ReminderRepository) UpdateReminder(rem *model.Reminder) error {
	query := `
		UPDATE habit_reminders
		SET remind_at = $1::time, days_of_week = $2, enabled = $3, updated_at = $4
		WHERE id = $5 AND user_id = $6`
	res, err := r.db.Exec(query,
		rem.RemindAt,
		pq.Array(rem.DaysOfWeek),
		rem.Enabled,
		rem.UpdatedAt,
		rem.ID,
		rem.UserID,
	)
	if err != nil {
		return fmt.Errorf("reminder.Update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("reminder.Update: not found or forbidden")
	}
	return nil
}

// DeleteReminder removes a reminder, verifying ownership.
func (r *ReminderRepository) DeleteReminder(reminderID, userID string) error {
	res, err := r.db.Exec(
		`DELETE FROM habit_reminders WHERE id = $1 AND user_id = $2`,
		reminderID, userID,
	)
	if err != nil {
		return fmt.Errorf("reminder.Delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("reminder.Delete: not found or forbidden")
	}
	return nil
}

// GetByID returns a single reminder by ID, checking ownership.
func (r *ReminderRepository) GetByID(reminderID, userID string) (*model.Reminder, error) {
	query := `
		SELECT id, habit_id, user_id, remind_at::text, days_of_week, enabled, created_at, updated_at
		FROM habit_reminders
		WHERE id = $1 AND user_id = $2`
	row := r.db.QueryRow(query, reminderID, userID)
	rem, err := scanReminder(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reminder.GetByID: %w", err)
	}
	return rem, nil
}

// ---- helpers ----

func scanReminders(rows *sql.Rows) ([]model.Reminder, error) {
	var reminders []model.Reminder
	for rows.Next() {
		var rem model.Reminder
		var days []int64
		if err := rows.Scan(
			&rem.ID,
			&rem.HabitID,
			&rem.UserID,
			&rem.RemindAt,
			pq.Array(&days),
			&rem.Enabled,
			&rem.CreatedAt,
			&rem.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rem.DaysOfWeek = toIntSlice(days)
		reminders = append(reminders, rem)
	}
	return reminders, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReminder(row rowScanner) (*model.Reminder, error) {
	var rem model.Reminder
	var days []int64
	err := row.Scan(
		&rem.ID,
		&rem.HabitID,
		&rem.UserID,
		&rem.RemindAt,
		pq.Array(&days),
		&rem.Enabled,
		&rem.CreatedAt,
		&rem.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	rem.DaysOfWeek = toIntSlice(days)
	return &rem, nil
}

// toIntSlice converts []int64 (from pq.Array) to []int.
func toIntSlice(in []int64) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

// reminderUpdatedNow returns current UTC time, used for updated_at stamps.
func reminderUpdatedNow() time.Time {
	return time.Now().UTC()
}

// Ensure reminderUpdatedNow is referenced to avoid unused-import issues.
var _ = reminderUpdatedNow
