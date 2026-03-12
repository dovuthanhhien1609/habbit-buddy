package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo  *repository.UserRepository
	jwtSecret string
}

func NewAuthHandler(userRepo *repository.UserRepository, jwtSecret string) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwtSecret: jwtSecret}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email, username, and password are required")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	emailExists, err := h.userRepo.EmailExists(req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if emailExists {
		writeError(w, http.StatusConflict, "email already in use")
		return
	}

	usernameExists, err := h.userRepo.UsernameExists(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if usernameExists {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.userRepo.Create(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.generateToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.generateToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) generateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 day expiry
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
