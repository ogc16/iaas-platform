CREATE TABLE IF NOT EXISTS invoices (
    id              BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    amount_cents    BIGINT NOT NULL DEFAULT 0,
    currency        TEXT NOT NULL DEFAULT 'usd',
    status          TEXT NOT NULL DEFAULT 'pending',
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS invoice_line_items (
    id              BIGSERIAL PRIMARY KEY,
    invoice_id      BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description     TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    quantity        DOUBLE PRECISION NOT NULL DEFAULT 0,
    unit_price_cents BIGINT NOT NULL DEFAULT 0,
    amount_cents     BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_invoices_org ON invoices(organization_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_line_items_invoice ON invoice_line_items(invoice_id);
