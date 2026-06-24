package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/iaas-platform/internal/models"
)

type ComputeRepository struct {
	pool *pgxpool.Pool
}

func NewComputeRepository(pool *pgxpool.Pool) *ComputeRepository {
	return &ComputeRepository{pool: pool}
}

func (r *ComputeRepository) Create(ctx context.Context, inst *models.ComputeInstance) error {
	query := `INSERT INTO compute_instances
		(organization_id, user_id, name, instance_type, status, region, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`
	now := time.Now().UTC()
	inst.CreatedAt = now
	inst.UpdatedAt = now
	return r.pool.QueryRow(ctx, query,
		inst.OrganizationID, inst.UserID, inst.Name, inst.InstanceType, inst.Status,
		inst.Region, inst.CPUCores, inst.MemoryMB, inst.DiskGB, inst.IPAddress,
		inst.CreatedAt, inst.UpdatedAt,
	).Scan(&inst.ID)
}

func (r *ComputeRepository) FindByID(ctx context.Context, id int64) (*models.ComputeInstance, error) {
	query := `SELECT id, organization_id, user_id, name, instance_type, status, region, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at
		FROM compute_instances WHERE id = $1`
	inst := &models.ComputeInstance{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&inst.ID, &inst.OrganizationID, &inst.UserID, &inst.Name, &inst.InstanceType,
		&inst.Status, &inst.Region, &inst.CPUCores, &inst.MemoryMB, &inst.DiskGB,
		&inst.IPAddress, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find instance: %w", err)
	}
	return inst, nil
}

func (r *ComputeRepository) ListByOrg(ctx context.Context, orgID int64) ([]models.ComputeInstance, error) {
	query := `SELECT id, organization_id, user_id, name, instance_type, status, region, cpu_cores, memory_mb, disk_gb, ip_address, created_at, updated_at
		FROM compute_instances WHERE organization_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []models.ComputeInstance
	for rows.Next() {
		var inst models.ComputeInstance
		if err := rows.Scan(
			&inst.ID, &inst.OrganizationID, &inst.UserID, &inst.Name, &inst.InstanceType,
			&inst.Status, &inst.Region, &inst.CPUCores, &inst.MemoryMB, &inst.DiskGB,
			&inst.IPAddress, &inst.CreatedAt, &inst.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, nil
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
