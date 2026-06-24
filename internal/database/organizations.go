package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/iaas-platform/internal/models"
)

type OrgRepository struct {
	pool *pgxpool.Pool
}

func NewOrgRepository(pool *pgxpool.Pool) *OrgRepository {
	return &OrgRepository{pool: pool}
}

func (r *OrgRepository) Create(ctx context.Context, org *models.Organization) error {
	query := `INSERT INTO organizations (name, slug, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`
	now := time.Now().UTC()
	org.CreatedAt = now
	org.UpdatedAt = now
	return r.pool.QueryRow(ctx, query, org.Name, org.Slug, org.CreatedAt, org.UpdatedAt).Scan(&org.ID)
}

func (r *OrgRepository) FindByID(ctx context.Context, id int64) (*models.Organization, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM organizations WHERE id = $1`
	org := &models.Organization{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find org: %w", err)
	}
	return org, nil
}

func (r *OrgRepository) FindBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM organizations WHERE slug = $1`
	org := &models.Organization{}
	err := r.pool.QueryRow(ctx, query, slug).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find org by slug: %w", err)
	}
	return org, nil
}

func (r *OrgRepository) AddMember(ctx context.Context, member *models.OrgMember) error {
	query := `INSERT INTO organization_members (organization_id, user_id, role, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	member.CreatedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, member.OrganizationID, member.UserID, member.Role, member.CreatedAt).Scan(&member.ID)
}

func (r *OrgRepository) FindMembers(ctx context.Context, orgID int64) ([]models.OrgMember, error) {
	query := `SELECT id, organization_id, user_id, role, created_at FROM organization_members WHERE organization_id = $1`
	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("find members: %w", err)
	}
	defer rows.Close()

	var members []models.OrgMember
	for rows.Next() {
		var m models.OrgMember
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *OrgRepository) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	query := `SELECT id, organization_id, user_id, role, created_at FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	m := &models.OrgMember{}
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find member: %w", err)
	}
	return m, nil
}

func (r *OrgRepository) ListByUser(ctx context.Context, userID int64) ([]models.Organization, error) {
	query := `SELECT o.id, o.name, o.slug, o.created_at, o.updated_at
		FROM organizations o
		JOIN organization_members om ON om.organization_id = o.id
		WHERE om.user_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list orgs by user: %w", err)
	}
	defer rows.Close()

	var orgs []models.Organization
	for rows.Next() {
		var o models.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, nil
}
