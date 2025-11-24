package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CoverageSnapshot represents a point-in-time coverage measurement
type CoverageSnapshot struct {
	ID                 uuid.UUID       `json:"id"`
	RepositoryID       uuid.UUID       `json:"repository_id"`
	CommitSHA          *string         `json:"commit_sha,omitempty"`
	Branch             *string         `json:"branch,omitempty"`
	RunID              *uuid.UUID      `json:"run_id,omitempty"`
	Language           string          `json:"language"`
	TotalLines         int             `json:"total_lines"`
	CoveredLines       int             `json:"covered_lines"`
	CoveragePercent    float64         `json:"coverage_percent"`
	PreviousSnapshotID *uuid.UUID      `json:"previous_snapshot_id,omitempty"`
	LinesDelta         int             `json:"lines_delta"`
	CoverageDelta      float64         `json:"coverage_delta"`
	FileCoverage       json.RawMessage `json:"file_coverage"`
	UncoveredItems     json.RawMessage `json:"uncovered_items"`
	CreatedAt          time.Time       `json:"created_at"`
	Metadata           json.RawMessage `json:"metadata"`
}

// FileCoverageDetail represents per-file coverage info
type FileCoverageDetail struct {
	Path            string  `json:"path"`
	TotalLines      int     `json:"total_lines"`
	CoveredLines    int     `json:"covered_lines"`
	CoveragePercent float64 `json:"coverage_percent"`
	UncoveredLines  []int   `json:"uncovered_lines,omitempty"`
}

// CoverageTrend represents aggregated coverage over time
type CoverageTrend struct {
	Date          string  `json:"date"`
	AvgCoverage   float64 `json:"avg_coverage"`
	SnapshotCount int     `json:"snapshot_count"`
}

// CoverageSummary provides overall coverage statistics
type CoverageSummary struct {
	TotalRepos       int     `json:"total_repos"`
	AvgCoverage      float64 `json:"avg_coverage"`
	TotalLines       int     `json:"total_lines"`
	TotalCovered     int     `json:"total_covered"`
	ReposAbove80     int     `json:"repos_above_80"`
	ReposBelow50     int     `json:"repos_below_50"`
	TrendDirection   string  `json:"trend_direction"` // "up", "down", "stable"
	TrendDelta       float64 `json:"trend_delta"`
}

// CreateCoverageSnapshot creates a new coverage snapshot
func (s *Store) CreateCoverageSnapshot(ctx context.Context, snap *CoverageSnapshot) error {
	snap.ID = uuid.New()
	snap.CreatedAt = time.Now()

	if snap.FileCoverage == nil {
		snap.FileCoverage = json.RawMessage(`[]`)
	}
	if snap.UncoveredItems == nil {
		snap.UncoveredItems = json.RawMessage(`[]`)
	}
	if snap.Metadata == nil {
		snap.Metadata = json.RawMessage(`{}`)
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO coverage_snapshots (
			id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, snap.ID, snap.RepositoryID, snap.CommitSHA, snap.Branch, snap.RunID, snap.Language,
		snap.TotalLines, snap.CoveredLines, snap.CoveragePercent,
		snap.PreviousSnapshotID, snap.LinesDelta, snap.CoverageDelta,
		snap.FileCoverage, snap.UncoveredItems, snap.CreatedAt, snap.Metadata)

	if err != nil {
		return fmt.Errorf("failed to create coverage snapshot: %w", err)
	}
	return nil
}

// GetCoverageSnapshot retrieves a snapshot by ID
func (s *Store) GetCoverageSnapshot(ctx context.Context, id uuid.UUID) (*CoverageSnapshot, error) {
	snap := &CoverageSnapshot{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots WHERE id = $1
	`, id).Scan(&snap.ID, &snap.RepositoryID, &snap.CommitSHA, &snap.Branch, &snap.RunID, &snap.Language,
		&snap.TotalLines, &snap.CoveredLines, &snap.CoveragePercent,
		&snap.PreviousSnapshotID, &snap.LinesDelta, &snap.CoverageDelta,
		&snap.FileCoverage, &snap.UncoveredItems, &snap.CreatedAt, &snap.Metadata)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get coverage snapshot: %w", err)
	}
	return snap, nil
}

// GetLatestCoverageSnapshot gets the most recent snapshot for a repository
func (s *Store) GetLatestCoverageSnapshot(ctx context.Context, repoID uuid.UUID) (*CoverageSnapshot, error) {
	snap := &CoverageSnapshot{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots
		WHERE repository_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, repoID).Scan(&snap.ID, &snap.RepositoryID, &snap.CommitSHA, &snap.Branch, &snap.RunID, &snap.Language,
		&snap.TotalLines, &snap.CoveredLines, &snap.CoveragePercent,
		&snap.PreviousSnapshotID, &snap.LinesDelta, &snap.CoverageDelta,
		&snap.FileCoverage, &snap.UncoveredItems, &snap.CreatedAt, &snap.Metadata)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}
	return snap, nil
}

// ListCoverageSnapshots lists snapshots with optional filters
func (s *Store) ListCoverageSnapshots(ctx context.Context, repoID *uuid.UUID, limit int) ([]CoverageSnapshot, error) {
	query := `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots
	`
	var args []interface{}
	argIdx := 1

	if repoID != nil {
		query += fmt.Sprintf(" WHERE repository_id = $%d", argIdx)
		args = append(args, *repoID)
		argIdx++
	}

	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []CoverageSnapshot
	for rows.Next() {
		var snap CoverageSnapshot
		if err := rows.Scan(&snap.ID, &snap.RepositoryID, &snap.CommitSHA, &snap.Branch, &snap.RunID, &snap.Language,
			&snap.TotalLines, &snap.CoveredLines, &snap.CoveragePercent,
			&snap.PreviousSnapshotID, &snap.LinesDelta, &snap.CoverageDelta,
			&snap.FileCoverage, &snap.UncoveredItems, &snap.CreatedAt, &snap.Metadata); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

// GetCoverageTrend returns coverage trend over time
func (s *Store) GetCoverageTrend(ctx context.Context, repoID uuid.UUID, days int) ([]CoverageTrend, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			DATE(created_at)::TEXT AS snapshot_date,
			AVG(coverage_percent)::DECIMAL(5, 2) AS avg_coverage,
			COUNT(*)::INTEGER AS snapshot_count
		FROM coverage_snapshots
		WHERE repository_id = $1
		  AND created_at >= CURRENT_DATE - $2 * INTERVAL '1 day'
		GROUP BY DATE(created_at)
		ORDER BY snapshot_date DESC
	`, repoID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get coverage trend: %w", err)
	}
	defer rows.Close()

	var trends []CoverageTrend
	for rows.Next() {
		var t CoverageTrend
		if err := rows.Scan(&t.Date, &t.AvgCoverage, &t.SnapshotCount); err != nil {
			return nil, fmt.Errorf("failed to scan trend: %w", err)
		}
		trends = append(trends, t)
	}
	return trends, nil
}

// GetCoverageSummary returns overall coverage statistics
func (s *Store) GetCoverageSummary(ctx context.Context, orgID *uuid.UUID) (*CoverageSummary, error) {
	// Get latest snapshot per repo
	query := `
		WITH latest_snapshots AS (
			SELECT DISTINCT ON (repository_id)
				repository_id, coverage_percent, total_lines, covered_lines, coverage_delta
			FROM coverage_snapshots cs
			JOIN repositories r ON r.id = cs.repository_id
			WHERE ($1::UUID IS NULL OR r.organization_id = $1)
			ORDER BY repository_id, created_at DESC
		)
		SELECT
			COUNT(*) AS total_repos,
			COALESCE(AVG(coverage_percent), 0) AS avg_coverage,
			COALESCE(SUM(total_lines), 0) AS total_lines,
			COALESCE(SUM(covered_lines), 0) AS total_covered,
			COUNT(*) FILTER (WHERE coverage_percent >= 80) AS repos_above_80,
			COUNT(*) FILTER (WHERE coverage_percent < 50) AS repos_below_50,
			COALESCE(AVG(coverage_delta), 0) AS avg_delta
		FROM latest_snapshots
	`

	var summary CoverageSummary
	var avgDelta float64
	err := s.pool.QueryRow(ctx, query, orgID).Scan(
		&summary.TotalRepos, &summary.AvgCoverage, &summary.TotalLines,
		&summary.TotalCovered, &summary.ReposAbove80, &summary.ReposBelow50, &avgDelta,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get coverage summary: %w", err)
	}

	summary.TrendDelta = avgDelta
	if avgDelta > 0.5 {
		summary.TrendDirection = "up"
	} else if avgDelta < -0.5 {
		summary.TrendDirection = "down"
	} else {
		summary.TrendDirection = "stable"
	}

	return &summary, nil
}
