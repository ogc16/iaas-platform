package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/iaas-platform/internal/models"
)

type UsageRepository struct {
	pool *pgxpool.Pool
}

func NewUsageRepository(pool *pgxpool.Pool) *UsageRepository {
	return &UsageRepository{pool: pool}
}

func (r *UsageRepository) Record(ctx context.Context, u *models.UsageRecord) error {
	query := `INSERT INTO usage_records (organization_id, instance_id, resource_type, quantity, recorded_at) VALUES ($1,$2,$3,$4,$5) RETURNING id`
	u.RecordedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, u.OrganizationID, u.InstanceID, u.ResourceType, u.Quantity, u.RecordedAt).Scan(&u.ID)
}

func (r *UsageRepository) GetSummary(ctx context.Context, orgID int64, since time.Time) (*models.UsageSummary, error) {
	query := `SELECT resource_type, COALESCE(SUM(quantity), 0) FROM usage_records WHERE organization_id = $1 AND recorded_at >= $2 GROUP BY resource_type`
	rows, err := r.pool.Query(ctx, query, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("get usage summary: %w", err)
	}
	defer rows.Close()

	s := &models.UsageSummary{}
	for rows.Next() {
		var rt string
		var qty float64
		if err := rows.Scan(&rt, &qty); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		switch rt {
		case models.ResourceTypeCPUHours:
			s.CPUHours = qty
		case models.ResourceTypeMemoryGBH:
			s.MemoryGBHours = qty
		case models.ResourceTypeDiskGBH:
			s.DiskGBHours = qty
		}
	}
	return s, nil
}

type InvoiceRepository struct {
	pool *pgxpool.Pool
}

func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *models.Invoice) error {
	query := `INSERT INTO invoices (organization_id, amount_cents, currency, status, period_start, period_end, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`
	inv.CreatedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, inv.OrganizationID, inv.AmountCents, inv.Currency, inv.Status, inv.PeriodStart, inv.PeriodEnd, inv.CreatedAt).Scan(&inv.ID)
}

func (r *InvoiceRepository) AddLineItem(ctx context.Context, item *models.InvoiceLineItem) error {
	query := `INSERT INTO invoice_line_items (invoice_id, description, resource_type, quantity, unit_price_cents, amount_cents) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`
	return r.pool.QueryRow(ctx, query, item.InvoiceID, item.Description, item.ResourceType, item.Quantity, item.UnitPriceCents, item.AmountCents).Scan(&item.ID)
}

func (r *InvoiceRepository) ListByOrg(ctx context.Context, orgID int64) ([]models.Invoice, error) {
	query := `SELECT id, organization_id, amount_cents, currency, status, period_start, period_end, created_at FROM invoices WHERE organization_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(&inv.ID, &inv.OrganizationID, &inv.AmountCents, &inv.Currency, &inv.Status, &inv.PeriodStart, &inv.PeriodEnd, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetLineItems(ctx context.Context, invoiceID int64) ([]models.InvoiceLineItem, error) {
	query := `SELECT id, invoice_id, description, resource_type, quantity, unit_price_cents, amount_cents FROM invoice_line_items WHERE invoice_id = $1`
	rows, err := r.pool.Query(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list line items: %w", err)
	}
	defer rows.Close()

	var items []models.InvoiceLineItem
	for rows.Next() {
		var li models.InvoiceLineItem
		if err := rows.Scan(&li.ID, &li.InvoiceID, &li.Description, &li.ResourceType, &li.Quantity, &li.UnitPriceCents, &li.AmountCents); err != nil {
			return nil, fmt.Errorf("scan line item: %w", err)
		}
		items = append(items, li)
	}
	return items, nil
}
