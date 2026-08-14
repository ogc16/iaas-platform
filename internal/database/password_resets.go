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

type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

func (r *PasswordResetRepository) Create(ctx context.Context, reset *models.PasswordReset) error {
	query := `INSERT INTO password_resets (user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	reset.CreatedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, reset.UserID, reset.TokenHash, reset.ExpiresAt, reset.CreatedAt).Scan(&reset.ID)
}

// FindByTokenHash looks up a reset row by the SHA-256 hash of its token. The
// caller checks expiry/consumption; this returns the row regardless.
func (r *PasswordResetRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordReset, error) {
	query := `SELECT id, user_id, token_hash, expires_at, used_at, created_at FROM password_resets WHERE token_hash = $1`
	reset := &models.PasswordReset{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&reset.ID, &reset.UserID, &reset.TokenHash, &reset.ExpiresAt, &reset.UsedAt, &reset.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find password reset: %w", err)
	}
	return reset, nil
}

// DeleteForUser invalidates every outstanding reset token for a user (for
// example when a new reset is requested or a password change succeeds). It is
// not an error when the user has no tokens.
func (r *PasswordResetRepository) DeleteForUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM password_resets WHERE user_id = $1`
	if _, err := r.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("delete password resets: %w", err)
	}
	return nil
}

// MarkUsed consumes a reset token so it cannot be replayed.
func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id int64) error {
	query := `UPDATE password_resets SET used_at = $2 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, time.Now().UTC()); err != nil {
		return fmt.Errorf("mark password reset used: %w", err)
	}
	return nil
}
