package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/api/ratelimit"
	"github.com/QTest-hq/qtest/internal/auth"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
	"github.com/QTest-hq/qtest/internal/telemetry"
	"github.com/QTest-hq/qtest/internal/webhook"
)

func main() {
	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Create base context
	ctx := context.Background()

	// Initialize telemetry (OpenTelemetry tracing)
	var telemetryProvider *telemetry.Provider
	if cfg.Telemetry.Enabled {
		telemetryProvider, err = telemetry.New(ctx, telemetry.Config{
			Enabled:      cfg.Telemetry.Enabled,
			OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
			SamplingRate: cfg.Telemetry.SamplingRate,
			ServiceName:  cfg.Telemetry.ServiceName,
			Environment:  os.Getenv("ENV"),
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to initialize telemetry, tracing disabled")
		} else if telemetryProvider != nil {
			defer telemetryProvider.Shutdown(context.Background())
			log.Info().Msg("OpenTelemetry tracing enabled")
		}
	}

	// Connect to database (single connection pool for all components)
	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer database.Close()

	// Create job repository using the same connection pool
	// StdDB() returns a *sql.DB that wraps the pgx pool, avoiding duplicate connections
	jobRepo := jobs.NewRepository(database.StdDB())
	log.Info().Msg("job repository initialized (using shared connection pool)")

	// Connect to NATS (optional)
	var natsClient *qtestnats.Client
	if cfg.NATSURL != "" {
		natsClient, err = qtestnats.NewClient(cfg.NATSURL)
		if err != nil {
			log.Warn().Err(err).Msg("failed to connect to NATS, job notifications disabled")
		} else {
			log.Info().Str("url", cfg.NATSURL).Msg("connected to NATS")
			defer natsClient.Close()

			// Setup streams
			if err := natsClient.SetupStreams(ctx); err != nil {
				log.Warn().Err(err).Msg("failed to setup NATS streams")
			}
		}
	}

	// Create server
	srv, err := api.NewServer(cfg, database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create server")
	}

	// Configure job system
	srv.SetJobSystem(jobRepo, natsClient)
	log.Info().Msg("job system enabled")

	// Configure rate limiting
	if cfg.RateLimit.Enabled {
		rateLimitCfg := &ratelimit.Config{
			Enabled:          cfg.RateLimit.Enabled,
			DefaultPerMinute: cfg.RateLimit.DefaultPerMinute,
			DefaultPerHour:   cfg.RateLimit.DefaultPerHour,
			IPPerMinute:      cfg.RateLimit.IPPerMinute,
			IPPerHour:        cfg.RateLimit.IPPerHour,
			StorageBackend:   cfg.RateLimit.StorageBackend,
			RedisURL:         cfg.RedisURL,
		}

		rateLimiter, err := ratelimit.New(rateLimitCfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create rate limiter")
		}
		defer rateLimiter.Close()

		srv.SetRateLimiter(rateLimiter)
		storageBackend := cfg.RateLimit.StorageBackend
		if storageBackend == "" {
			storageBackend = "memory"
		}
		log.Info().
			Str("storage", storageBackend).
			Int("default_per_minute", cfg.RateLimit.DefaultPerMinute).
			Int("default_per_hour", cfg.RateLimit.DefaultPerHour).
			Int("ip_per_minute", cfg.RateLimit.IPPerMinute).
			Int("ip_per_hour", cfg.RateLimit.IPPerHour).
			Msg("rate limiting enabled")
	}

	// Configure webhooks
	webhookStore := db.NewStore(database)
	webhookService := webhook.NewService(webhookStore)
	srv.SetWebhookService(webhookService)

	// Configure JWT and authentication services
	var jwtService *auth.JWTService
	var refreshService *auth.RefreshService
	var lockoutService *auth.LockoutService

	// Initialize JWT service if keys are configured
	if cfg.Security.JWTPrivateKeyPath != "" || cfg.Security.JWTPrivateKeyPEM != "" {
		var err error
		jwtService, err = auth.NewJWTService(auth.JWTConfig{
			PrivateKeyPath: cfg.Security.JWTPrivateKeyPath,
			PrivateKeyPEM:  cfg.Security.JWTPrivateKeyPEM,
			PublicKeyPath:  cfg.Security.JWTPublicKeyPath,
			Issuer:         cfg.Security.JWTIssuer,
			AccessTTL:      cfg.Security.JWTAccessTTL,
			RefreshTTL:     cfg.Security.JWTRefreshTTL,
			KeyID:          cfg.Security.JWTKeyID,
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to initialize JWT service, JWT auth disabled")
		} else {
			log.Info().
				Str("issuer", cfg.Security.JWTIssuer).
				Dur("access_ttl", cfg.Security.JWTAccessTTL).
				Dur("refresh_ttl", cfg.Security.JWTRefreshTTL).
				Msg("JWT service initialized")

			// Initialize refresh service (requires JWT service)
			refreshService = auth.NewRefreshService(database.Pool(), jwtService)
			log.Info().Msg("refresh token service initialized")
		}
	}

	// Initialize lockout service
	lockoutService = auth.NewLockoutService(database.Pool(), auth.LockoutConfig{
		MaxAttempts:     cfg.Security.LockoutMaxAttempts,
		LockoutDuration: cfg.Security.LockoutDuration,
		WindowDuration:  cfg.Security.LockoutWindowDuration,
	})
	log.Info().
		Int("max_attempts", cfg.Security.LockoutMaxAttempts).
		Dur("lockout_duration", cfg.Security.LockoutDuration).
		Msg("account lockout service initialized")

	// Wire auth services to server
	srv.SetAuthServices(jwtService, refreshService, lockoutService)

	// Start WebSocket hub for real-time updates
	srv.StartWebSocketHub(ctx)

	// Initialize server (setup middleware and routes after all components configured)
	if err := srv.Initialize(); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize server")
	}
	log.Info().Msg("server initialized")

	// Start webhook dispatcher for background delivery
	webhookDispatcher := webhook.NewDispatcher(webhookService, 5*time.Second, 50)
	if err := webhookDispatcher.Start(ctx); err != nil {
		log.Warn().Err(err).Msg("failed to start webhook dispatcher")
	} else {
		log.Info().Msg("webhook dispatcher started")
	}

	// Start server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info().Msg("server is shutting down...")

		// Stop webhook dispatcher
		webhookDispatcher.Stop()

		// Close server background tasks (clone pool, etc.)
		srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("could not gracefully shutdown the server")
		}
		close(done)
	}()

	log.Info().Int("port", cfg.Port).Msg("starting API server")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("could not listen on port")
	}

	<-done
	log.Info().Msg("server stopped")
}
