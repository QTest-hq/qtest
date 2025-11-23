-- Migration 008: Add coverage snapshots
-- Stores historical coverage data for tracking trends and deltas

-- Coverage snapshots table
CREATE TABLE IF NOT EXISTS coverage_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,

    -- Snapshot identification
    commit_sha VARCHAR(40),
    branch VARCHAR(255),
    run_id UUID REFERENCES generation_runs(id) ON DELETE SET NULL,

    -- Coverage metrics
    language VARCHAR(50) NOT NULL,
    total_lines INTEGER NOT NULL DEFAULT 0,
    covered_lines INTEGER NOT NULL DEFAULT 0,
    coverage_percent DECIMAL(5, 2) NOT NULL DEFAULT 0,

    -- Delta from previous snapshot
    previous_snapshot_id UUID REFERENCES coverage_snapshots(id) ON DELETE SET NULL,
    lines_delta INTEGER DEFAULT 0,
    coverage_delta DECIMAL(5, 2) DEFAULT 0,

    -- Detailed per-file coverage (JSON)
    file_coverage JSONB DEFAULT '[]'::jsonb,
    uncovered_items JSONB DEFAULT '[]'::jsonb,

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_coverage_snapshots_repo_id ON coverage_snapshots(repository_id);
CREATE INDEX IF NOT EXISTS idx_coverage_snapshots_commit_sha ON coverage_snapshots(commit_sha);
CREATE INDEX IF NOT EXISTS idx_coverage_snapshots_branch ON coverage_snapshots(branch);
CREATE INDEX IF NOT EXISTS idx_coverage_snapshots_created_at ON coverage_snapshots(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coverage_snapshots_run_id ON coverage_snapshots(run_id);

-- Function to get latest snapshot for a repository
CREATE OR REPLACE FUNCTION get_latest_coverage_snapshot(repo_id UUID)
RETURNS coverage_snapshots AS $$
    SELECT * FROM coverage_snapshots
    WHERE repository_id = repo_id
    ORDER BY created_at DESC
    LIMIT 1
$$ LANGUAGE SQL STABLE;

-- Function to calculate coverage trend over time
CREATE OR REPLACE FUNCTION get_coverage_trend(repo_id UUID, days INTEGER DEFAULT 30)
RETURNS TABLE (
    snapshot_date DATE,
    avg_coverage DECIMAL(5, 2),
    snapshot_count INTEGER
) AS $$
    SELECT
        DATE(created_at) AS snapshot_date,
        AVG(coverage_percent)::DECIMAL(5, 2) AS avg_coverage,
        COUNT(*)::INTEGER AS snapshot_count
    FROM coverage_snapshots
    WHERE repository_id = repo_id
      AND created_at >= CURRENT_DATE - days * INTERVAL '1 day'
    GROUP BY DATE(created_at)
    ORDER BY snapshot_date DESC
$$ LANGUAGE SQL STABLE;

COMMENT ON TABLE coverage_snapshots IS 'Historical code coverage snapshots for tracking trends';
COMMENT ON COLUMN coverage_snapshots.file_coverage IS 'JSON array of per-file coverage data';
COMMENT ON COLUMN coverage_snapshots.uncovered_items IS 'JSON array of uncovered code sections';
COMMENT ON COLUMN coverage_snapshots.coverage_delta IS 'Change in coverage percent from previous snapshot';
