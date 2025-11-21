package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/config"
	qtestnats "github.com/QTest-hq/qtest/internal/nats"
	"github.com/QTest-hq/qtest/internal/worker"
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

	// Determine worker type from env or args
	workerType := os.Getenv("WORKER_TYPE")
	if workerType == "" {
		workerType = "all" // Run all worker types
	}

	// Connect to database (optional)
	var db *sql.DB
	if cfg.DatabaseURL != "" {
		db, err = sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			log.Warn().Err(err).Msg("failed to connect to database, workers will run in limited mode")
		} else if err := db.Ping(); err != nil {
			log.Warn().Err(err).Msg("database ping failed, workers will run in limited mode")
			db.Close()
			db = nil
		} else {
			log.Info().Msg("connected to database")
			defer db.Close()
		}
	}

	// Connect to NATS (optional)
	var natsClient *qtestnats.Client
	if cfg.NATSURL != "" {
		natsClient, err = qtestnats.NewClient(cfg.NATSURL)
		if err != nil {
			log.Warn().Err(err).Msg("failed to connect to NATS, workers will poll database")
		} else {
			log.Info().Str("url", cfg.NATSURL).Msg("connected to NATS")
			defer natsClient.Close()
		}
	}

	// Create worker pool
	poolCfg := worker.PoolConfig{
		Config:     cfg,
		WorkerType: workerType,
		DB:         db,
		NATS:       natsClient,
	}

	pool, err := worker.NewPool(poolCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create worker pool")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info().Msg("worker pool is shutting down...")
		cancel()
	}()

	log.Info().Str("type", workerType).Msg("starting worker pool")
	if err := pool.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("worker pool error")
	}

	log.Info().Msg("worker pool stopped")
}
