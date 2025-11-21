-- QTest Jobs Schema
-- Async job processing for worker system

-- Jobs table for tracking async work
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Job identification
    type TEXT NOT NULL,  -- 'ingestion', 'modeling', 'planning', 'generation', 'mutation', 'integration'
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'completed', 'failed', 'retrying', 'cancelled'
    priority INTEGER NOT NULL DEFAULT 0,  -- Higher = more urgent

    -- Context references
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    generation_run_id UUID REFERENCES generation_runs(id) ON DELETE CASCADE,
    parent_job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,  -- For job chaining

    -- Payload and results
    payload JSONB NOT NULL DEFAULT '{}',  -- Input data for job (type-specific)
    result JSONB,  -- Output data from job

    -- Error handling
    error_message TEXT,
    error_details JSONB,  -- Stack trace, context
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,

    -- Timing
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,

    -- Distributed locking
    locked_until TIMESTAMP WITH TIME ZONE,
    worker_id TEXT,  -- Which worker instance claimed this job

    -- Constraints
    CONSTRAINT valid_job_type CHECK (type IN ('ingestion', 'modeling', 'planning', 'generation', 'mutation', 'integration')),
    CONSTRAINT valid_job_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'retrying', 'cancelled'))
);

-- Indexes for efficient job querying
CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(type, status);
CREATE INDEX IF NOT EXISTS idx_jobs_status_priority ON jobs(status, priority DESC, created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_repository ON jobs(repository_id);
CREATE INDEX IF NOT EXISTS idx_jobs_run ON jobs(generation_run_id);
CREATE INDEX IF NOT EXISTS idx_jobs_parent ON jobs(parent_job_id);
CREATE INDEX IF NOT EXISTS idx_jobs_worker ON jobs(worker_id) WHERE worker_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_locked ON jobs(locked_until) WHERE status = 'running';

-- Partial index for pending jobs (most common query)
CREATE INDEX IF NOT EXISTS idx_jobs_pending ON jobs(type, priority DESC, created_at)
    WHERE status = 'pending';

-- Updated timestamp trigger for jobs
CREATE TRIGGER jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Job history table for audit trail (optional, for debugging)
CREATE TABLE IF NOT EXISTS job_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    previous_status TEXT NOT NULL,
    new_status TEXT NOT NULL,
    changed_by TEXT,  -- worker_id or 'api'
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    details JSONB  -- Additional context about the change
);

CREATE INDEX IF NOT EXISTS idx_job_history_job ON job_history(job_id);
CREATE INDEX IF NOT EXISTS idx_job_history_time ON job_history(changed_at);
