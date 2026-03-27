package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/signsafe-io/signsafe-api/internal/cache"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/storage"
)

func main() {
	// --- Configuration ---
	port := getEnv("PORT", "8080")
	databaseURL := getEnv("DATABASE_URL", "postgres://signsafe:signsafe@signsafe-db:5432/signsafe?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://signsafe-cache:6379")
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@signsafe-queue:5672")
	s3Endpoint := getEnv("S3_ENDPOINT", "http://signsafe-store:8333")
	s3AccessKey := getEnv("S3_ACCESS_KEY", "")
	s3SecretKey := getEnv("S3_SECRET_KEY", "")
	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./migrations")
	allowedOrigins := strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ",")

	// --- DB ---
	db, err := repository.NewDB(databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	slog.Info("database connected")

	if err := repository.RunMigrations(databaseURL, migrationsPath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	slog.Info("migrations applied")

	// --- Redis ---
	cacheClient, err := cache.NewClient(redisURL)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer cacheClient.Close()
	slog.Info("redis connected")

	// --- RabbitMQ ---
	queueClient, err := queue.NewClient(rabbitmqURL)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer queueClient.Close()
	slog.Info("rabbitmq connected")

	// --- Storage ---
	storageClient := storage.NewClient(s3Endpoint, s3AccessKey, s3SecretKey)
	slog.Info("storage client created")

	// --- Router ---
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(allowedOrigins))

	// Health check (no auth)
	r.Get("/health", healthHandler)

	// Auth routes (no auth middleware)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", notImplemented)
		r.Post("/verify-email", notImplemented)
		r.Post("/login", notImplemented)
		r.Post("/refresh", notImplemented)
		r.Post("/logout", notImplemented)
		r.Post("/password/forgot", notImplemented)
		r.Post("/password/reset", notImplemented)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtSecret))

		r.Get("/users/me", notImplemented)

		r.Route("/contracts", func(r chi.Router) {
			r.Get("/", notImplemented)
			r.Post("/", notImplemented)
			r.Route("/{contractId}", func(r chi.Router) {
				r.Get("/clauses", notImplemented)
				r.Get("/snippets", notImplemented)
				r.Post("/risk-analyses", notImplemented)
			})
		})

		r.Route("/ingestion-jobs/{jobId}", func(r chi.Router) {
			r.Get("/", notImplemented)
		})

		r.Route("/risk-analyses/{analysisId}", func(r chi.Router) {
			r.Get("/", notImplemented)
			r.Post("/overrides", notImplemented)
		})

		r.Route("/evidence-sets/{evidenceSetId}", func(r chi.Router) {
			r.Get("/", notImplemented)
			r.Post("/retrieve", notImplemented)
		})

		r.Post("/audit-events", notImplemented)
	})

	// Log dependency references to avoid unused variable errors.
	_ = db
	_ = cacheClient
	_ = queueClient
	_ = storageClient

	addr := fmt.Sprintf(":%s", port)
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not implemented"})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
