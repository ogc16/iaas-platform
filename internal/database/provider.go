package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderStateRepository persists the simulation provider's desired state.
// It satisfies the compute.SimStateStore interface structurally, which keeps
// the database package free of a dependency on the compute package.
type ProviderStateRepository struct {
	pool *pgxpool.Pool
}

func NewProviderStateRepository(pool *pgxpool.Pool) *ProviderStateRepository {
	return &ProviderStateRepository{pool: pool}
}

func (r *ProviderStateRepository) Get(ctx context.Context, providerID string) (desiredState string, transitionAt time.Time, err error) {
	query := `SELECT desired_state, transition_at FROM provider_state WHERE provider_id = $1`
	err = r.pool.QueryRow(ctx, query, providerID).Scan(&desiredState, &transitionAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, ErrNotFound
		}
		return "", time.Time{}, fmt.Errorf("get provider state: %w", err)
	}
	return desiredState, transitionAt, nil
}

func (r *ProviderStateRepository) Set(ctx context.Context, providerID, desiredState string, transitionAt time.Time) error {
	query := `INSERT INTO provider_state (provider_id, desired_state, transition_at, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (provider_id) DO UPDATE SET
			desired_state = EXCLUDED.desired_state,
			transition_at = EXCLUDED.transition_at,
			updated_at    = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, query, providerID, desiredState, transitionAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set provider state: %w", err)
	}
	return nil
}
