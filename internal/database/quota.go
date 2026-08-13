package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogc16/iaas-platform/internal/models"
)

// QuotaRepository returns per-organization quotas, falling back to the
// platform defaults when an organization has no explicit quota row.
type QuotaRepository struct {
	pool *pgxpool.Pool
}

func NewQuotaRepository(pool *pgxpool.Pool) *QuotaRepository {
	return &QuotaRepository{pool: pool}
}

func (r *QuotaRepository) Get(ctx context.Context, orgID int64) (models.Quota, error) {
	query := `SELECT organization_id, max_instances, max_cpu_cores, max_memory_mb, max_disk_gb
		FROM organization_quotas WHERE organization_id = $1`
	var q models.Quota
	err := r.pool.QueryRow(ctx, query, orgID).
		Scan(&q.OrganizationID, &q.MaxInstances, &q.MaxCPUCores, &q.MaxMemoryMB, &q.MaxDiskGB)
	if errors.Is(err, pgx.ErrNoRows) {
		q = models.DefaultQuota
		q.OrganizationID = orgID
		return q, nil
	}
	if err != nil {
		return q, fmt.Errorf("get quota: %w", err)
	}
	return q, nil
}

func (r *QuotaRepository) Upsert(ctx context.Context, q models.Quota) error {
	query := `INSERT INTO organization_quotas
		(organization_id, max_instances, max_cpu_cores, max_memory_mb, max_disk_gb)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (organization_id) DO UPDATE SET
			max_instances = EXCLUDED.max_instances,
			max_cpu_cores = EXCLUDED.max_cpu_cores,
			max_memory_mb = EXCLUDED.max_memory_mb,
			max_disk_gb   = EXCLUDED.max_disk_gb`
	_, err := r.pool.Exec(ctx, query, q.OrganizationID, q.MaxInstances, q.MaxCPUCores, q.MaxMemoryMB, q.MaxDiskGB)
	if err != nil {
		return fmt.Errorf("upsert quota: %w", err)
	}
	return nil
}
