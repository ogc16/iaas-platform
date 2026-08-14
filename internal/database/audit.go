package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogc16/iaas-platform/internal/models"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

// Record appends an event to the audit trail.
func (r *AuditRepository) Record(ctx context.Context, e *models.AuditEvent) error {
	metadata := "{}"
	if e.Metadata != nil {
		if b, err := json.Marshal(e.Metadata); err == nil {
			metadata = string(b)
		}
	}
	query := `INSERT INTO audit_events
		(organization_id, user_id, actor_email, action, resource_type, resource_id, metadata, ip, request_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10) RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query,
		e.OrganizationID, e.UserID, e.ActorEmail, e.Action, e.ResourceType, e.ResourceID,
		metadata, e.IP, e.RequestID, e.CreatedAt,
	).Scan(&e.ID, &e.CreatedAt)
}

func (r *AuditRepository) List(ctx context.Context, orgID int64, f models.AuditFilter) ([]models.AuditEvent, error) {
	where, args := buildAuditWhere(orgID, f)
	query := `SELECT id, organization_id, user_id, actor_email, action, resource_type, resource_id,
			metadata, ip, request_id, created_at
		FROM audit_events` + where + ` ORDER BY created_at DESC, id DESC LIMIT ` +
		strconv.Itoa(f.Limit) + ` OFFSET ` + strconv.Itoa(f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	var events []models.AuditEvent
	for rows.Next() {
		e, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *e)
	}
	return events, nil
}

func (r *AuditRepository) Count(ctx context.Context, orgID int64, f models.AuditFilter) (int64, error) {
	where, args := buildAuditWhere(orgID, f)
	query := `SELECT COUNT(*) FROM audit_events` + where
	var n int64
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count audit events: %w", err)
	}
	return n, nil
}

// buildAuditWhere returns the WHERE clause and its arguments. Parameter
// placeholders are numbered after the fixed orgID argument.
func buildAuditWhere(orgID int64, f models.AuditFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"organization_id = $1"}

	if f.UserID != nil {
		args = append(args, *f.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if f.Action != "" {
		args = append(args, f.Action)
		clauses = append(clauses, fmt.Sprintf("action = $%d", len(args)))
	}
	if f.Resource != "" {
		args = append(args, f.Resource)
		clauses = append(clauses, fmt.Sprintf("resource_id = $%d", len(args)))
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

type auditScanner interface {
	Scan(dest ...any) error
}

func scanAuditEvent(row auditScanner) (*models.AuditEvent, error) {
	var (
		e          models.AuditEvent
		metadata   []byte
		resourceID *string
		ip         *string
		requestID  *string
	)
	if err := row.Scan(&e.ID, &e.OrganizationID, &e.UserID, &e.ActorEmail, &e.Action, &e.ResourceType,
		&resourceID, &metadata, &ip, &requestID, &e.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan audit event: %w", err)
	}
	if resourceID != nil {
		e.ResourceID = *resourceID
	}
	if ip != nil {
		e.IP = *ip
	}
	if requestID != nil {
		e.RequestID = *requestID
	}
	if len(metadata) > 0 && string(metadata) != "null" {
		_ = json.Unmarshal(metadata, &e.Metadata)
	}
	return &e, nil
}
