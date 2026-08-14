-- Suspended members lose access to the organization until suspended_until
-- passes, at which point their rights are automatically restored. NULL means
-- the member is not suspended.
ALTER TABLE organization_members ADD COLUMN IF NOT EXISTS suspended_until TIMESTAMPTZ;
