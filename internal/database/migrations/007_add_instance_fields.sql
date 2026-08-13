ALTER TABLE compute_instances
    ADD COLUMN IF NOT EXISTS provider_id TEXT,
    ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT 'debian-12',
    ADD COLUMN IF NOT EXISTS port INT NOT NULL DEFAULT 0;

ALTER TABLE compute_instances ALTER COLUMN status SET DEFAULT 'pending';

CREATE INDEX IF NOT EXISTS idx_instances_region_status ON compute_instances(region, status);
CREATE INDEX IF NOT EXISTS idx_instances_provider ON compute_instances(provider_id);
