CREATE TABLE IF NOT EXISTS region_capacity (
    region    TEXT PRIMARY KEY,
    cpu_cores BIGINT NOT NULL,
    memory_mb BIGINT NOT NULL,
    disk_gb   BIGINT NOT NULL
);

INSERT INTO region_capacity (region, cpu_cores, memory_mb, disk_gb) VALUES
    ('us-east-1', 64, 131072, 2000),
    ('us-west-1', 48, 98304, 1500),
    ('eu-west-1', 32, 65536, 1000)
ON CONFLICT (region) DO NOTHING;
