CREATE TABLE IF NOT EXISTS organization_quotas (
    organization_id BIGINT PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    max_instances   BIGINT NOT NULL DEFAULT 20,
    max_cpu_cores   BIGINT NOT NULL DEFAULT 16,
    max_memory_mb   BIGINT NOT NULL DEFAULT 32768,
    max_disk_gb     BIGINT NOT NULL DEFAULT 500
);
