-- Mutation runs table for tracking complete mutation testing sessions
CREATE TABLE IF NOT EXISTS mutation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID REFERENCES jobs(id) ON DELETE CASCADE,
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    generation_run_id UUID REFERENCES generation_runs(id) ON DELETE CASCADE,

    -- File paths
    source_file TEXT NOT NULL,
    test_file TEXT NOT NULL,

    -- Results summary
    total_mutants INTEGER NOT NULL DEFAULT 0,
    killed INTEGER NOT NULL DEFAULT 0,
    survived INTEGER NOT NULL DEFAULT 0,
    timeout INTEGER NOT NULL DEFAULT 0,
    score FLOAT NOT NULL DEFAULT 0.0,

    -- Quality assessment
    quality TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'poor', 'acceptable', 'good'

    -- Report storage
    report_data JSONB,  -- Full mutation result with mutant details
    report_file_path TEXT,

    -- Timing
    duration_ms INTEGER,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT valid_quality CHECK (quality IN ('pending', 'poor', 'acceptable', 'good'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_mutation_runs_job ON mutation_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_mutation_runs_repo ON mutation_runs(repository_id);
CREATE INDEX IF NOT EXISTS idx_mutation_runs_gen_run ON mutation_runs(generation_run_id);
CREATE INDEX IF NOT EXISTS idx_mutation_runs_quality ON mutation_runs(quality);
CREATE INDEX IF NOT EXISTS idx_mutation_runs_score ON mutation_runs(score);

-- Mutants table for storing individual mutant details (replaces/enhances mutation_results)
CREATE TABLE IF NOT EXISTS mutants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    mutation_run_id UUID NOT NULL REFERENCES mutation_runs(id) ON DELETE CASCADE,

    -- Mutant identification
    line_number INTEGER NOT NULL,
    mutation_type TEXT NOT NULL,  -- 'arithmetic', 'comparison', 'boolean', 'return', 'statement', 'branch'

    -- Status
    status TEXT NOT NULL,  -- 'killed', 'survived', 'timeout', 'error'

    -- Details
    description TEXT,
    original_code TEXT,
    mutated_code TEXT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT valid_mutant_status CHECK (status IN ('killed', 'survived', 'timeout', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_mutants_run ON mutants(mutation_run_id);
CREATE INDEX IF NOT EXISTS idx_mutants_status ON mutants(status);
CREATE INDEX IF NOT EXISTS idx_mutants_type ON mutants(mutation_type);
