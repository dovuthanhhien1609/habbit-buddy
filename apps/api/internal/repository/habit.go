package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/habit-buddy/api/internal/model"
)

type HabitRepository struct {
	db *sql.DB
}

func NewHabitRepository(db *sql.DB) *HabitRepository {
	return &HabitRepository{db: db}
}

// GetActiveByUserID returns all active habits for a user.
func (r *HabitRepository) GetActiveByUserID(userID string) ([]model.Habit, error) {
	query := `
		SELECT id, user_id, name, description, color, icon, is_active, created_at
		FROM habits
		WHERE user_id = $1 AND is_active = TRUE
		ORDER BY created_at ASC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("habit.GetActiveByUserID: %w", err)
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		var h model.Habit
		if err := rows.Scan(&h.ID, &h.UserID, &h.Name, &h.Description,
			&h.Color, &h.Icon, &h.IsActive, &h.CreatedAt); err != nil {
			return nil, err
		}
		habits = append(habits, h)
	}
	return habits, rows.Err()
}

// GetByID returns a habit by ID, checking ownership.
func (r *HabitRepository) GetByID(habitID, userID string) (*model.Habit, error) {
	var h model.Habit
	query := `
		SELECT id, user_id, name, description, color, icon, is_active, created_at
		FROM habits WHERE id = $1 AND user_id = $2`
	err := r.db.QueryRow(query, habitID, userID).Scan(
		&h.ID, &h.UserID, &h.Name, &h.Description,
		&h.Color, &h.Icon, &h.IsActive, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("habit.GetByID: %w", err)
	}
	return &h, nil
}

// Create inserts a new habit.
func (r *HabitRepository) Create(h *model.Habit) error {
	query := `
		INSERT INTO habits (id, user_id, name, description, color, icon, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(query,
		h.ID, h.UserID, h.Name, h.Description, h.Color, h.Icon, h.IsActive, h.CreatedAt)
	if err != nil {
		return fmt.Errorf("habit.Create: %w", err)
	}
	return nil
}

// Update updates mutable fields of a habit.
func (r *HabitRepository) Update(h *model.Habit) error {
	query := `
		UPDATE habits SET name=$1, description=$2, color=$3, icon=$4
		WHERE id=$5 AND user_id=$6`
	_, err := r.db.Exec(query, h.Name, h.Description, h.Color, h.Icon, h.ID, h.UserID)
	if err != nil {
		return fmt.Errorf("habit.Update: %w", err)
	}
	return nil
}

// Archive soft-deletes a habit.
func (r *HabitRepository) Archive(habitID, userID string) error {
	now := time.Now().UTC()
	query := `UPDATE habits SET is_active=FALSE, archived_at=$1 WHERE id=$2 AND user_id=$3`
	_, err := r.db.Exec(query, now, habitID, userID)
	if err != nil {
		return fmt.Errorf("habit.Archive: %w", err)
	}
	return nil
}

// AddCompletion inserts a completion record. Returns false if already exists.
func (r *HabitRepository) AddCompletion(habitID, userID, date string) (bool, error) {
	id := newUUID()
	now := time.Now().UTC()
	query := `
		INSERT INTO habit_completions (id, habit_id, user_id, completed_date, completed_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (habit_id, completed_date) DO NOTHING`
	res, err := r.db.Exec(query, id, habitID, userID, date, now)
	if err != nil {
		return false, fmt.Errorf("habit.AddCompletion: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// RemoveCompletion deletes today's completion. Returns false if nothing was deleted.
func (r *HabitRepository) RemoveCompletion(habitID, userID, date string) (bool, error) {
	query := `DELETE FROM habit_completions WHERE habit_id=$1 AND user_id=$2 AND completed_date=$3`
	res, err := r.db.Exec(query, habitID, userID, date)
	if err != nil {
		return false, fmt.Errorf("habit.RemoveCompletion: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// IsCompletedOnDate checks if a habit was completed on a given date.
func (r *HabitRepository) IsCompletedOnDate(habitID, date string) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM habit_completions WHERE habit_id=$1 AND completed_date=$2`,
		habitID, date).Scan(&count)
	return count > 0, err
}

// GetCompletionDates returns all completed dates for a habit in the last N days.
func (r *HabitRepository) GetCompletionDates(habitID string, days int) ([]string, error) {
	query := `
		SELECT completed_date::text
		FROM habit_completions
		WHERE habit_id = $1
		  AND completed_date >= CURRENT_DATE - ($2 || ' days')::interval
		ORDER BY completed_date DESC`
	rows, err := r.db.Query(query, habitID, days)
	if err != nil {
		return nil, fmt.Errorf("habit.GetCompletionDates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// CalculateStreak computes the current streak from completion history.
// A streak is the number of consecutive days ending today (or yesterday if not yet done today).
func (r *HabitRepository) CalculateStreak(habitID string) (int, error) {
	query := `
		SELECT completed_date::text
		FROM habit_completions
		WHERE habit_id = $1
		ORDER BY completed_date DESC
		LIMIT 365`
	rows, err := r.db.Query(query, habitID)
	if err != nil {
		return 0, fmt.Errorf("habit.CalculateStreak: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return computeStreak(dates), nil
}

// computeStreak calculates streak from a list of dates (most recent first, YYYY-MM-DD).
func computeStreak(dates []string) int {
	if len(dates) == 0 {
		return 0
	}

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Streak must start from today or yesterday.
	if dates[0] != today && dates[0] != yesterday {
		return 0
	}

	streak := 0
	prev := dates[0]
	for _, d := range dates {
		prevT, _ := time.Parse("2006-01-02", prev)
		curT, _ := time.Parse("2006-01-02", d)
		diff := prevT.Sub(curT)
		if streak == 0 {
			// First iteration
			streak = 1
			prev = d
			continue
		}
		if diff == 24*time.Hour {
			streak++
			prev = d
		} else {
			break
		}
	}
	return streak
}

// GetTotalCompletions counts all completions ever for a habit.
func (r *HabitRepository) GetTotalCompletions(habitID string) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM habit_completions WHERE habit_id=$1`, habitID).Scan(&count)
	return count, err
}

// newUUID is a simple wrapper — we use google/uuid in service layer,
// but keep repository import-clean.
func newUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()) // placeholder, overridden by caller
}
