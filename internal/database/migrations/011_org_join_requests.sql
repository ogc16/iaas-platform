CREATE TABLE IF NOT EXISTS org_join_requests (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, user_id)
);

CREATE INDEX idx_org_join_requests_org ON org_join_requests(organization_id);
CREATE INDEX idx_org_join_requests_user ON org_join_requests(user_id);
