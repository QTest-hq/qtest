-- QTest Initial Schema
-- This file is used by docker-compose for initial setup

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    owner TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    language TEXT,
    last_commit_sha TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- System models table (stores parsed code structure)
CREATE TABLE IF NOT EXISTS system_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    commit_sha TEXT NOT NULL,
    model_data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repository_id, commit_sha)
);

-- Generation runs table
CREATE TABLE IF NOT EXISTS generation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    system_model_id UUID REFERENCES system_models(id),
    status TEXT NOT NULL DEFAULT 'pending',
    config JSONB NOT NULL DEFAULT '{}',
    summary JSONB,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Generated tests table
CREATE TABLE IF NOT EXISTS generated_tests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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
    mutation_score FLOAT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Mutation results table
CREATE TABLE IF NOT EXISTS mutation_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id UUID NOT NULL REFERENCES generated_tests(id) ON DELETE CASCADE,
    mutant_id TEXT NOT NULL,
    operator TEXT NOT NULL,
    location TEXT NOT NULL,
    killed BOOLEAN NOT NULL,
    runtime_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_repositories_url ON repositories(url);
CREATE INDEX IF NOT EXISTS idx_system_models_repo ON system_models(repository_id);
CREATE INDEX IF NOT EXISTS idx_generation_runs_repo ON generation_runs(repository_id);
CREATE INDEX IF NOT EXISTS idx_generation_runs_status ON generation_runs(status);
CREATE INDEX IF NOT EXISTS idx_generated_tests_run ON generated_tests(run_id);
CREATE INDEX IF NOT EXISTS idx_generated_tests_status ON generated_tests(status);
CREATE INDEX IF NOT EXISTS idx_mutation_results_test ON mutation_results(test_id);

-- Updated timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER repositories_updated_at
    BEFORE UPDATE ON repositories
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER generated_tests_updated_at
    BEFORE UPDATE ON generated_tests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
