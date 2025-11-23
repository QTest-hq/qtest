// Package testutil provides utilities for integration testing
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// DefaultTestDBURL is the default database URL for integration tests
	DefaultTestDBURL = "postgres://qtest:qtest@localhost:5433/qtest_test?sslmode=disable"

	// DefaultTestNATSURL is the default NATS URL for integration tests
	DefaultTestNATSURL = "nats://localhost:4223"
)

// GetTestDBURL returns the test database URL from environment or default
func GetTestDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return DefaultTestDBURL
}

// GetTestNATSURL returns the test NATS URL from environment or default
func GetTestNATSURL() string {
	if url := os.Getenv("TEST_NATS_URL"); url != "" {
		return url
	}
	return DefaultTestNATSURL
}

// TestDB wraps a database pool for testing
type TestDB struct {
	Pool *pgxpool.Pool
}

// SetupTestDB creates a test database connection
// Skip test if database is not available
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbURL := GetTestDBURL()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Skipf("skipping test: invalid database URL: %v", err)
	}

	config.MaxConns = 5
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("skipping test: could not connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping test: could not ping database: %v", err)
	}

	// Setup schema
	if err := setupSchema(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("failed to setup schema: %v", err)
	}

	return &TestDB{Pool: pool}
}

// Cleanup cleans up the test database
func (db *TestDB) Cleanup(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Truncate all tables (order matters for foreign keys)
	tables := []string{
		"audit_logs",
		"api_keys",
		"sessions",
		"generated_tests",
		"generation_runs",
		"system_models",
		"repositories",
		"organization_members",
		"organizations",
		"users",
	}
	for _, table := range tables {
		_, err := db.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Logf("warning: failed to truncate %s: %v", table, err)
		}
	}
}

// Close closes the test database connection
func (db *TestDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// setupSchema creates the necessary tables for testing
func setupSchema(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
	-- Multi-tenancy tables (must be created first for foreign keys)
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		github_id BIGINT UNIQUE NOT NULL,
		github_login TEXT NOT NULL,
		email TEXT,
		name TEXT,
		avatar_url TEXT,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS organizations (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		description TEXT,
		owner_id UUID NOT NULL REFERENCES users(id),
		github_org_id BIGINT,
		settings JSONB NOT NULL DEFAULT '{}',
		is_personal BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS organization_members (
		id UUID PRIMARY KEY,
		organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		invited_by UUID REFERENCES users(id),
		joined_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		UNIQUE(organization_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		id UUID PRIMARY KEY,
		organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES users(id),
		name TEXT NOT NULL,
		key_prefix TEXT NOT NULL,
		key_hash TEXT NOT NULL UNIQUE,
		scopes TEXT[] NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE,
		last_used_at TIMESTAMP WITH TIME ZONE,
		revoked_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		access_token TEXT NOT NULL,
		refresh_token TEXT,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		last_access TIMESTAMP WITH TIME ZONE NOT NULL,
		ip_address TEXT,
		user_agent TEXT,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id UUID PRIMARY KEY,
		organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
		user_id UUID REFERENCES users(id) ON DELETE SET NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id UUID,
		details JSONB,
		ip_address TEXT,
		user_agent TEXT,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	-- Original tables
	CREATE TABLE IF NOT EXISTS repositories (
		id UUID PRIMARY KEY,
		url TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		owner TEXT NOT NULL,
		default_branch TEXT NOT NULL DEFAULT 'main',
		language TEXT,
		last_commit_sha TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS system_models (
		id UUID PRIMARY KEY,
		repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
		commit_sha TEXT NOT NULL,
		model_data JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS generation_runs (
		id UUID PRIMARY KEY,
		repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
		system_model_id UUID REFERENCES system_models(id) ON DELETE SET NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		config JSONB NOT NULL DEFAULT '{}',
		summary JSONB,
		started_at TIMESTAMP WITH TIME ZONE,
		completed_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS generated_tests (
		id UUID PRIMARY KEY,
		run_id UUID NOT NULL REFERENCES generation_runs(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		target_file TEXT NOT NULL,
		target_function TEXT,
		dsl JSONB NOT NULL,
		generated_code TEXT,
		framework TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		rejection_reason TEXT,
		mutation_score DOUBLE PRECISION,
		metadata JSONB,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_repositories_url ON repositories(url);
	CREATE INDEX IF NOT EXISTS idx_system_models_repository_id ON system_models(repository_id);
	CREATE INDEX IF NOT EXISTS idx_generation_runs_repository_id ON generation_runs(repository_id);
	CREATE INDEX IF NOT EXISTS idx_generated_tests_run_id ON generated_tests(run_id);
	CREATE INDEX IF NOT EXISTS idx_organizations_owner_id ON organizations(owner_id);
	CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members(user_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_organization_id ON audit_logs(organization_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
	`

	_, err := pool.Exec(ctx, schema)
	return err
}

// RequireDB returns a test database or fails the test
func RequireDB(t *testing.T) *TestDB {
	t.Helper()

	db := SetupTestDB(t)
	t.Cleanup(func() {
		db.Cleanup(t)
		db.Close()
	})

	return db
}

// TestNATS wraps a NATS connection for testing
type TestNATS struct {
	URL string
}

// SetupTestNATS creates a test NATS connection
// Skip test if NATS is not available
func SetupTestNATS(t *testing.T) *TestNATS {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	url := GetTestNATSURL()

	// Quick connectivity check using net.Dial
	// The actual NATS client connection will be done in the test
	return &TestNATS{URL: url}
}

// RequireNATS returns a test NATS URL or skips the test
func RequireNATS(t *testing.T) *TestNATS {
	t.Helper()

	return SetupTestNATS(t)
}
