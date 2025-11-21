package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/api"
	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/jobs"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
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

	// Connect to database (pgx for existing store)
	ctx := context.Background()
	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer database.Close()

	// Connect to database (sql.DB for jobs repository)
	var sqlDB *sql.DB
	var jobRepo *jobs.Repository
	sqlDB, err = sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Warn().Err(err).Msg("failed to open SQL connection for jobs, job system disabled")
	} else if err := sqlDB.Ping(); err != nil {
		log.Warn().Err(err).Msg("failed to ping SQL database for jobs, job system disabled")
		sqlDB.Close()
		sqlDB = nil
	} else {
		jobRepo = jobs.NewRepository(sqlDB)
		log.Info().Msg("job repository initialized")
		defer sqlDB.Close()
	}

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
	if jobRepo != nil {
		srv.SetJobSystem(jobRepo, natsClient)
		log.Info().Msg("job system enabled")
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
