-- Migration 004: Add test quality metrics
-- Adds quality scoring, assertion counts, coverage data, and regeneration tracking

-- Add quality metrics to generated_tests table
ALTER TABLE generated_tests
ADD COLUMN IF NOT EXISTS quality_score DECIMAL(5, 2),
ADD COLUMN IF NOT EXISTS quality_grade VARCHAR(1),
ADD COLUMN IF NOT EXISTS assertion_count INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS coverage_percent DECIMAL(5, 2),
ADD COLUMN IF NOT EXISTS regen_attempts INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS rejection_reason TEXT,
ADD COLUMN IF NOT EXISTS quality_issues JSONB DEFAULT '[]'::jsonb;

-- Add quality breakdown for detailed analysis
ALTER TABLE generated_tests
ADD COLUMN IF NOT EXISTS quality_breakdown JSONB;

-- Create enum type for rejection reasons if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'rejection_reason_type') THEN
        CREATE TYPE rejection_reason_type AS ENUM (
            'no_assertions',
            'trivial_assertions',
            'low_coverage',
            'low_mutation_score',
            'target_not_called',
            'quality_below_threshold',
            'compile_error',
            'test_failure',
            'max_regen_exceeded',
            'manual_rejection'
        );
    END IF;
END $$;

-- Create test_quality_metrics table for historical tracking
CREATE TABLE IF NOT EXISTS test_quality_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL REFERENCES generated_tests(id) ON DELETE CASCADE,
    run_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Scores (0-100)
    overall_score DECIMAL(5, 2) NOT NULL,
    assertion_score DECIMAL(5, 2),
    coverage_score DECIMAL(5, 2),
    mutation_score DECIMAL(5, 2),
    static_score DECIMAL(5, 2),

    -- Breakdown metrics
    assertion_count INTEGER DEFAULT 0,
    trivial_assertions INTEGER DEFAULT 0,
    tests_with_assertions INTEGER DEFAULT 0,
    total_tests INTEGER DEFAULT 0,
    coverage_percent DECIMAL(5, 2),
    target_func_covered BOOLEAN DEFAULT false,
    mutation_kill_rate DECIMAL(5, 4),
    mutants_killed INTEGER DEFAULT 0,
    mutants_total INTEGER DEFAULT 0,

    -- Issues found
    issues JSONB DEFAULT '[]'::jsonb,

    -- Result
    passed BOOLEAN DEFAULT false,
    grade VARCHAR(1),
    recommendation TEXT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_test_quality_metrics_test_id ON test_quality_metrics(test_id);
CREATE INDEX IF NOT EXISTS idx_test_quality_metrics_score ON test_quality_metrics(overall_score);
CREATE INDEX IF NOT EXISTS idx_test_quality_metrics_passed ON test_quality_metrics(passed);

-- Create quality_thresholds table for configurable standards
CREATE TABLE IF NOT EXISTS quality_thresholds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,

    -- Thresholds
    min_score DECIMAL(5, 2) DEFAULT 60.0,
    min_assertions INTEGER DEFAULT 1,
    min_coverage DECIMAL(5, 2) DEFAULT 50.0,
    min_mutation_kill_rate DECIMAL(5, 4) DEFAULT 0.50,
    max_trivial_assertions DECIMAL(5, 2) DEFAULT 25.0,
    require_target_coverage BOOLEAN DEFAULT true,

    -- Weights
    assertion_weight DECIMAL(3, 2) DEFAULT 0.20,
    coverage_weight DECIMAL(3, 2) DEFAULT 0.20,
    mutation_weight DECIMAL(3, 2) DEFAULT 0.40,
    static_weight DECIMAL(3, 2) DEFAULT 0.20,

    -- Regeneration limits
    max_regen_attempts INTEGER DEFAULT 2,

    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert default quality threshold
INSERT INTO quality_thresholds (name, description, is_default)
VALUES ('default', 'Default quality thresholds for test validation', true)
ON CONFLICT (name) DO NOTHING;

-- Add index for quality-based queries on generated_tests
CREATE INDEX IF NOT EXISTS idx_generated_tests_quality_score ON generated_tests(quality_score);
CREATE INDEX IF NOT EXISTS idx_generated_tests_quality_grade ON generated_tests(quality_grade);
CREATE INDEX IF NOT EXISTS idx_generated_tests_regen_attempts ON generated_tests(regen_attempts);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_quality_thresholds_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS trigger_quality_thresholds_updated_at ON quality_thresholds;
CREATE TRIGGER trigger_quality_thresholds_updated_at
    BEFORE UPDATE ON quality_thresholds
    FOR EACH ROW
    EXECUTE FUNCTION update_quality_thresholds_updated_at();

-- Add status for quality-related rejections
-- Check if status column exists and is varchar, then add new values
DO $$
BEGIN
    -- Add 'quality_rejected' to status if using varchar
    -- This is handled at application level since we're using varchar for status
    NULL;
END $$;

COMMENT ON COLUMN generated_tests.quality_score IS 'Overall quality score 0-100';
COMMENT ON COLUMN generated_tests.quality_grade IS 'Letter grade A-F based on quality score';
COMMENT ON COLUMN generated_tests.assertion_count IS 'Number of assertions in the test';
COMMENT ON COLUMN generated_tests.coverage_percent IS 'Code coverage percentage achieved by this test';
COMMENT ON COLUMN generated_tests.regen_attempts IS 'Number of times this test was regenerated due to quality issues';
COMMENT ON COLUMN generated_tests.rejection_reason IS 'Reason for rejection if test failed quality checks';
COMMENT ON COLUMN generated_tests.quality_issues IS 'JSON array of quality issues found during validation';
COMMENT ON TABLE test_quality_metrics IS 'Historical tracking of test quality assessments';
COMMENT ON TABLE quality_thresholds IS 'Configurable quality standards for different projects/tiers';
