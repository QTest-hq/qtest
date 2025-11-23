-- Migration 005: Multi-tenancy support
-- Adds users, organizations, and tenant isolation

-- =============================================
-- USERS TABLE
-- =============================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id BIGINT UNIQUE NOT NULL,
    github_login TEXT NOT NULL,
    email TEXT,
    name TEXT,
    avatar_url TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id);
CREATE INDEX IF NOT EXISTS idx_users_github_login ON users(github_login);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- =============================================
-- ORGANIZATIONS TABLE
-- =============================================
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL REFERENCES users(id),
    github_org_id BIGINT,
    settings JSONB DEFAULT '{}'::jsonb,
    is_personal BOOLEAN DEFAULT false,  -- True for auto-created personal orgs
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);
CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations(owner_id);
CREATE INDEX IF NOT EXISTS idx_organizations_github_org ON organizations(github_org_id);

-- =============================================
-- ORGANIZATION MEMBERS TABLE
-- =============================================
CREATE TYPE member_role AS ENUM ('owner', 'admin', 'member', 'viewer');

CREATE TABLE IF NOT EXISTS organization_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role member_role NOT NULL DEFAULT 'member',
    invited_by UUID REFERENCES users(id),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(organization_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_members_org ON organization_members(organization_id);
CREATE INDEX IF NOT EXISTS idx_org_members_user ON organization_members(user_id);

-- =============================================
-- SESSIONS TABLE (database-backed sessions)
-- =============================================
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_access TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- =============================================
-- ADD ORGANIZATION_ID TO EXISTING TABLES
-- =============================================

-- Add organization_id to repositories (nullable initially for migration)
ALTER TABLE repositories
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id),
ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_repositories_org ON repositories(organization_id);

-- Add organization_id to generation_runs
ALTER TABLE generation_runs
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

CREATE INDEX IF NOT EXISTS idx_generation_runs_org ON generation_runs(organization_id);

-- Add organization_id to generated_tests
ALTER TABLE generated_tests
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

CREATE INDEX IF NOT EXISTS idx_generated_tests_org ON generated_tests(organization_id);

-- Add organization_id to jobs
ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

CREATE INDEX IF NOT EXISTS idx_jobs_org ON jobs(organization_id);

-- Add organization_id to mutation_runs
ALTER TABLE mutation_runs
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

CREATE INDEX IF NOT EXISTS idx_mutation_runs_org ON mutation_runs(organization_id);

-- Add organization_id to quality_thresholds (per-org configuration)
ALTER TABLE quality_thresholds
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

CREATE INDEX IF NOT EXISTS idx_quality_thresholds_org ON quality_thresholds(organization_id);

-- =============================================
-- AUDIT LOG TABLE
-- =============================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID,
    details JSONB DEFAULT '{}'::jsonb,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_org ON audit_logs(organization_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);

-- =============================================
-- API KEYS TABLE (for programmatic access)
-- =============================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,  -- First 8 chars for identification
    key_hash TEXT NOT NULL,     -- SHA256 hash of full key
    scopes TEXT[] DEFAULT ARRAY['read', 'write'],
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys(organization_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);

-- =============================================
-- TRIGGERS FOR UPDATED_AT
-- =============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;
CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_organizations_updated_at ON organizations;
CREATE TRIGGER trigger_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- HELPER FUNCTION: Create personal org for user
-- =============================================
CREATE OR REPLACE FUNCTION create_personal_organization()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO organizations (name, slug, owner_id, is_personal)
    VALUES (
        NEW.github_login || '''s Workspace',
        NEW.github_login,
        NEW.id,
        true
    );

    -- Add user as owner of their personal org
    INSERT INTO organization_members (organization_id, user_id, role)
    SELECT id, NEW.id, 'owner'
    FROM organizations
    WHERE owner_id = NEW.id AND is_personal = true;

    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS trigger_create_personal_org ON users;
CREATE TRIGGER trigger_create_personal_org
    AFTER INSERT ON users
    FOR EACH ROW
    EXECUTE FUNCTION create_personal_organization();

-- =============================================
-- COMMENTS
-- =============================================
COMMENT ON TABLE users IS 'User accounts linked to GitHub OAuth';
COMMENT ON TABLE organizations IS 'Tenant organizations for multi-tenancy';
COMMENT ON TABLE organization_members IS 'User membership and roles in organizations';
COMMENT ON TABLE sessions IS 'Database-backed user sessions';
COMMENT ON TABLE audit_logs IS 'Audit trail for security and compliance';
COMMENT ON TABLE api_keys IS 'API keys for programmatic access';

COMMENT ON COLUMN organizations.is_personal IS 'Auto-created personal workspace for each user';
COMMENT ON COLUMN organization_members.role IS 'owner: full access, admin: manage members, member: read/write, viewer: read-only';
