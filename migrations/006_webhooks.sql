-- Migration 006: Webhooks System
-- Allows organizations to register webhook URLs for event notifications

-- =============================================
-- WEBHOOKS TABLE
-- =============================================
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id),

    -- Webhook configuration
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,  -- For HMAC signature verification

    -- Event subscriptions (array of event types)
    events TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],

    -- Status and configuration
    is_active BOOLEAN DEFAULT true,

    -- Rate limiting / backoff settings
    max_retries INTEGER NOT NULL DEFAULT 5,
    timeout_seconds INTEGER NOT NULL DEFAULT 30,

    -- Metadata
    description TEXT,
    headers JSONB DEFAULT '{}'::jsonb,  -- Custom headers to send with webhook

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_triggered_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_webhooks_org ON webhooks(organization_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(organization_id, is_active) WHERE is_active = true;

-- =============================================
-- WEBHOOK DELIVERIES TABLE (Delivery History)
-- =============================================
CREATE TYPE webhook_delivery_status AS ENUM ('pending', 'success', 'failed', 'retrying');

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,

    -- Event information
    event_type TEXT NOT NULL,
    event_id UUID NOT NULL,  -- ID of the resource that triggered the event

    -- Delivery details
    payload JSONB NOT NULL,

    -- Request/response
    request_headers JSONB,
    response_status INTEGER,
    response_body TEXT,
    response_headers JSONB,

    -- Status tracking
    status webhook_delivery_status NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMP WITH TIME ZONE,

    -- Error tracking
    error_message TEXT,

    -- Timing
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status, next_retry_at)
    WHERE status IN ('pending', 'retrying');
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event ON webhook_deliveries(event_type, event_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created ON webhook_deliveries(created_at);

-- =============================================
-- TRIGGERS
-- =============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS trigger_webhooks_updated_at ON webhooks;
CREATE TRIGGER trigger_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- COMMENTS
-- =============================================
COMMENT ON TABLE webhooks IS 'Webhook endpoints registered by organizations for event notifications';
COMMENT ON TABLE webhook_deliveries IS 'History of webhook delivery attempts';
COMMENT ON COLUMN webhooks.secret IS 'Shared secret for HMAC-SHA256 signature verification';
COMMENT ON COLUMN webhooks.events IS 'Array of subscribed event types (e.g., job.completed, run.completed)';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'When to retry this delivery (exponential backoff)';
