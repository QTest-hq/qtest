package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/QTest-hq/qtest/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

// DB wraps the database connection pool
type DB struct {
	pool         *pgxpool.Pool
	metricsStop  chan struct{}
}

// New creates a new database connection
func New(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().Str("host", config.ConnConfig.Host).Msg("connected to database")

	db := &DB{
		pool:        pool,
		metricsStop: make(chan struct{}),
	}

	// Start metrics collection goroutine
	go db.collectMetrics()

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() {
	// Stop metrics collection
	close(db.metricsStop)
	db.pool.Close()
}

// Pool returns the underlying connection pool
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// HealthCheck verifies database connectivity
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// StdDB returns a *sql.DB that uses the underlying pgx pool.
// This allows code that requires database/sql interface to share
// the same connection pool, avoiding duplicate connections.
// The returned *sql.DB should NOT be closed separately - it will
// be closed when the parent DB is closed.
func (db *DB) StdDB() *sql.DB {
	return stdlib.OpenDBFromPool(db.pool)
}

// collectMetrics periodically updates Prometheus metrics with pool stats
func (db *DB) collectMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-db.metricsStop:
			return
		case <-ticker.C:
			db.updateMetrics()
		}
	}
}

// updateMetrics updates Prometheus gauges with current pool statistics
func (db *DB) updateMetrics() {
	stats := db.pool.Stat()

	metrics.DBConnectionsOpen.Set(float64(stats.TotalConns()))
	metrics.DBConnectionsInUse.Set(float64(stats.AcquiredConns()))
	metrics.DBConnectionsIdle.Set(float64(stats.IdleConns()))
	metrics.DBConnectionsMaxOpen.Set(float64(stats.MaxConns()))
}
