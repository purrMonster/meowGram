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

	"meowgram/server/internal/config"
	"meowgram/server/internal/handler"
	"meowgram/server/internal/middleware"
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
	)

	// Setup multiplexer / router
	mux := http.NewServeMux()

	// Endpoints
	mux.HandleFunc("GET /healthz", handler.HealthHandler(cfg))
	mux.HandleFunc("HEAD /healthz", handler.HealthHandler(cfg))
	mux.HandleFunc("/ws", handler.EchoWebSocketHandler(cfg, logger))

	// Middleware chain: RequestLogger -> CORS -> Mux
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

	// Server shutdown channel
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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced shutdown with error", "error", err)
			_ = srv.Close()
		}

		logger.Info("meowGram server gracefully stopped")
	}
}
