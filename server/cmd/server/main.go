package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/database"
	"meowgram/server/internal/handler"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/repository"
)

func main() {
	// Initialize structured logger
	var logLevel slog.Level
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Load configuration strictly from environment variables
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load environment configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting meowGram backend service",
		"domain", cfg.AppDomain,
		"port", cfg.Port,
		"environment", cfg.Environment,
		"cors_origins", cfg.CORSOrigins,
		"authelia_issuer", cfg.AutheliaIssuer,
		"authelia_jwks_url", cfg.AutheliaJWKSURL,
	)

	// Initialize PostgreSQL connection pool and apply schema migrations
	db, err := database.Open(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("Failed to connect to database or execute migrations", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize repository layers
	userRepo := repository.NewUserRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Initialize the Real-Time Broadcast Hub
	hub := chat.NewHub(logger)
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	// Run Hub in its own dedicated background goroutine
	go hub.Run(hubCtx)

	// Initialize Authelia OIDC Token Verifier
	oidcVerifier, err := auth.NewOIDCVerifier(context.Background(), cfg)
	if err != nil {
		logger.Error("Failed to initialize OIDC verifier", "error", err)
		os.Exit(1)
	}

	// Setup multiplexer / router
	mux := http.NewServeMux()

	// Public health-check endpoints
	mux.HandleFunc("GET /healthz", handler.HealthHandler(cfg))
	mux.HandleFunc("HEAD /healthz", handler.HealthHandler(cfg))

	// Protected WebSocket endpoint enforcing Authelia OIDC token verification & Hub broadcast
	authMiddleware := auth.Middleware(oidcVerifier, userRepo, logger)
	mux.Handle("/ws", authMiddleware(handler.WebSocketHandler(hub, messageRepo, cfg, logger)))

	// Global Middleware chain: RequestLogger -> CORS -> Mux
	var rootHandler http.Handler = mux
	rootHandler = middleware.CORS(cfg)(rootHandler)
	rootHandler = middleware.RequestLogger(logger)(rootHandler)

	srv := &http.Server{
		Addr:         cfg.Address(),
		Handler:      rootHandler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Server startup failure channel
	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("Server listening for incoming connections", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Graceful shutdown handling
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		logger.Error("Fatal server startup failure", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		logger.Info("Shutdown signal received", "signal", sig.String())

		// Cancel Hub context to disconnect active clients
		hubCancel()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced shutdown with error", "error", err)
			_ = srv.Close()
		}

		logger.Info("meowGram server gracefully stopped")
	}
}
