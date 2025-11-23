// Package codecov provides coverage collection and storage.
package codecov

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CoverageSnapshot represents a stored coverage snapshot
type CoverageSnapshot struct {
	ID               uuid.UUID       `json:"id" db:"id"`
	RepositoryID     uuid.UUID       `json:"repository_id" db:"repository_id"`
	CommitSHA        string          `json:"commit_sha,omitempty" db:"commit_sha"`
	Branch           string          `json:"branch,omitempty" db:"branch"`
	RunID            *uuid.UUID      `json:"run_id,omitempty" db:"run_id"`
	Language         string          `json:"language" db:"language"`
	TotalLines       int             `json:"total_lines" db:"total_lines"`
	CoveredLines     int             `json:"covered_lines" db:"covered_lines"`
	CoveragePercent  float64         `json:"coverage_percent" db:"coverage_percent"`
	PreviousSnapshot *uuid.UUID      `json:"previous_snapshot_id,omitempty" db:"previous_snapshot_id"`
	LinesDelta       int             `json:"lines_delta" db:"lines_delta"`
	CoverageDelta    float64         `json:"coverage_delta" db:"coverage_delta"`
	FileCoverage     []FileCoverage  `json:"file_coverage" db:"-"`
	UncoveredItems   []UncoveredItem `json:"uncovered_items" db:"-"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	Metadata         map[string]any  `json:"metadata,omitempty" db:"-"`
}

// CoverageTrend represents a daily coverage trend point
type CoverageTrend struct {
	Date          time.Time `json:"date"`
	AvgCoverage   float64   `json:"avg_coverage"`
	SnapshotCount int       `json:"snapshot_count"`
}

// CoverageSummary provides summary statistics for coverage
type CoverageSummary struct {
	RepositoryID    uuid.UUID       `json:"repository_id"`
	LatestSnapshot  *CoverageSnapshot `json:"latest_snapshot,omitempty"`
	TotalSnapshots  int             `json:"total_snapshots"`
	AvgCoverage     float64         `json:"avg_coverage"`
	MinCoverage     float64         `json:"min_coverage"`
	MaxCoverage     float64         `json:"max_coverage"`
	CoverageTrend   string          `json:"coverage_trend"` // "improving", "stable", "declining"
	TrendPercentage float64         `json:"trend_percentage"`
}

// Store provides storage operations for coverage snapshots
type Store struct {
	db *sql.DB
}

// NewStore creates a new coverage store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveSnapshot saves a coverage snapshot to the database
func (s *Store) SaveSnapshot(ctx context.Context, snapshot *CoverageSnapshot) error {
	if snapshot.ID == uuid.Nil {
		snapshot.ID = uuid.New()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}

	fileCovJSON, err := json.Marshal(snapshot.FileCoverage)
	if err != nil {
		return err
	}

	uncoveredJSON, err := json.Marshal(snapshot.UncoveredItems)
	if err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO coverage_snapshots (
			id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	_, err = s.db.ExecContext(ctx, query,
		snapshot.ID, snapshot.RepositoryID, snapshot.CommitSHA, snapshot.Branch, snapshot.RunID,
		snapshot.Language, snapshot.TotalLines, snapshot.CoveredLines, snapshot.CoveragePercent,
		snapshot.PreviousSnapshot, snapshot.LinesDelta, snapshot.CoverageDelta,
		fileCovJSON, uncoveredJSON, snapshot.CreatedAt, metadataJSON,
	)
	return err
}

// GetSnapshot retrieves a snapshot by ID
func (s *Store) GetSnapshot(ctx context.Context, id uuid.UUID) (*CoverageSnapshot, error) {
	query := `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots
		WHERE id = $1
	`

	var snapshot CoverageSnapshot
	var fileCovJSON, uncoveredJSON, metadataJSON []byte
	var commitSHA, branch sql.NullString
	var runID, prevSnapshot sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&snapshot.ID, &snapshot.RepositoryID, &commitSHA, &branch, &runID,
		&snapshot.Language, &snapshot.TotalLines, &snapshot.CoveredLines, &snapshot.CoveragePercent,
		&prevSnapshot, &snapshot.LinesDelta, &snapshot.CoverageDelta,
		&fileCovJSON, &uncoveredJSON, &snapshot.CreatedAt, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	snapshot.CommitSHA = commitSHA.String
	snapshot.Branch = branch.String
	if runID.Valid {
		rid, _ := uuid.Parse(runID.String)
		snapshot.RunID = &rid
	}
	if prevSnapshot.Valid {
		pid, _ := uuid.Parse(prevSnapshot.String)
		snapshot.PreviousSnapshot = &pid
	}

	json.Unmarshal(fileCovJSON, &snapshot.FileCoverage)
	json.Unmarshal(uncoveredJSON, &snapshot.UncoveredItems)
	json.Unmarshal(metadataJSON, &snapshot.Metadata)

	return &snapshot, nil
}

// GetLatestSnapshot retrieves the most recent snapshot for a repository
func (s *Store) GetLatestSnapshot(ctx context.Context, repoID uuid.UUID) (*CoverageSnapshot, error) {
	query := `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots
		WHERE repository_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var snapshot CoverageSnapshot
	var fileCovJSON, uncoveredJSON, metadataJSON []byte
	var commitSHA, branch sql.NullString
	var runID, prevSnapshot sql.NullString

	err := s.db.QueryRowContext(ctx, query, repoID).Scan(
		&snapshot.ID, &snapshot.RepositoryID, &commitSHA, &branch, &runID,
		&snapshot.Language, &snapshot.TotalLines, &snapshot.CoveredLines, &snapshot.CoveragePercent,
		&prevSnapshot, &snapshot.LinesDelta, &snapshot.CoverageDelta,
		&fileCovJSON, &uncoveredJSON, &snapshot.CreatedAt, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	snapshot.CommitSHA = commitSHA.String
	snapshot.Branch = branch.String
	if runID.Valid {
		rid, _ := uuid.Parse(runID.String)
		snapshot.RunID = &rid
	}
	if prevSnapshot.Valid {
		pid, _ := uuid.Parse(prevSnapshot.String)
		snapshot.PreviousSnapshot = &pid
	}

	json.Unmarshal(fileCovJSON, &snapshot.FileCoverage)
	json.Unmarshal(uncoveredJSON, &snapshot.UncoveredItems)
	json.Unmarshal(metadataJSON, &snapshot.Metadata)

	return &snapshot, nil
}

// ListSnapshots retrieves snapshots for a repository with pagination
func (s *Store) ListSnapshots(ctx context.Context, repoID uuid.UUID, limit, offset int) ([]*CoverageSnapshot, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, repository_id, commit_sha, branch, run_id, language,
			total_lines, covered_lines, coverage_percent,
			previous_snapshot_id, lines_delta, coverage_delta,
			file_coverage, uncovered_items, created_at, metadata
		FROM coverage_snapshots
		WHERE repository_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*CoverageSnapshot
	for rows.Next() {
		var snapshot CoverageSnapshot
		var fileCovJSON, uncoveredJSON, metadataJSON []byte
		var commitSHA, branch sql.NullString
		var runID, prevSnapshot sql.NullString

		err := rows.Scan(
			&snapshot.ID, &snapshot.RepositoryID, &commitSHA, &branch, &runID,
			&snapshot.Language, &snapshot.TotalLines, &snapshot.CoveredLines, &snapshot.CoveragePercent,
			&prevSnapshot, &snapshot.LinesDelta, &snapshot.CoverageDelta,
			&fileCovJSON, &uncoveredJSON, &snapshot.CreatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		snapshot.CommitSHA = commitSHA.String
		snapshot.Branch = branch.String
		if runID.Valid {
			rid, _ := uuid.Parse(runID.String)
			snapshot.RunID = &rid
		}
		if prevSnapshot.Valid {
			pid, _ := uuid.Parse(prevSnapshot.String)
			snapshot.PreviousSnapshot = &pid
		}

		json.Unmarshal(fileCovJSON, &snapshot.FileCoverage)
		json.Unmarshal(uncoveredJSON, &snapshot.UncoveredItems)
		json.Unmarshal(metadataJSON, &snapshot.Metadata)

		snapshots = append(snapshots, &snapshot)
	}

	return snapshots, nil
}

// GetCoverageTrend retrieves coverage trend over the specified number of days
func (s *Store) GetCoverageTrend(ctx context.Context, repoID uuid.UUID, days int) ([]CoverageTrend, error) {
	if days <= 0 {
		days = 30
	}

	query := `
		SELECT DATE(created_at) AS snapshot_date,
			AVG(coverage_percent) AS avg_coverage,
			COUNT(*) AS snapshot_count
		FROM coverage_snapshots
		WHERE repository_id = $1
		  AND created_at >= CURRENT_DATE - $2 * INTERVAL '1 day'
		GROUP BY DATE(created_at)
		ORDER BY snapshot_date DESC
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []CoverageTrend
	for rows.Next() {
		var trend CoverageTrend
		if err := rows.Scan(&trend.Date, &trend.AvgCoverage, &trend.SnapshotCount); err != nil {
			return nil, err
		}
		trends = append(trends, trend)
	}

	return trends, nil
}

// GetSummary returns coverage summary for a repository
func (s *Store) GetSummary(ctx context.Context, repoID uuid.UUID) (*CoverageSummary, error) {
	summary := &CoverageSummary{
		RepositoryID: repoID,
	}

	// Get latest snapshot
	latest, err := s.GetLatestSnapshot(ctx, repoID)
	if err != nil {
		return nil, err
	}
	summary.LatestSnapshot = latest

	// Get aggregate stats
	statsQuery := `
		SELECT COUNT(*), COALESCE(AVG(coverage_percent), 0),
			COALESCE(MIN(coverage_percent), 0), COALESCE(MAX(coverage_percent), 0)
		FROM coverage_snapshots
		WHERE repository_id = $1
	`
	err = s.db.QueryRowContext(ctx, statsQuery, repoID).Scan(
		&summary.TotalSnapshots, &summary.AvgCoverage,
		&summary.MinCoverage, &summary.MaxCoverage,
	)
	if err != nil {
		return nil, err
	}

	// Calculate trend (compare last 7 days to previous 7 days)
	trendQuery := `
		WITH recent AS (
			SELECT AVG(coverage_percent) AS avg
			FROM coverage_snapshots
			WHERE repository_id = $1 AND created_at >= CURRENT_DATE - 7 * INTERVAL '1 day'
		),
		previous AS (
			SELECT AVG(coverage_percent) AS avg
			FROM coverage_snapshots
			WHERE repository_id = $1
			  AND created_at >= CURRENT_DATE - 14 * INTERVAL '1 day'
			  AND created_at < CURRENT_DATE - 7 * INTERVAL '1 day'
		)
		SELECT COALESCE(r.avg, 0) - COALESCE(p.avg, 0)
		FROM recent r, previous p
	`
	var trendDiff float64
	if err := s.db.QueryRowContext(ctx, trendQuery, repoID).Scan(&trendDiff); err == nil {
		summary.TrendPercentage = trendDiff
		if trendDiff > 1 {
			summary.CoverageTrend = "improving"
		} else if trendDiff < -1 {
			summary.CoverageTrend = "declining"
		} else {
			summary.CoverageTrend = "stable"
		}
	} else {
		summary.CoverageTrend = "unknown"
	}

	return summary, nil
}

// DeleteSnapshot deletes a snapshot by ID
func (s *Store) DeleteSnapshot(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM coverage_snapshots WHERE id = $1", id)
	return err
}

// DeleteOldSnapshots deletes snapshots older than the specified duration
func (s *Store) DeleteOldSnapshots(ctx context.Context, repoID uuid.UUID, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM coverage_snapshots WHERE repository_id = $1 AND created_at < $2",
		repoID, cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CreateSnapshotFromReport creates a CoverageSnapshot from a CoverageReport
func CreateSnapshotFromReport(repoID uuid.UUID, report *CoverageReport) *CoverageSnapshot {
	return &CoverageSnapshot{
		ID:              uuid.New(),
		RepositoryID:    repoID,
		Language:        report.Language,
		TotalLines:      report.TotalLines,
		CoveredLines:    report.CoveredLines,
		CoveragePercent: report.Percentage,
		FileCoverage:    report.Files,
		UncoveredItems:  report.Uncovered,
		CreatedAt:       report.Timestamp,
	}
}

// SaveSnapshotWithDelta saves a snapshot and calculates delta from the previous one
func (s *Store) SaveSnapshotWithDelta(ctx context.Context, snapshot *CoverageSnapshot) error {
	// Get the previous snapshot to calculate delta
	prev, err := s.GetLatestSnapshot(ctx, snapshot.RepositoryID)
	if err != nil {
		return err
	}

	if prev != nil {
		snapshot.PreviousSnapshot = &prev.ID
		snapshot.LinesDelta = snapshot.CoveredLines - prev.CoveredLines
		snapshot.CoverageDelta = snapshot.CoveragePercent - prev.CoveragePercent
	}

	return s.SaveSnapshot(ctx, snapshot)
}
