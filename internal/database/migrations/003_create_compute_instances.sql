CREATE TABLE IF NOT EXISTS compute_instances (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    instance_type   TEXT NOT NULL DEFAULT 'vm',
    status          TEXT NOT NULL DEFAULT 'stopped',
    region          TEXT NOT NULL DEFAULT 'us-east',
    cpu_cores       INT NOT NULL DEFAULT 1,
    memory_mb       INT NOT NULL DEFAULT 1024,
    disk_gb         INT NOT NULL DEFAULT 10,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_instances_org ON compute_instances(organization_id);
CREATE INDEX idx_instances_user ON compute_instances(user_id);
CREATE INDEX idx_instances_status ON compute_instances(status);
