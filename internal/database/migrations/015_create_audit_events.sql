-- Immutable, append-only record of every security-relevant action taken
-- against the control plane, scoped to an organization so admins can audit
-- who did what and when. Actors that no longer exist keep their email.
CREATE TABLE IF NOT EXISTS audit_events (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_email     TEXT,
    action          TEXT NOT NULL,
    resource_type   TEXT,
    resource_id     TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    ip              TEXT,
    request_id      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_org ON audit_events (organization_id, created_at DESC);
CREATE INDEX idx_audit_events_org_action ON audit_events (organization_id, action, created_at DESC);
