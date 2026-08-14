-- Outbound webhook endpoints. Each organization can register endpoints that
-- receive realtime notifications of the events they subscribe to. Deliveries
-- are HMAC-SHA256 signed with the endpoint's secret so receivers can verify
-- the payload really came from the platform.
CREATE TABLE IF NOT EXISTS webhooks (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    secret          TEXT NOT NULL,
    events          TEXT[] NOT NULL DEFAULT '{}',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_org ON webhooks (organization_id);

-- A queued (or already attempted) delivery of one event to one webhook. The
-- dispatcher worker picks rows that are due (pending/failed with a
-- next_attempt_at in the past) and POSTs the signed payload, retrying with
-- exponential backoff until max_attempts is exhausted.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              BIGSERIAL PRIMARY KEY,
    webhook_id      BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 5,
    last_error      TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE status IN ('pending', 'failed');
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id);
