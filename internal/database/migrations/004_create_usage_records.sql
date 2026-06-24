CREATE TABLE IF NOT EXISTS usage_records (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    instance_id     BIGINT NOT NULL REFERENCES compute_instances(id) ON DELETE CASCADE,
    resource_type   TEXT NOT NULL,
    quantity        DOUBLE PRECISION NOT NULL DEFAULT 0,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_org ON usage_records(organization_id);
CREATE INDEX idx_usage_instance ON usage_records(instance_id);
CREATE INDEX idx_usage_recorded ON usage_records(recorded_at);
