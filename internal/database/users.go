package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/iaas-platform/internal/models"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	query := `
		INSERT INTO users (email, password_hash, name, role, api_key, organization, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	err := r.pool.QueryRow(ctx, query,
		u.Email, u.PasswordHash, u.Name, u.Role, u.APIKey, u.Organization,
		u.CreatedAt, u.UpdatedAt,
	).Scan(&u.ID)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, role, api_key, organization, created_at, updated_at
		FROM users WHERE email = $1`

	u := &models.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.APIKey, &u.Organization, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, role, api_key, organization, created_at, updated_at
		FROM users WHERE id = $1`

	u := &models.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.APIKey, &u.Organization, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, role, api_key, organization, created_at, updated_at
		FROM users WHERE api_key = $1`

	u := &models.User{}
	err := r.pool.QueryRow(ctx, query, apiKey).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.APIKey, &u.Organization, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find user by api key: %w", err)
	}
	return u, nil
}
