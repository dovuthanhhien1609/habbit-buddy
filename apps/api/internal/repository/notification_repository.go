package repository

import (
	"database/sql"
	"fmt"

	"github.com/habit-buddy/api/internal/model"
)

// NotificationRepository handles persistence for notifications.
type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// CreateNotification inserts a new notification row.
func (r *NotificationRepository) CreateNotification(n *model.Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, type, title, body, read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(query,
		n.ID, n.UserID, n.Type, n.Title, n.Body, n.Read, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("notification.Create: %w", err)
	}
	return nil
}

// ListUnread returns all unread notifications for a user, newest first.
func (r *NotificationRepository) ListUnread(userID string) ([]model.Notification, error) {
	query := `
		SELECT id, user_id, type, title, body, read, created_at
		FROM notifications
		WHERE user_id = $1 AND read = FALSE
		ORDER BY created_at DESC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("notification.ListUnread: %w", err)
	}
	defer rows.Close()

	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

// MarkRead sets read=true on the notification, verifying ownership.
func (r *NotificationRepository) MarkRead(notificationID, userID string) error {
	res, err := r.db.Exec(
		`UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	)
	if err != nil {
		return fmt.Errorf("notification.MarkRead: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("notification.MarkRead: not found or forbidden")
	}
	return nil
}

// GetByID returns a notification by ID for ownership checks.
func (r *NotificationRepository) GetByID(notificationID, userID string) (*model.Notification, error) {
	var n model.Notification
	query := `
		SELECT id, user_id, type, title, body, read, created_at
		FROM notifications WHERE id = $1 AND user_id = $2`
	err := r.db.QueryRow(query, notificationID, userID).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Read, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notification.GetByID: %w", err)
	}
	return &n, nil
}
