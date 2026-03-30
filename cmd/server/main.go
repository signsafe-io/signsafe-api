package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/signsafe-io/signsafe-api/internal/cache"
	"github.com/signsafe-io/signsafe-api/internal/email"
	"github.com/signsafe-io/signsafe-api/internal/handler"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/service"
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
	smtpHost := getEnv("SMTP_HOST", "")
	smtpPort := getEnv("SMTP_PORT", "587")
	smtpUser := getEnv("SMTP_USER", "")
	smtpPass := getEnv("SMTP_PASS", "")
	smtpFrom := getEnv("SMTP_FROM", "")
	appBaseURL := getEnv("APP_BASE_URL", "https://signsafe-web.dsmhs.kr")

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

	// --- Email ---
	emailClient := email.NewClient(email.Config{
		Host:   smtpHost,
		Port:   smtpPort,
		User:   smtpUser,
		Pass:   smtpPass,
		From:   smtpFrom,
		AppURL: appBaseURL,
	})

	// --- Services ---
	userRepo := repository.NewUserRepo(db)
	authSvc := service.NewAuthService(userRepo, cacheClient, emailClient, jwtSecret)
	orgSvc := service.NewOrgService(userRepo)

	contractRepo := repository.NewContractRepo(db)
	contractSvc := service.NewContractService(contractRepo, userRepo, queueClient, storageClient)

	analysisRepo := repository.NewAnalysisRepo(db)
	analysisSvc := service.NewAnalysisService(analysisRepo, contractRepo, userRepo, queueClient, cacheClient)

	evidenceRepo := repository.NewEvidenceRepo(db)
	evidenceSvc := service.NewEvidenceService(evidenceRepo, analysisRepo, contractRepo, userRepo, queueClient)

	auditRepo := repository.NewAuditRepo(db)
	auditSvc := service.NewAuditService(auditRepo)

	// --- Handlers ---
	authHandler := handler.NewAuthHandler(authSvc)
	orgHandler := handler.NewOrgHandler(orgSvc, authSvc)
	contractHandler := handler.NewContractHandler(contractSvc, auditSvc)
	analysisHandler := handler.NewAnalysisHandler(analysisSvc, auditSvc)
	evidenceHandler := handler.NewEvidenceHandler(evidenceSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)

	// --- Router ---
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID) // must run before Logger so GetReqID works
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(allowedOrigins))

	// Health check (no auth) — probes DB, Redis, RabbitMQ connectivity.
	r.Get("/health", makeHealthHandler(db, cacheClient, queueClient))

	// Auth routes (no auth middleware)
	// Rate limiters: login/signup are brute-force targets.
	loginLimiter := middleware.RateLimiter(cacheClient, 10, 1*time.Minute)
	signupLimiter := middleware.RateLimiter(cacheClient, 5, 1*time.Minute)
	r.Route("/auth", func(r chi.Router) {
		r.With(signupLimiter).Post("/signup", authHandler.Signup)
		r.Post("/verify-email", authHandler.VerifyEmail)
		r.With(loginLimiter).Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
		r.Post("/password/forgot", authHandler.ForgotPassword)
		r.Post("/password/reset", authHandler.ResetPassword)
		r.With(signupLimiter).Post("/resend-verification", authHandler.ResendVerification)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtSecret))

		r.Get("/users/me", authHandler.GetMe)
		r.Patch("/users/me", orgHandler.UpdateProfile)
		r.Patch("/users/me/password", orgHandler.ChangePassword)

		r.Route("/organizations/{orgId}", func(r chi.Router) {
			r.Get("/", orgHandler.GetOrganization)
			r.Patch("/", orgHandler.UpdateOrganization)
			r.Get("/members", orgHandler.ListMembers)
			r.Post("/members", orgHandler.InviteMember)
			r.Delete("/members/{userId}", orgHandler.RemoveMember)
		})

		r.Route("/contracts", func(r chi.Router) {
			r.Get("/", contractHandler.List)
			r.Post("/", contractHandler.Upload)
			r.Route("/{contractId}", func(r chi.Router) {
				r.Get("/", contractHandler.Get)
				r.Delete("/", contractHandler.Delete)
				r.Patch("/", contractHandler.Update)
				r.Get("/file", contractHandler.GetFile)
				r.Get("/clauses", contractHandler.ListClauses)
				r.Get("/snippets", contractHandler.GetSnippets)
				r.Get("/risk-analyses", analysisHandler.GetLatestAnalysis)
				r.Post("/risk-analyses", analysisHandler.CreateAnalysis)
			})
		})

		r.Route("/ingestion-jobs/{jobId}", func(r chi.Router) {
			r.Get("/", contractHandler.GetIngestionJob)
		})

		r.Route("/risk-analyses/{analysisId}", func(r chi.Router) {
			r.Get("/", analysisHandler.GetAnalysis)
			r.Post("/overrides", analysisHandler.CreateOverride)
		})

		r.Route("/evidence-sets/{evidenceSetId}", func(r chi.Router) {
			r.Get("/", evidenceHandler.GetEvidenceSet)
			r.Post("/retrieve", evidenceHandler.RetrieveEvidence)
		})

		r.Post("/audit-events", auditHandler.CreateAuditEvent)
	})

	// --- Server with graceful shutdown ---
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Listen for OS signals in a goroutine.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until signal received.
	<-ctx.Done()
	slog.Info("shutdown signal received, draining in-flight requests")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
	slog.Info("server shut down cleanly")
}

// dbPinger is satisfied by *sqlx.DB (used for health check only).
type dbPinger interface {
	PingContext(ctx context.Context) error
}

func makeHealthHandler(db dbPinger, cacheClient *cache.Client, queueClient *queue.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		type componentStatus struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}

		components := map[string]componentStatus{}
		healthy := true

		// DB ping.
		if err := db.PingContext(ctx); err != nil {
			components["db"] = componentStatus{Status: "unhealthy", Error: err.Error()}
			healthy = false
		} else {
			components["db"] = componentStatus{Status: "ok"}
		}

		// Redis ping.
		if err := cacheClient.Ping(ctx); err != nil {
			components["redis"] = componentStatus{Status: "unhealthy", Error: err.Error()}
			healthy = false
		} else {
			components["redis"] = componentStatus{Status: "ok"}
		}

		// RabbitMQ connection check.
		if err := queueClient.Ping(); err != nil {
			components["rabbitmq"] = componentStatus{Status: "unhealthy", Error: err.Error()}
			healthy = false
		} else {
			components["rabbitmq"] = componentStatus{Status: "ok"}
		}

		status := "ok"
		code := http.StatusOK
		if !healthy {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     status,
			"components": components,
		})
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
