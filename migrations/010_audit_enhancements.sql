-- Migration 010: Audit Logging Enhancements
-- Adds severity levels, additional indexes, and new audit action types

-- Add severity column to audit_logs
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS severity VARCHAR(20) DEFAULT 'info';

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_severity ON audit_logs(severity) WHERE severity IN ('warning', 'critical');
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_action ON audit_logs(organization_id, action);

-- Add comments documenting audit action types
COMMENT ON COLUMN audit_logs.action IS 'Audit action type. Standard actions:
  - auth.login, auth.logout, auth.failure, auth.lockout
  - user.create, user.update, user.delete
  - org.create, org.update, org.delete, org.member.add, org.member.remove
  - repo.create, repo.delete
  - job.create, job.complete, job.fail, job.cancel
  - apikey.create, apikey.revoke, apikey.rotate
  - webhook.create, webhook.update, webhook.delete
  - rate.limited
  - admin.access, admin.config
  - system.circuit_open, system.circuit_close';

COMMENT ON COLUMN audit_logs.severity IS 'Log severity: info, warning, critical';

-- Create partial index for high-severity events for faster querying
CREATE INDEX IF NOT EXISTS idx_audit_logs_high_severity ON audit_logs(created_at DESC)
  WHERE severity IN ('warning', 'critical');
