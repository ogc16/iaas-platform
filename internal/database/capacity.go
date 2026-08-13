package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogc16/iaas-platform/internal/models"
)

// CapacityRepository returns the total allocatable capacity of each region,
// seeded by migration 006.
type CapacityRepository struct {
	pool *pgxpool.Pool
}

func NewCapacityRepository(pool *pgxpool.Pool) *CapacityRepository {
	return &CapacityRepository{pool: pool}
}

func (r *CapacityRepository) GetRegion(ctx context.Context, region string) (models.RegionCapacity, error) {
	query := `SELECT region, cpu_cores, memory_mb, disk_gb FROM region_capacity WHERE region = $1`
	var c models.RegionCapacity
	err := r.pool.QueryRow(ctx, query, region).
		Scan(&c.Region, &c.CPUCores, &c.MemoryMB, &c.DiskGB)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("get region capacity: %w", err)
	}
	return c, nil
}
