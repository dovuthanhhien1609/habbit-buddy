package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/redisclient"
	"github.com/habit-buddy/api/internal/repository"
)

type HabitService struct {
	habitRepo *repository.HabitRepository
	redis     *redisclient.Client
}

func NewHabitService(habitRepo *repository.HabitRepository, redis *redisclient.Client) *HabitService {
	return &HabitService{habitRepo: habitRepo, redis: redis}
}

// CreateHabit creates a new habit and invalidates the user's cache.
func (s *HabitService) CreateHabit(userID string, req *model.CreateHabitRequest) (*model.Habit, error) {
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	if req.Icon == "" {
		req.Icon = "check"
	}

	h := &model.Habit{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.habitRepo.Create(h); err != nil {
		return nil, err
	}

	s.invalidateHabitCache(userID)
	return h, nil
}

// UpdateHabit updates a habit and invalidates cache.
func (s *HabitService) UpdateHabit(habitID, userID string, req *model.UpdateHabitRequest) (*model.Habit, error) {
	h, err := s.habitRepo.GetByID(habitID, userID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}

	if req.Name != nil {
		h.Name = *req.Name
	}
	if req.Description != nil {
		h.Description = *req.Description
	}
	if req.Color != nil {
		h.Color = *req.Color
	}
	if req.Icon != nil {
		h.Icon = *req.Icon
	}

	if err := s.habitRepo.Update(h); err != nil {
		return nil, err
	}

	s.invalidateHabitCache(userID)
	return h, nil
}

// ArchiveHabit soft-deletes a habit.
func (s *HabitService) ArchiveHabit(habitID, userID string) error {
	if err := s.habitRepo.Archive(habitID, userID); err != nil {
		return err
	}
	s.invalidateHabitCache(userID)
	return nil
}

// GetDashboard returns today's habits enriched with streak + completion status.
func (s *HabitService) GetDashboard(userID string) (*model.DashboardResponse, error) {
	habits, err := s.getEnrichedHabits(userID)
	if err != nil {
		return nil, err
	}

	completed := 0
	for _, h := range habits {
		if h.CompletedToday {
			completed++
		}
	}

	rate := 0.0
	if len(habits) > 0 {
		rate = float64(completed) / float64(len(habits))
	}

	return &model.DashboardResponse{
		Date:           time.Now().UTC().Format("2006-01-02"),
		CompletedCount: completed,
		TotalCount:     len(habits),
		CompletionRate: rate,
		Habits:         habits,
	}, nil
}

// GetHabits returns all active habits for a user, enriched.
func (s *HabitService) GetHabits(userID string) ([]model.Habit, error) {
	return s.getEnrichedHabits(userID)
}

// CompleteHabit marks a habit as done today. Returns streak and a milestone message.
func (s *HabitService) CompleteHabit(habitID, userID string) (int, string, error) {
	today := time.Now().UTC().Format("2006-01-02")

	// Check ownership
	h, err := s.habitRepo.GetByID(habitID, userID)
	if err != nil {
		return 0, "", err
	}
	if h == nil {
		return 0, "", fmt.Errorf("habit not found")
	}

	inserted, err := s.habitRepo.AddCompletion(habitID, userID, today)
	if err != nil {
		return 0, "", err
	}
	if !inserted {
		// Already completed today — return current streak.
		streak := s.getCachedStreak(habitID)
		return streak, "", nil
	}

	// Recalculate streak from DB.
	streak, err := s.habitRepo.CalculateStreak(habitID)
	if err != nil {
		return 0, "", err
	}

	// Persist streak in Redis.
	streakKey := fmt.Sprintf("hb:habit:%s:streak", habitID)
	lastDateKey := fmt.Sprintf("hb:habit:%s:last_date", habitID)
	if err := s.redis.Set(streakKey, strconv.Itoa(streak)); err != nil {
		log.Printf("redis: failed to set streak: %v", err)
	}
	if err := s.redis.Set(lastDateKey, today); err != nil {
		log.Printf("redis: failed to set last_date: %v", err)
	}

	// Increment daily counter.
	dailyKey := fmt.Sprintf("hb:user:%s:daily:%s", userID, today)
	if _, err := s.redis.Incr(dailyKey); err != nil {
		log.Printf("redis: failed to incr daily counter: %v", err)
	}

	// Increment total counter.
	totalKey := fmt.Sprintf("hb:user:%s:total", userID)
	if _, err := s.redis.Incr(totalKey); err != nil {
		log.Printf("redis: failed to incr total: %v", err)
	}

	// Invalidate habit list cache.
	s.invalidateHabitCache(userID)

	milestone := milestoneMessage(streak)
	return streak, milestone, nil
}

// UndoCompletion removes today's completion for a habit.
func (s *HabitService) UndoCompletion(habitID, userID string) (int, error) {
	today := time.Now().UTC().Format("2006-01-02")

	h, err := s.habitRepo.GetByID(habitID, userID)
	if err != nil {
		return 0, err
	}
	if h == nil {
		return 0, fmt.Errorf("habit not found")
	}

	removed, err := s.habitRepo.RemoveCompletion(habitID, userID, today)
	if err != nil {
		return 0, err
	}
	if !removed {
		return s.getCachedStreak(habitID), nil
	}

	// Recalculate streak.
	streak, err := s.habitRepo.CalculateStreak(habitID)
	if err != nil {
		return 0, err
	}

	streakKey := fmt.Sprintf("hb:habit:%s:streak", habitID)
	if err := s.redis.Set(streakKey, strconv.Itoa(streak)); err != nil {
		log.Printf("redis: failed to update streak after undo: %v", err)
	}

	s.invalidateHabitCache(userID)
	return streak, nil
}

// GetHabitStats returns detailed stats for a single habit.
func (s *HabitService) GetHabitStats(habitID, userID string) (*model.HabitStats, error) {
	h, err := s.habitRepo.GetByID(habitID, userID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}

	streak, err := s.habitRepo.CalculateStreak(habitID)
	if err != nil {
		return nil, err
	}

	total, err := s.habitRepo.GetTotalCompletions(habitID)
	if err != nil {
		return nil, err
	}

	history, err := s.habitRepo.GetCompletionDates(habitID, 30)
	if err != nil {
		return nil, err
	}

	rate := 0.0
	if len(history) > 0 {
		rate = float64(len(history)) / 30.0
	}

	return &model.HabitStats{
		HabitID:        habitID,
		HabitName:      h.Name,
		Streak:         streak,
		LongestStreak:  streak, // simplified — full longest streak calc omitted for brevity
		TotalCompleted: total,
		Rate30Day:      rate,
		History:        history,
	}, nil
}

// ---- helpers ----

// getEnrichedHabits fetches habits and adds streak + completedToday fields.
func (s *HabitService) getEnrichedHabits(userID string) ([]model.Habit, error) {
	// Try cache first.
	cacheKey := fmt.Sprintf("hb:user:%s:habits", userID)
	cached, found, err := s.redis.Get(cacheKey)
	if err == nil && found {
		var habits []model.Habit
		if json.Unmarshal([]byte(cached), &habits) == nil {
			if habits == nil {
				habits = []model.Habit{}
			}
			return habits, nil
		}
	}

	// Cache miss — fetch from DB.
	habits, err := s.habitRepo.GetActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	if habits == nil {
		habits = []model.Habit{}
	}

	today := time.Now().UTC().Format("2006-01-02")

	for i := range habits {
		// Streak from Redis (fast path).
		habits[i].Streak = s.getCachedStreak(habits[i].ID)
		if habits[i].Streak == 0 {
			// Fallback to DB calculation.
			streak, err := s.habitRepo.CalculateStreak(habits[i].ID)
			if err == nil {
				habits[i].Streak = streak
			}
		}

		// CompletedToday from DB.
		done, err := s.habitRepo.IsCompletedOnDate(habits[i].ID, today)
		if err == nil {
			habits[i].CompletedToday = done
		}
	}

	// Cache the enriched list.
	if data, err := json.Marshal(habits); err == nil {
		if err := s.redis.Set(cacheKey, string(data)); err != nil {
			log.Printf("redis: failed to cache habits: %v", err)
		}
	}

	return habits, nil
}

func (s *HabitService) getCachedStreak(habitID string) int {
	key := fmt.Sprintf("hb:habit:%s:streak", habitID)
	val, found, err := s.redis.Get(key)
	if err != nil || !found {
		return 0
	}
	n, _ := strconv.Atoi(val)
	return n
}

func (s *HabitService) invalidateHabitCache(userID string) {
	key := fmt.Sprintf("hb:user:%s:habits", userID)
	if err := s.redis.Del(key); err != nil {
		log.Printf("redis: failed to invalidate cache: %v", err)
	}
}

func milestoneMessage(streak int) string {
	switch {
	case streak == 7:
		return "One week streak! You're on fire!"
	case streak == 14:
		return "Two weeks! Building a real habit!"
	case streak == 21:
		return "21 days — this is now a habit!"
	case streak == 30:
		return "30-day streak! Incredible!"
	case streak > 0 && streak%10 == 0:
		return fmt.Sprintf("%d days and counting!", streak)
	default:
		return ""
	}
}
