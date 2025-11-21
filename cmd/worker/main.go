package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/QTest-hq/qtest/internal/config"
	"github.com/QTest-hq/qtest/internal/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	// Create worker pool
	pool, err := worker.NewPool(cfg, workerType)
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
