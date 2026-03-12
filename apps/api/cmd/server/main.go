package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/habit-buddy/api/internal/api"
	"github.com/habit-buddy/api/internal/redisclient"
	"github.com/habit-buddy/api/internal/repository"
	"github.com/habit-buddy/api/internal/service"
	"github.com/habit-buddy/api/internal/ws"
)

func main() {
	cfg := loadConfig()

	// Connect to PostgreSQL with retry
	db := mustConnectDB(cfg.DatabaseURL)
	defer db.Close()

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Connect to go-redis
	redis := mustConnectRedis(cfg.RedisAddr)
	defer redis.Close()

	// Build dependency graph
	userRepo := repository.NewUserRepository(db)
	habitRepo := repository.NewHabitRepository(db)
	habitSvc := service.NewHabitService(habitRepo, redis)

	hub := ws.NewHub()
	go hub.Run()

	bridge, err := ws.NewEventBridge(cfg.RedisAddr, hub)
	if err != nil {
		log.Fatalf("event bridge failed: %v", err)
	}
	defer bridge.Close()

	authHandler := api.NewAuthHandler(userRepo, cfg.JWTSecret)
	habitHandler := api.NewHabitHandler(habitSvc, bridge)

	router := api.NewRouter(authHandler, habitHandler, hub, cfg.JWTSecret)

	addr := ":" + cfg.Port
	log.Printf("habit-buddy API listening on %s", addr)
	log.Printf("go-redis connected at %s", cfg.RedisAddr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
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
				log.Println("connected to PostgreSQL")
				return db
			}
		}
		log.Printf("waiting for postgres (%d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("could not connect to postgres: %v", err)
	return nil
}

func mustConnectRedis(addr string) *redisclient.Client {
	var client *redisclient.Client
	var err error

	for i := 0; i < 10; i++ {
		client, err = redisclient.NewClient(addr)
		if err == nil {
			log.Printf("connected to go-redis at %s", addr)
			return client
		}
		log.Printf("waiting for go-redis (%d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("could not connect to go-redis: %v", err)
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
	log.Println("migrations applied")
	return nil
}
