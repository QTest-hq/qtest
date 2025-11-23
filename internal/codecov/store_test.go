package codecov

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateSnapshotFromReport(t *testing.T) {
	repoID := uuid.New()
	report := &CoverageReport{
		Timestamp:    time.Now(),
		Language:     "go",
		TotalLines:   100,
		CoveredLines: 80,
		Percentage:   80.0,
		Files: []FileCoverage{
			{Path: "main.go", TotalLines: 50, CoveredLines: 40, Percentage: 80.0},
			{Path: "util.go", TotalLines: 50, CoveredLines: 40, Percentage: 80.0},
		},
		Uncovered: []UncoveredItem{
			{File: "main.go", StartLine: 10, EndLine: 15, Type: "line"},
		},
	}

	snapshot := CreateSnapshotFromReport(repoID, report)

	assert.NotEqual(t, uuid.Nil, snapshot.ID)
	assert.Equal(t, repoID, snapshot.RepositoryID)
	assert.Equal(t, "go", snapshot.Language)
	assert.Equal(t, 100, snapshot.TotalLines)
	assert.Equal(t, 80, snapshot.CoveredLines)
	assert.Equal(t, 80.0, snapshot.CoveragePercent)
	assert.Len(t, snapshot.FileCoverage, 2)
	assert.Len(t, snapshot.UncoveredItems, 1)
}

func TestCoverageSnapshot_Fields(t *testing.T) {
	repoID := uuid.New()
	runID := uuid.New()
	prevID := uuid.New()

	snapshot := CoverageSnapshot{
		ID:               uuid.New(),
		RepositoryID:     repoID,
		CommitSHA:        "abc123",
		Branch:           "main",
		RunID:            &runID,
		Language:         "python",
		TotalLines:       200,
		CoveredLines:     150,
		CoveragePercent:  75.0,
		PreviousSnapshot: &prevID,
		LinesDelta:       10,
		CoverageDelta:    2.5,
		FileCoverage: []FileCoverage{
			{Path: "app.py", TotalLines: 100, CoveredLines: 80, Percentage: 80.0},
		},
		UncoveredItems: []UncoveredItem{},
		CreatedAt:      time.Now(),
		Metadata:       map[string]any{"version": "1.0"},
	}

	assert.Equal(t, repoID, snapshot.RepositoryID)
	assert.Equal(t, "abc123", snapshot.CommitSHA)
	assert.Equal(t, "main", snapshot.Branch)
	assert.Equal(t, &runID, snapshot.RunID)
	assert.Equal(t, "python", snapshot.Language)
	assert.Equal(t, 200, snapshot.TotalLines)
	assert.Equal(t, 150, snapshot.CoveredLines)
	assert.Equal(t, 75.0, snapshot.CoveragePercent)
	assert.Equal(t, &prevID, snapshot.PreviousSnapshot)
	assert.Equal(t, 10, snapshot.LinesDelta)
	assert.Equal(t, 2.5, snapshot.CoverageDelta)
	assert.Len(t, snapshot.FileCoverage, 1)
	assert.Equal(t, "1.0", snapshot.Metadata["version"])
}

func TestCoverageTrend_Fields(t *testing.T) {
	trend := CoverageTrend{
		Date:          time.Now(),
		AvgCoverage:   75.5,
		SnapshotCount: 5,
	}

	assert.Equal(t, 75.5, trend.AvgCoverage)
	assert.Equal(t, 5, trend.SnapshotCount)
}

func TestCoverageSummary_Fields(t *testing.T) {
	repoID := uuid.New()
	snapshot := &CoverageSnapshot{
		ID:              uuid.New(),
		RepositoryID:    repoID,
		CoveragePercent: 80.0,
	}

	summary := CoverageSummary{
		RepositoryID:    repoID,
		LatestSnapshot:  snapshot,
		TotalSnapshots:  10,
		AvgCoverage:     75.0,
		MinCoverage:     60.0,
		MaxCoverage:     85.0,
		CoverageTrend:   "improving",
		TrendPercentage: 2.5,
	}

	assert.Equal(t, repoID, summary.RepositoryID)
	assert.NotNil(t, summary.LatestSnapshot)
	assert.Equal(t, 10, summary.TotalSnapshots)
	assert.Equal(t, 75.0, summary.AvgCoverage)
	assert.Equal(t, 60.0, summary.MinCoverage)
	assert.Equal(t, 85.0, summary.MaxCoverage)
	assert.Equal(t, "improving", summary.CoverageTrend)
	assert.Equal(t, 2.5, summary.TrendPercentage)
}

func TestNewStore(t *testing.T) {
	// Test that NewStore can be called with nil (will be replaced in integration tests)
	store := NewStore(nil)
	assert.NotNil(t, store)
}

func TestCreateSnapshotFromReport_EmptyReport(t *testing.T) {
	repoID := uuid.New()
	report := &CoverageReport{
		Timestamp:    time.Now(),
		Language:     "javascript",
		TotalLines:   0,
		CoveredLines: 0,
		Percentage:   0,
		Files:        []FileCoverage{},
		Uncovered:    []UncoveredItem{},
	}

	snapshot := CreateSnapshotFromReport(repoID, report)

	assert.NotEqual(t, uuid.Nil, snapshot.ID)
	assert.Equal(t, "javascript", snapshot.Language)
	assert.Equal(t, 0, snapshot.TotalLines)
	assert.Equal(t, 0.0, snapshot.CoveragePercent)
	assert.Empty(t, snapshot.FileCoverage)
}

func TestCoverageSummary_TrendTypes(t *testing.T) {
	tests := []struct {
		trend    string
		expected string
	}{
		{"improving", "improving"},
		{"stable", "stable"},
		{"declining", "declining"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		summary := CoverageSummary{CoverageTrend: tt.trend}
		assert.Equal(t, tt.expected, summary.CoverageTrend)
	}
}

func TestCreateSnapshotFromReport_WithUncovered(t *testing.T) {
	repoID := uuid.New()
	report := &CoverageReport{
		Timestamp:    time.Now(),
		Language:     "go",
		TotalLines:   100,
		CoveredLines: 70,
		Percentage:   70.0,
		Files: []FileCoverage{
			{
				Path:           "main.go",
				TotalLines:     100,
				CoveredLines:   70,
				Percentage:     70.0,
				UncoveredLines: []int{10, 11, 12, 20, 21},
			},
		},
		Uncovered: []UncoveredItem{
			{File: "main.go", StartLine: 10, EndLine: 12, Type: "line"},
			{File: "main.go", StartLine: 20, EndLine: 21, Type: "branch"},
			{File: "main.go", StartLine: 30, EndLine: 30, Type: "function", Name: "unusedFunc"},
		},
	}

	snapshot := CreateSnapshotFromReport(repoID, report)

	assert.Len(t, snapshot.UncoveredItems, 3)
	assert.Equal(t, "line", snapshot.UncoveredItems[0].Type)
	assert.Equal(t, "branch", snapshot.UncoveredItems[1].Type)
	assert.Equal(t, "function", snapshot.UncoveredItems[2].Type)
	assert.Equal(t, "unusedFunc", snapshot.UncoveredItems[2].Name)
}

func TestCoverageSnapshot_Metadata(t *testing.T) {
	snapshot := CoverageSnapshot{
		ID:       uuid.New(),
		Metadata: make(map[string]any),
	}

	snapshot.Metadata["ci_build_id"] = "12345"
	snapshot.Metadata["triggered_by"] = "push"
	snapshot.Metadata["extra"] = map[string]string{"key": "value"}

	assert.Equal(t, "12345", snapshot.Metadata["ci_build_id"])
	assert.Equal(t, "push", snapshot.Metadata["triggered_by"])
}

func TestCoverageSnapshot_NilFields(t *testing.T) {
	snapshot := CoverageSnapshot{
		ID:           uuid.New(),
		RepositoryID: uuid.New(),
		Language:     "go",
	}

	assert.Nil(t, snapshot.RunID)
	assert.Nil(t, snapshot.PreviousSnapshot)
	assert.Empty(t, snapshot.CommitSHA)
	assert.Empty(t, snapshot.Branch)
}
