package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogc16/iaas-platform/internal/models"
)

type ComputeRepository struct {
	pool *pgxpool.Pool
}

func NewComputeRepository(pool *pgxpool.Pool) *ComputeRepository {
	return &ComputeRepository{pool: pool}
}

func (r *ComputeRepository) Create(ctx context.Context, inst *models.ComputeInstance) error {
	query := `INSERT INTO compute_instances
		(organization_id, user_id, name, instance_type, status, region, provider_id, image, port, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`
	now := time.Now().UTC()
	inst.CreatedAt = now
	inst.UpdatedAt = now
	return r.pool.QueryRow(ctx, query,
		inst.OrganizationID, inst.UserID, inst.Name, inst.InstanceType, inst.Status,
		inst.Region, inst.ProviderID, inst.Image, inst.Port,
		inst.CPUCores, inst.MemoryMB, inst.DiskGB, inst.IPAddress,
		inst.CreatedAt, inst.UpdatedAt,
	).Scan(&inst.ID)
}

func (r *ComputeRepository) FindByID(ctx context.Context, id int64) (*models.ComputeInstance, error) {
	query := `SELECT id, organization_id, user_id, name, instance_type, status, region, provider_id, image, port, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at
		FROM compute_instances WHERE id = $1`
	inst := &models.ComputeInstance{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&inst.ID, &inst.OrganizationID, &inst.UserID, &inst.Name, &inst.InstanceType,
		&inst.Status, &inst.Region, &inst.ProviderID, &inst.Image, &inst.Port,
		&inst.CPUCores, &inst.MemoryMB, &inst.DiskGB, &inst.IPAddress,
		&inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find instance: %w", err)
	}
	return inst, nil
}

func (r *ComputeRepository) ListByOrg(ctx context.Context, orgID int64, limit, offset int) ([]models.ComputeInstance, error) {
	query := `SELECT id, organization_id, user_id, name, instance_type, status, region, provider_id, image, port, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at
		FROM compute_instances WHERE organization_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`
	return r.list(ctx, query, orgID, limit, offset)
}

func (r *ComputeRepository) CountByOrg(ctx context.Context, orgID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM compute_instances WHERE organization_id = $1`
	var n int64
	if err := r.pool.QueryRow(ctx, query, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count instances: %w", err)
	}
	return n, nil
}

func (r *ComputeRepository) ListActive(ctx context.Context) ([]models.ComputeInstance, error) {
	query := `SELECT id, organization_id, user_id, name, instance_type, status, region, provider_id, image, port, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at
		FROM compute_instances WHERE status <> $1 ORDER BY id`
	return r.list(ctx, query, models.InstanceStatusTerminated)
}

func (r *ComputeRepository) list(ctx context.Context, query string, args ...interface{}) ([]models.ComputeInstance, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []models.ComputeInstance
	for rows.Next() {
		var inst models.ComputeInstance
		if err := rows.Scan(
			&inst.ID, &inst.OrganizationID, &inst.UserID, &inst.Name, &inst.InstanceType,
			&inst.Status, &inst.Region, &inst.ProviderID, &inst.Image, &inst.Port,
			&inst.CPUCores, &inst.MemoryMB, &inst.DiskGB, &inst.IPAddress,
			&inst.CreatedAt, &inst.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (r *ComputeRepository) SumActiveByOrg(ctx context.Context, orgID int64) (models.OrgUsage, error) {
	query := `SELECT COUNT(*),
		COALESCE(SUM(cpu_cores), 0),
		COALESCE(SUM(memory_mb), 0),
		COALESCE(SUM(disk_gb), 0)
		FROM compute_instances WHERE organization_id = $1 AND status <> $2`
	var usage models.OrgUsage
	if err := r.pool.QueryRow(ctx, query, orgID, models.InstanceStatusTerminated).
		Scan(&usage.Count, &usage.CPUCores, &usage.MemoryMB, &usage.DiskGB); err != nil {
		return usage, fmt.Errorf("sum org usage: %w", err)
	}
	return usage, nil
}

func (r *ComputeRepository) SumActiveByRegion(ctx context.Context, region string) (models.RegionUsage, error) {
	query := `SELECT
		COALESCE(SUM(cpu_cores), 0),
		COALESCE(SUM(memory_mb), 0),
		COALESCE(SUM(disk_gb), 0)
		FROM compute_instances WHERE region = $1 AND status <> $2`
	var usage models.RegionUsage
	if err := r.pool.QueryRow(ctx, query, region, models.InstanceStatusTerminated).
		Scan(&usage.CPUCores, &usage.MemoryMB, &usage.DiskGB); err != nil {
		return usage, fmt.Errorf("sum region usage: %w", err)
	}
	return usage, nil
}

func (r *ComputeRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE compute_instances SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

func (r *ComputeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM compute_instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}
