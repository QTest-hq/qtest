-- 007_usage_tracking.sql
-- Usage tracking and analytics

-- API usage tracking table
CREATE TABLE IF NOT EXISTS api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,

    -- Request details
    endpoint TEXT NOT NULL,
    method TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    request_size_bytes INTEGER DEFAULT 0,
    response_size_bytes INTEGER DEFAULT 0,

    -- Client info
    ip_address TEXT,
    user_agent TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Daily aggregated usage stats
CREATE TABLE IF NOT EXISTS usage_stats_daily (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    date DATE NOT NULL,

    -- Request counts
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    failed_requests INTEGER DEFAULT 0,

    -- Job counts
    jobs_created INTEGER DEFAULT 0,
    jobs_completed INTEGER DEFAULT 0,
    jobs_failed INTEGER DEFAULT 0,

    -- Test generation
    tests_generated INTEGER DEFAULT 0,
    tests_validated INTEGER DEFAULT 0,
    tests_accepted INTEGER DEFAULT 0,

    -- Resource usage
    total_response_time_ms BIGINT DEFAULT 0,
    total_request_bytes BIGINT DEFAULT 0,
    total_response_bytes BIGINT DEFAULT 0,

    -- Unique users/keys
    unique_users INTEGER DEFAULT 0,
    unique_api_keys INTEGER DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(organization_id, date)
);

-- Monthly aggregated usage stats
CREATE TABLE IF NOT EXISTS usage_stats_monthly (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,

    -- Request counts
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    failed_requests INTEGER DEFAULT 0,

    -- Job counts
    jobs_created INTEGER DEFAULT 0,
    jobs_completed INTEGER DEFAULT 0,
    jobs_failed INTEGER DEFAULT 0,

    -- Test generation
    tests_generated INTEGER DEFAULT 0,
    tests_validated INTEGER DEFAULT 0,
    tests_accepted INTEGER DEFAULT 0,

    -- Average response time
    avg_response_time_ms INTEGER DEFAULT 0,

    -- Peak usage
    peak_daily_requests INTEGER DEFAULT 0,
    peak_concurrent_jobs INTEGER DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(organization_id, year, month)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_api_usage_org_created ON api_usage(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_usage_endpoint ON api_usage(endpoint, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_usage_user ON api_usage(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_usage_api_key ON api_usage(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_stats_daily_org_date ON usage_stats_daily(organization_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_usage_stats_monthly_org ON usage_stats_monthly(organization_id, year DESC, month DESC);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_usage_stats_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_usage_stats_daily_updated_at
    BEFORE UPDATE ON usage_stats_daily
    FOR EACH ROW
    EXECUTE FUNCTION update_usage_stats_updated_at();

CREATE TRIGGER update_usage_stats_monthly_updated_at
    BEFORE UPDATE ON usage_stats_monthly
    FOR EACH ROW
    EXECUTE FUNCTION update_usage_stats_updated_at();
