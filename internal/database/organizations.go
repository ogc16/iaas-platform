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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find org: %w", err)
	}
	return org, nil
}

func (r *OrgRepository) FindBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM organizations WHERE slug = $1`
	org := &models.Organization{}
	err := r.pool.QueryRow(ctx, query, slug).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find org by slug: %w", err)
	}
	return org, nil
}

func (r *OrgRepository) AddMember(ctx context.Context, member *models.OrgMember) error {
	query := `INSERT INTO organization_members (organization_id, user_id, role, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	member.CreatedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, member.OrganizationID, member.UserID, member.Role, member.CreatedAt).Scan(&member.ID)
}

// RemoveMember revokes a member's access to an organization. It returns
// ErrNotFound when the user is not a member.
func (r *OrgRepository) RemoveMember(ctx context.Context, orgID, userID int64) error {
	query := `DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SuspendMember revokes a member's access until the given time. The member's
// rights are automatically restored once the time passes. It returns
// ErrNotFound when the user is not a member.
func (r *OrgRepository) SuspendMember(ctx context.Context, orgID, userID int64, until time.Time) error {
	query := `UPDATE organization_members SET suspended_until = $3 WHERE organization_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, orgID, userID, until)
	if err != nil {
		return fmt.Errorf("suspend member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UnsuspendMember immediately restores a suspended member's access. It
// returns ErrNotFound when the user is not a member.
func (r *OrgRepository) UnsuspendMember(ctx context.Context, orgID, userID int64) error {
	query := `UPDATE organization_members SET suspended_until = NULL WHERE organization_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("unsuspend member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *OrgRepository) AddJoinRequest(ctx context.Context, request *models.JoinRequest) error {
	query := `INSERT INTO org_join_requests (organization_id, user_id, created_at) VALUES ($1, $2, $3) RETURNING id`
	request.CreatedAt = time.Now().UTC()
	return r.pool.QueryRow(ctx, query, request.OrganizationID, request.UserID, request.CreatedAt).Scan(&request.ID)
}

func (r *OrgRepository) FindJoinRequest(ctx context.Context, orgID, userID int64) (*models.JoinRequest, error) {
	query := `SELECT jr.id, jr.organization_id, jr.user_id, u.email, u.name, jr.created_at
		FROM org_join_requests jr
		JOIN users u ON u.id = jr.user_id
		WHERE jr.organization_id = $1 AND jr.user_id = $2`
	request := &models.JoinRequest{}
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(
		&request.ID, &request.OrganizationID, &request.UserID, &request.Email, &request.Name, &request.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find join request: %w", err)
	}
	return request, nil
}

func (r *OrgRepository) FindJoinRequests(ctx context.Context, orgID int64, limit, offset int) ([]models.JoinRequest, error) {
	query := `SELECT jr.id, jr.organization_id, jr.user_id, u.email, u.name, jr.created_at
		FROM org_join_requests jr
		JOIN users u ON u.id = jr.user_id
		WHERE jr.organization_id = $1 ORDER BY jr.id LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find join requests: %w", err)
	}
	defer rows.Close()

	var requests []models.JoinRequest
	for rows.Next() {
		var request models.JoinRequest
		if err := rows.Scan(&request.ID, &request.OrganizationID, &request.UserID, &request.Email, &request.Name, &request.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan join request: %w", err)
		}
		requests = append(requests, request)
	}
	return requests, nil
}

func (r *OrgRepository) CountJoinRequests(ctx context.Context, orgID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM org_join_requests WHERE organization_id = $1`
	var n int64
	if err := r.pool.QueryRow(ctx, query, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count join requests: %w", err)
	}
	return n, nil
}

// DeleteJoinRequest removes a pending join request. It returns ErrNotFound
// when no matching request exists.
func (r *OrgRepository) DeleteJoinRequest(ctx context.Context, orgID, userID int64) error {
	query := `DELETE FROM org_join_requests WHERE organization_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("delete join request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *OrgRepository) FindMembers(ctx context.Context, orgID int64, limit, offset int) ([]models.OrgMember, error) {
	query := `SELECT m.id, m.organization_id, m.user_id, u.name, u.email, m.role, m.suspended_until, m.created_at
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1 ORDER BY m.id LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find members: %w", err)
	}
	defer rows.Close()

	var members []models.OrgMember
	for rows.Next() {
		var m models.OrgMember
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Name, &m.Email, &m.Role, &m.SuspendedUntil, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *OrgRepository) CountMembers(ctx context.Context, orgID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM organization_members WHERE organization_id = $1`
	var n int64
	if err := r.pool.QueryRow(ctx, query, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

// FindMember returns an active member, i.e. one that is not suspended. It is
// the universal access gate: suspended members are treated as not members, so
// the user loses access to the organization until the suspension expires.
// FindMemberAny returns the member row regardless of suspension status.
func (r *OrgRepository) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	query := `SELECT m.id, m.organization_id, m.user_id, u.name, u.email, m.role, m.suspended_until, m.created_at
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1 AND m.user_id = $2
		  AND (m.suspended_until IS NULL OR m.suspended_until <= NOW())`
	m := &models.OrgMember{}
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Name, &m.Email, &m.Role, &m.SuspendedUntil, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find member: %w", err)
	}
	return m, nil
}

func (r *OrgRepository) FindMemberAny(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	query := `SELECT m.id, m.organization_id, m.user_id, u.name, u.email, m.role, m.suspended_until, m.created_at
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1 AND m.user_id = $2`
	m := &models.OrgMember{}
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Name, &m.Email, &m.Role, &m.SuspendedUntil, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find member: %w", err)
	}
	return m, nil
}

func (r *OrgRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.Organization, error) {
	query := `SELECT o.id, o.name, o.slug, o.created_at, o.updated_at
		FROM organizations o
		JOIN organization_members om ON om.organization_id = o.id
		WHERE om.user_id = $1
		  AND (om.suspended_until IS NULL OR om.suspended_until <= NOW())
		ORDER BY o.id LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
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

func (r *OrgRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	query := `SELECT COUNT(*)
		FROM organizations o
		JOIN organization_members om ON om.organization_id = o.id
		WHERE om.user_id = $1
		  AND (om.suspended_until IS NULL OR om.suspended_until <= NOW())`
	var n int64
	if err := r.pool.QueryRow(ctx, query, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count orgs by user: %w", err)
	}
	return n, nil
}
