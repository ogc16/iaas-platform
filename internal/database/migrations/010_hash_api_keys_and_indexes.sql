-- Hash existing plaintext API keys so a database dump never leaks credentials.
-- Keys are stored as the hex SHA-256 digest of "iaas_<...>"; the raw key is
-- returned to the client exactly once, at signup. The regex guard keeps this
-- migration idempotent: it only rewrites values that are not already digests.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

UPDATE users
SET api_key = encode(digest(api_key, 'sha256'), 'hex')
WHERE api_key <> '' AND api_key !~ '^[a-f0-9]{64}$';

-- Composite indexes for the most common list and aggregation queries:
--   usage_records:  GetSummary (WHERE organization_id = $1 AND recorded_at >= $2)
--   invoices:       ListByOrg (WHERE organization_id = $1 ORDER BY created_at DESC)
--   instances:      ListByOrg (WHERE organization_id = $1 ORDER BY created_at DESC)
CREATE INDEX IF NOT EXISTS idx_usage_org_recorded ON usage_records (organization_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_invoices_org_created ON invoices (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_instances_org_created ON compute_instances (organization_id, created_at DESC);
