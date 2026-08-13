CREATE TABLE IF NOT EXISTS provider_state (
    provider_id   TEXT PRIMARY KEY,
    desired_state TEXT NOT NULL,
    transition_at TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
