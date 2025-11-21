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
	"github.com/QTest-hq/qtest/internal/db"
	"github.com/QTest-hq/qtest/internal/llm"
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
	var dbConn *sql.DB
	if cfg.DatabaseURL != "" {
		dbConn, err = sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			log.Warn().Err(err).Msg("failed to connect to database, workers will run in limited mode")
		} else if err := dbConn.Ping(); err != nil {
			log.Warn().Err(err).Msg("database ping failed, workers will run in limited mode")
			dbConn.Close()
			dbConn = nil
		} else {
			log.Info().Msg("connected to database")
			defer dbConn.Close()
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

	// Create database store for domain operations (uses pgxpool)
	var store *db.Store
	if cfg.DatabaseURL != "" {
		pgDB, dbErr := db.New(context.Background(), cfg.DatabaseURL)
		if dbErr != nil {
			log.Warn().Err(dbErr).Msg("failed to create pgx pool for store, some features disabled")
		} else {
			store = db.NewStore(pgDB)
			log.Info().Msg("database store initialized")
			defer pgDB.Close()
		}
	}

	// Create LLM router for test generation
	var llmRouter *llm.Router
	llmRouter, err = llm.NewRouter(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("failed to create LLM router, generation workers will run in limited mode")
	} else {
		log.Info().Msg("LLM router initialized")
	}

	// Create worker pool
	poolCfg := worker.PoolConfig{
		Config:     cfg,
		WorkerType: workerType,
		DB:         dbConn,
		NATS:       natsClient,
		Store:      store,
		LLMRouter:  llmRouter,
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
