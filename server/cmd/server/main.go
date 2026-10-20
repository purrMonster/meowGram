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
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/database"
	"meowgram/server/internal/fcm"
	"meowgram/server/internal/handler"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/ratelimit"
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
		"authelia_domain", cfg.AutheliaDomain,
		"authelia_issuer", cfg.AutheliaIssuer,
		"authelia_jwks_url", cfg.AutheliaJWKSURL,
		"sync_endpoint", cfg.SyncEndpoint,
		"health_endpoint", cfg.HealthEndpoint,
		"ws_endpoint", cfg.WSEndpoint,
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
	deviceRepo := repository.NewDeviceRepository(db)

	// Initialize FCM service if credentials are provided
	var fcmService *fcm.Service
	if cfg.GoogleCredentials != "" {
		svc, err := fcm.NewService(context.Background(), logger, cfg.GoogleCredentials, deviceRepo)
		if err != nil {
			logger.Warn("Failed to initialize FCM service, push notifications will be disabled", "error", err)
		} else {
			fcmService = svc
			logger.Info("FCM Service initialized successfully")
		}
	} else {
		logger.Warn("GOOGLE_APPLICATION_CREDENTIALS not set; push notifications disabled")
	}

	// Initialize the Real-Time Broadcast Hub
	var publisher chat.PushPublisher
	if fcmService != nil {
		publisher = fcmService // avoid a non-nil interface wrapping a nil *fcm.Service
	}
	hub := chat.NewHub(logger, publisher, ratelimit.New(cfg.RateLimits.MessagePerMinute, cfg.RateLimits.MessageBurst))
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

	// Public health-check endpoints (standard + dynamic route)
	mux.HandleFunc("GET /healthz", handler.HealthHandler(cfg))
	mux.HandleFunc("HEAD /healthz", handler.HealthHandler(cfg))
	if cfg.HealthEndpoint != "/healthz" {
		mux.HandleFunc("GET "+cfg.HealthEndpoint, handler.HealthHandler(cfg))
		mux.HandleFunc("HEAD "+cfg.HealthEndpoint, handler.HealthHandler(cfg))
	}

	// Public sign-in settings for domain-free client builds (runbook §10.11).
	mux.HandleFunc("GET "+handler.ClientConfigPath, handler.ClientConfigHandler(cfg))

	// Protected WebSocket endpoint enforcing Authelia OIDC token verification & Hub broadcast
	verifyIdentity := auth.Middleware(oidcVerifier, userRepo, logger)
	userBudget := middleware.UserRateLimit(ratelimit.New(cfg.RateLimits.APIPerMinute, cfg.RateLimits.APIBurst))
	authMiddleware := func(next http.Handler) http.Handler { return verifyIdentity(userBudget(next)) }
	integration, err := config.LoadIntegration()
	if err == nil {
		err = integration.ValidateRoutes("/healthz", cfg.HealthEndpoint, handler.ClientConfigPath, "/ws", cfg.WSEndpoint, "/api/ws-ticket", cfg.WSTicketEndpoint, "/api/messages/sync", cfg.SyncEndpoint, cfg.PushDeviceEndpoint)
	}
	if err != nil {
		logger.Error("Invalid integration configuration")
		os.Exit(1)
	}
	if integration.SendPath != "" {
		var send http.Handler = handler.SendHandler(hub.MessageService(messageRepo))
		botConfig, err := config.LoadBotAuth()
		if err != nil {
			logger.Error("Invalid bot configuration")
			os.Exit(1)
		}
		if botConfig.Enabled {
			directory, err := botauth.NewDirectory(botConfig.Directory)
			if err != nil {
				logger.Error("Cannot initialize bot directory")
				os.Exit(1)
			}
			bots := repository.NewBotRepository(db)
			gate := botauth.Gate{Directory: directory, Resolve: func(ctx context.Context, id botauth.Identity) (botauth.Principal, error) {
				p, err := bots.Resolve(ctx, id.Namespace, id.ExternalID, id.DisplayName)
				if err != nil {
					return botauth.Principal{}, err
				}
				return botauth.Principal{ID: p.ID, DisplayName: p.Username}, nil
			}, Ingress: ratelimit.New(cfg.RateLimits.APIPerMinute, cfg.RateLimits.APIBurst), API: ratelimit.New(cfg.RateLimits.APIPerMinute, cfg.RateLimits.APIBurst), HumanOnly: authMiddleware, Logger: logger, SendPath: integration.SendPath, TrustedProxies: botConfig.TrustedProxies, Timeout: botConfig.Directory.Timeout}
			send, err = gate.Handler(send)
			if err != nil {
				logger.Error("Cannot initialize bot gate")
				os.Exit(1)
			}
		} else {
			send = authMiddleware(send)
		}
		mux.Handle("POST "+integration.SendPath, send)
	}
	if integration.ReviewPath != "" {
		bridge, err := handler.ReviewBridge(integration)
		if err != nil {
			logger.Error("Cannot initialize review bridge")
			os.Exit(1)
		}
		mux.Handle("POST "+integration.ReviewPath, authMiddleware(bridge))
	}
	ticketStore := auth.NewWSTicketStore()
	wsHandler := handler.WebSocketHandler(hub, messageRepo, ticketStore, cfg, logger)
	mux.Handle("/ws", wsHandler)
	if cfg.WSEndpoint != "/ws" {
		mux.Handle(cfg.WSEndpoint, wsHandler)
	}

	// Browser clients exchange their bearer token for a one-use WebSocket ticket.
	mux.Handle("POST /api/ws-ticket", authMiddleware(handler.WSTicketHandler(ticketStore, logger)))
	if cfg.WSTicketEndpoint != "/api/ws-ticket" {
		mux.Handle("POST "+cfg.WSTicketEndpoint, authMiddleware(handler.WSTicketHandler(ticketStore, logger)))
	}

	// Protected catch-up synchronization endpoint (UST-1.4.3)
	deviceHandler := authMiddleware(handler.PushDeviceHandler(deviceRepo))
	mux.Handle("PUT "+cfg.PushDeviceEndpoint, deviceHandler)
	mux.Handle("DELETE "+cfg.PushDeviceEndpoint, deviceHandler)

	mux.Handle("GET /api/messages/sync", authMiddleware(handler.SyncHandler(messageRepo, logger)))
	if cfg.SyncEndpoint != "/api/messages/sync" {
		mux.Handle("GET "+cfg.SyncEndpoint, authMiddleware(handler.SyncHandler(messageRepo, logger)))
	}

	// Global Middleware chain: RequestLogger -> CORS -> Mux
	var rootHandler http.Handler = mux
	rootHandler = middleware.GlobalRateLimit(ratelimit.New(cfg.RateLimits.GlobalPerMinute, cfg.RateLimits.GlobalBurst), cfg.HealthEndpoint)(rootHandler)
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

		// Stop accepting new connections first, then disconnect WebSocket clients.
		// (Hijacked WebSocket connections are not tracked by srv.Shutdown.)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced shutdown with error", "error", err)
			_ = srv.Close()
		}

		hubCancel()
		select {
		case <-hub.Done():
		case <-ctx.Done():
		}

		logger.Info("meowGram server gracefully stopped")
	}
}
