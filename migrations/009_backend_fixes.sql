-- Migration 009: Backend Fixes and Security Enhancements
-- 1. Update job type constraint to include all job types
-- 2. Add login_attempts table for account lockout
-- 3. Add refresh_tokens table for JWT token rotation
-- 4. Add token_blacklist table for revoked tokens

-- ============================================================
-- 1. FIX JOB TYPE CONSTRAINT
-- ============================================================

-- Drop the existing constraint that's missing validation and e2e types
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_job_type;

-- Add the updated constraint with all supported job types
ALTER TABLE jobs ADD CONSTRAINT valid_job_type CHECK (
    type IN (
        'ingestion',
        'modeling',
        'planning',
        'generation',
        'validation',
        'mutation',
        'integration',
        'e2e_discovery',
        'e2e_generation',
        'e2e_run'
    )
);

COMMENT ON CONSTRAINT valid_job_type ON jobs IS
    'Ensures job type is one of the supported pipeline stages. Updated 2025-11-24 to add validation and E2E types.';

-- ============================================================
-- 2. ACCOUNT LOCKOUT - Login Attempts Table
-- ============================================================

CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identifier being tracked (IP address, user ID, GitHub ID)
    identifier TEXT NOT NULL,
    identifier_type TEXT NOT NULL,  -- 'ip', 'user_id', 'github_id'

    -- Attempt details
    success BOOLEAN NOT NULL,
    ip_address TEXT,
    user_agent TEXT,

    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraint on identifier type
    CONSTRAINT valid_identifier_type CHECK (identifier_type IN ('ip', 'user_id', 'github_id'))
);

-- Indexes for efficient lockout checking
CREATE INDEX IF NOT EXISTS idx_login_attempts_identifier ON login_attempts(identifier, identifier_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_cleanup ON login_attempts(created_at);

-- ============================================================
-- 3. JWT REFRESH TOKENS
-- ============================================================

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Token ownership
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    -- Token data (only store hash, never the actual token)
    token_hash TEXT NOT NULL UNIQUE,
    family_id UUID NOT NULL,  -- For rotation chain tracking (detect reuse)

    -- Lifecycle
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    replaced_by UUID REFERENCES refresh_tokens(id),

    -- Audit info
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient token lookup
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_cleanup ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- ============================================================
-- 4. TOKEN BLACKLIST (for revoked access tokens)
-- ============================================================

CREATE TABLE IF NOT EXISTS token_blacklist (
    token_jti TEXT PRIMARY KEY,  -- JWT ID claim
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,  -- Auto-cleanup after expiry
    revoked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    reason TEXT  -- Optional: 'logout', 'password_change', 'security'
);

-- Index for cleanup job
CREATE INDEX IF NOT EXISTS idx_token_blacklist_expires ON token_blacklist(expires_at);

-- ============================================================
-- 5. API KEY ENHANCEMENTS
-- ============================================================

-- Add rotation tracking to api_keys table if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'api_keys' AND column_name = 'rotated_from') THEN
        ALTER TABLE api_keys ADD COLUMN rotated_from UUID REFERENCES api_keys(id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'api_keys' AND column_name = 'grace_period_ends') THEN
        ALTER TABLE api_keys ADD COLUMN grace_period_ends TIMESTAMP WITH TIME ZONE;
    END IF;
END $$;

-- ============================================================
-- 6. CLEANUP FUNCTIONS
-- ============================================================

-- Function to clean up expired login attempts (call periodically)
CREATE OR REPLACE FUNCTION cleanup_login_attempts(retention_hours INTEGER DEFAULT 24)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM login_attempts
    WHERE created_at < NOW() - (retention_hours || ' hours')::INTERVAL;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up expired blacklist entries
CREATE OR REPLACE FUNCTION cleanup_token_blacklist()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM token_blacklist WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up expired refresh tokens
CREATE OR REPLACE FUNCTION cleanup_refresh_tokens()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM refresh_tokens
    WHERE expires_at < NOW() - INTERVAL '7 days';  -- Keep for 7 days after expiry for audit
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;
