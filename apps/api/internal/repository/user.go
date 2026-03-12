package repository

import (
	"database/sql"
	"fmt"

	"github.com/habit-buddy/api/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	query := `
		INSERT INTO users (id, email, username, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(query,
		user.ID, user.Email, user.Username, user.PasswordHash, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("user.Create: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(email string) (*model.User, error) {
	user := &model.User{}
	query := `SELECT id, email, username, password_hash, created_at FROM users WHERE email = $1`
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user.GetByEmail: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByID(id string) (*model.User, error) {
	user := &model.User{}
	query := `SELECT id, email, username, password_hash, created_at FROM users WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user.GetByID: %w", err)
	}
	return user, nil
}

func (r *UserRepository) EmailExists(email string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count)
	return count > 0, err
}

func (r *UserRepository) UsernameExists(username string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = $1`, username).Scan(&count)
	return count > 0, err
}
