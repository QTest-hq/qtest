package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database operations
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new store
func NewStore(db *DB) *Store {
	return &Store{pool: db.Pool()}
}

// Ping verifies database connectivity
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Repository represents a repository record
type Repository struct {
	ID            uuid.UUID `json:"id"`
	URL           string    `json:"url"`
	Name          string    `json:"name"`
	Owner         string    `json:"owner"`
	DefaultBranch string    `json:"default_branch"`
	Language      *string   `json:"language,omitempty"`
	LastCommitSHA *string   `json:"last_commit_sha,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SystemModel represents a system model record
type SystemModel struct {
	ID           uuid.UUID       `json:"id"`
	RepositoryID uuid.UUID       `json:"repository_id"`
	CommitSHA    string          `json:"commit_sha"`
	ModelData    json.RawMessage `json:"model_data"`
	CreatedAt    time.Time       `json:"created_at"`
}

// GenerationRun represents a test generation run
type GenerationRun struct {
	ID            uuid.UUID        `json:"id"`
	RepositoryID  uuid.UUID        `json:"repository_id"`
	SystemModelID *uuid.UUID       `json:"system_model_id,omitempty"`
	Status        string           `json:"status"`
	Config        json.RawMessage  `json:"config"`
	Summary       *json.RawMessage `json:"summary,omitempty"`
	StartedAt     *time.Time       `json:"started_at,omitempty"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// GeneratedTest represents a generated test
type GeneratedTest struct {
	ID              uuid.UUID        `json:"id"`
	RunID           uuid.UUID        `json:"run_id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	TargetFile      string           `json:"target_file"`
	TargetFunction  *string          `json:"target_function,omitempty"`
	DSL             json.RawMessage  `json:"dsl"`
	GeneratedCode   *string          `json:"generated_code,omitempty"`
	Framework       *string          `json:"framework,omitempty"`
	Status          string           `json:"status"`
	RejectionReason *string          `json:"rejection_reason,omitempty"`
	MutationScore   *float64         `json:"mutation_score,omitempty"`
	Metadata        *json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// CreateRepository creates a new repository
func (s *Store) CreateRepository(ctx context.Context, repo *Repository) error {
	repo.ID = uuid.New()
	repo.Status = "pending"
	repo.CreatedAt = time.Now()
	repo.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO repositories (id, url, name, owner, default_branch, language, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, repo.ID, repo.URL, repo.Name, repo.Owner, repo.DefaultBranch, repo.Language, repo.Status, repo.CreatedAt, repo.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	return nil
}

// GetRepository gets a repository by ID
func (s *Store) GetRepository(ctx context.Context, id uuid.UUID) (*Repository, error) {
	repo := &Repository{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, url, name, owner, default_branch, language, last_commit_sha, status, created_at, updated_at
		FROM repositories WHERE id = $1
	`, id).Scan(&repo.ID, &repo.URL, &repo.Name, &repo.Owner, &repo.DefaultBranch, &repo.Language,
		&repo.LastCommitSHA, &repo.Status, &repo.CreatedAt, &repo.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repo, nil
}

// GetRepositoryByURL gets a repository by URL
func (s *Store) GetRepositoryByURL(ctx context.Context, url string) (*Repository, error) {
	repo := &Repository{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, url, name, owner, default_branch, language, last_commit_sha, status, created_at, updated_at
		FROM repositories WHERE url = $1
	`, url).Scan(&repo.ID, &repo.URL, &repo.Name, &repo.Owner, &repo.DefaultBranch, &repo.Language,
		&repo.LastCommitSHA, &repo.Status, &repo.CreatedAt, &repo.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repo, nil
}

// ListRepositories lists all repositories
func (s *Store) ListRepositories(ctx context.Context, limit, offset int) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, name, owner, default_branch, language, last_commit_sha, status, created_at, updated_at
		FROM repositories
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()

	repos := make([]Repository, 0)
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.URL, &repo.Name, &repo.Owner, &repo.DefaultBranch,
			&repo.Language, &repo.LastCommitSHA, &repo.Status, &repo.CreatedAt, &repo.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// UpdateRepositoryStatus updates repository status
func (s *Store) UpdateRepositoryStatus(ctx context.Context, id uuid.UUID, status string, commitSHA *string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE repositories SET status = $2, last_commit_sha = $3, updated_at = $4
		WHERE id = $1
	`, id, status, commitSHA, time.Now())
	return err
}

// CreateGenerationRun creates a new generation run
func (s *Store) CreateGenerationRun(ctx context.Context, run *GenerationRun) error {
	run.ID = uuid.New()
	run.Status = "pending"
	run.CreatedAt = time.Now()

	if run.Config == nil {
		run.Config = json.RawMessage(`{}`)
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO generation_runs (id, repository_id, system_model_id, status, config, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, run.ID, run.RepositoryID, run.SystemModelID, run.Status, run.Config, run.CreatedAt)

	return err
}

// GetGenerationRun gets a generation run by ID
func (s *Store) GetGenerationRun(ctx context.Context, id uuid.UUID) (*GenerationRun, error) {
	run := &GenerationRun{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, repository_id, system_model_id, status, config, summary, started_at, completed_at, created_at
		FROM generation_runs WHERE id = $1
	`, id).Scan(&run.ID, &run.RepositoryID, &run.SystemModelID, &run.Status, &run.Config,
		&run.Summary, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	return run, nil
}

// UpdateGenerationRunStatus updates a run's status
func (s *Store) UpdateGenerationRunStatus(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	var startedAt, completedAt *time.Time

	if status == "running" {
		startedAt = &now
	}
	if status == "completed" || status == "failed" {
		completedAt = &now
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE generation_runs
		SET status = $2,
		    started_at = COALESCE($3, started_at),
		    completed_at = COALESCE($4, completed_at)
		WHERE id = $1
	`, id, status, startedAt, completedAt)

	return err
}

// CreateGeneratedTest creates a new generated test
func (s *Store) CreateGeneratedTest(ctx context.Context, test *GeneratedTest) error {
	test.ID = uuid.New()
	test.Status = "pending"
	test.CreatedAt = time.Now()
	test.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO generated_tests (id, run_id, name, type, target_file, target_function, dsl, framework, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, test.ID, test.RunID, test.Name, test.Type, test.TargetFile, test.TargetFunction,
		test.DSL, test.Framework, test.Status, test.CreatedAt, test.UpdatedAt)

	return err
}

// ListTestsByRun lists all tests for a run
func (s *Store) ListTestsByRun(ctx context.Context, runID uuid.UUID) ([]GeneratedTest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, name, type, target_file, target_function, dsl, generated_code,
		       framework, status, rejection_reason, mutation_score, metadata, created_at, updated_at
		FROM generated_tests
		WHERE run_id = $1
		ORDER BY created_at
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tests: %w", err)
	}
	defer rows.Close()

	tests := make([]GeneratedTest, 0)
	for rows.Next() {
		var test GeneratedTest
		if err := rows.Scan(&test.ID, &test.RunID, &test.Name, &test.Type, &test.TargetFile,
			&test.TargetFunction, &test.DSL, &test.GeneratedCode, &test.Framework, &test.Status,
			&test.RejectionReason, &test.MutationScore, &test.Metadata, &test.CreatedAt, &test.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan test: %w", err)
		}
		tests = append(tests, test)
	}

	return tests, nil
}

// DeleteRepository deletes a repository and all related data (cascading)
func (s *Store) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	// The database schema has ON DELETE CASCADE, so this will delete related runs and tests
	result, err := s.pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("repository not found")
	}

	return nil
}

// GetTest gets a generated test by ID
func (s *Store) GetTest(ctx context.Context, id uuid.UUID) (*GeneratedTest, error) {
	test := &GeneratedTest{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, run_id, name, type, target_file, target_function, dsl, generated_code,
		       framework, status, rejection_reason, mutation_score, metadata, created_at, updated_at
		FROM generated_tests WHERE id = $1
	`, id).Scan(&test.ID, &test.RunID, &test.Name, &test.Type, &test.TargetFile,
		&test.TargetFunction, &test.DSL, &test.GeneratedCode, &test.Framework, &test.Status,
		&test.RejectionReason, &test.MutationScore, &test.Metadata, &test.CreatedAt, &test.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get test: %w", err)
	}

	return test, nil
}

// UpdateTestStatus updates the status of a generated test
func (s *Store) UpdateTestStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason *string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE generated_tests
		SET status = $2, rejection_reason = $3, updated_at = $4
		WHERE id = $1
	`, id, status, rejectionReason, time.Now())

	if err != nil {
		return fmt.Errorf("failed to update test status: %w", err)
	}

	return nil
}

// ListRunsByRepository lists all generation runs for a repository
func (s *Store) ListRunsByRepository(ctx context.Context, repoID uuid.UUID, limit, offset int) ([]GenerationRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, system_model_id, status, config, summary, started_at, completed_at, created_at
		FROM generation_runs
		WHERE repository_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, repoID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	runs := make([]GenerationRun, 0)
	for rows.Next() {
		var run GenerationRun
		if err := rows.Scan(&run.ID, &run.RepositoryID, &run.SystemModelID, &run.Status, &run.Config,
			&run.Summary, &run.StartedAt, &run.CompletedAt, &run.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}
