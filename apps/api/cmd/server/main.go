package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/habit-buddy/api/internal/api"
	"github.com/habit-buddy/api/internal/events"
	_ "github.com/habit-buddy/api/internal/logger" // init JSON structured logger
	"github.com/habit-buddy/api/internal/model"
	"github.com/habit-buddy/api/internal/redisclient"
	"github.com/habit-buddy/api/internal/repository"
	"github.com/habit-buddy/api/internal/service"
	"github.com/habit-buddy/api/internal/ws"
)

func main() {
	cfg := loadConfig()

	db := mustConnectDB(cfg.DatabaseURL)
	defer db.Close()

	if err := runMigrations(db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	redis := mustConnectRedis(cfg.RedisAddr)
	defer redis.Close()

	// --- WebSocket hub ---
	hub := ws.NewHub()
	go hub.Run()

	// --- Event router ---
	// Wire the routing layer: each event_type maps to one or more handlers.
	// WSBroadcastHandler fans out to hub channels; add further handlers here
	// (e.g. analytics, notifications) without touching transport code.
	router := events.NewRouter(events.DefaultRetryPolicy)
	wsBroadcast := events.NewWSBroadcastHandler(hub)

	for _, eventType := range []string{
		model.EventHabitCompleted,
		model.EventHabitUndone,
		model.EventHabitCreated,
		model.EventHabitUpdated,
		model.EventHabitArchived,
	} {
		router.On(eventType, wsBroadcast)
	}

	// --- Event bridge (transport layer) ---
	bridge, err := ws.NewEventBridge(cfg.RedisAddr, router)
	if err != nil {
		slog.Error("event bridge failed", "error", err)
		os.Exit(1)
	}
	defer bridge.Close()

	// --- HTTP handlers ---
	userRepo := repository.NewUserRepository(db)
	habitRepo := repository.NewHabitRepository(db)
	habitSvc := service.NewHabitService(habitRepo, redis)

	authHandler := api.NewAuthHandler(userRepo, cfg.JWTSecret)
	habitHandler := api.NewHabitHandler(habitSvc, bridge)

	httpRouter := api.NewRouter(authHandler, habitHandler, hub, cfg.JWTSecret)

	addr := ":" + cfg.Port
	slog.Info("habit-buddy API starting", "addr", addr, "redis_addr", cfg.RedisAddr)

	if err := http.ListenAndServe(addr, httpRouter); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

type config struct {
	DatabaseURL string
	RedisAddr   string
	JWTSecret   string
	Port        string
}

func loadConfig() config {
	return config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://habit:habit@localhost:5432/habitbuddy?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustConnectDB(url string) *sql.DB {
	var db *sql.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", url)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.SetMaxOpenConns(25)
				db.SetMaxIdleConns(5)
				db.SetConnMaxLifetime(5 * time.Minute)
				slog.Info("connected to PostgreSQL")
				return db
			}
		}
		slog.Warn("waiting for postgres", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	slog.Error("could not connect to postgres", "error", err)
	os.Exit(1)
	return nil
}

func mustConnectRedis(addr string) *redisclient.Client {
	var client *redisclient.Client
	var err error
	for i := 0; i < 10; i++ {
		client, err = redisclient.NewClient(addr)
		if err == nil {
			slog.Info("connected to go-redis", "addr", addr)
			return client
		}
		slog.Warn("waiting for go-redis", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	slog.Error("could not connect to go-redis", "error", err)
	os.Exit(1)
	return nil
}

func runMigrations(db *sql.DB) error {
	migration, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}
