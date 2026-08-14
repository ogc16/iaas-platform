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

type WebhookRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookRepository(pool *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

func (r *WebhookRepository) Create(ctx context.Context, w *models.Webhook) error {
	query := `INSERT INTO webhooks (organization_id, url, secret, events, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) RETURNING id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query, w.OrganizationID, w.URL, w.Secret, w.Events, w.Active).
		Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *WebhookRepository) FindByID(ctx context.Context, id int64) (*models.Webhook, error) {
	query := `SELECT id, organization_id, url, secret, events, active, created_at, updated_at
		FROM webhooks WHERE id = $1`
	w := &models.Webhook{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find webhook: %w", err)
	}
	return w, nil
}

func (r *WebhookRepository) ListByOrg(ctx context.Context, orgID int64, limit, offset int) ([]models.Webhook, error) {
	query := `SELECT id, organization_id, url, secret, events, active, created_at, updated_at
		FROM webhooks WHERE organization_id = $1 ORDER BY id LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, nil
}

func (r *WebhookRepository) CountByOrg(ctx context.Context, orgID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM webhooks WHERE organization_id = $1`
	var n int64
	if err := r.pool.QueryRow(ctx, query, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count webhooks: %w", err)
	}
	return n, nil
}

func (r *WebhookRepository) Update(ctx context.Context, w *models.Webhook) error {
	query := `UPDATE webhooks SET url = $2, secret = $3, events = $4, active = $5, updated_at = NOW()
		WHERE id = $1 RETURNING updated_at`
	return r.pool.QueryRow(ctx, query, w.ID, w.URL, w.Secret, w.Events, w.Active).Scan(&w.UpdatedAt)
}

// Delete removes a webhook. It returns ErrNotFound when no such webhook exists.
func (r *WebhookRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListActiveByOrgEvent returns the active webhooks of an organization that
// subscribe to the given event.
func (r *WebhookRepository) ListActiveByOrgEvent(ctx context.Context, orgID int64, event string) ([]models.Webhook, error) {
	query := `SELECT id, organization_id, url, secret, events, active, created_at, updated_at
		FROM webhooks WHERE organization_id = $1 AND active AND $2 = ANY(events) ORDER BY id`
	rows, err := r.pool.Query(ctx, query, orgID, event)
	if err != nil {
		return nil, fmt.Errorf("list webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, nil
}

// CreateDeliveries enqueues one delivery per active webhook subscribed to the
// event, returning how many deliveries were created.
func (r *WebhookRepository) CreateDeliveries(ctx context.Context, orgID int64, event string, payload []byte) (int, error) {
	query := `INSERT INTO webhook_deliveries (webhook_id, event, payload)
		SELECT id, $2, $3::jsonb FROM webhooks
		WHERE organization_id = $1 AND active AND $2 = ANY(events)`
	tag, err := r.pool.Exec(ctx, query, orgID, event, string(payload))
	if err != nil {
		return 0, fmt.Errorf("enqueue deliveries: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// CreateSingleDelivery enqueues a delivery to one specific webhook (used by
// the ping endpoint), returning the delivery id.
func (r *WebhookRepository) CreateSingleDelivery(ctx context.Context, webhookID int64, event string, payload []byte) (int64, error) {
	query := `INSERT INTO webhook_deliveries (webhook_id, event, payload) VALUES ($1, $2, $3::jsonb) RETURNING id`
	var id int64
	if err := r.pool.QueryRow(ctx, query, webhookID, event, string(payload)).Scan(&id); err != nil {
		return 0, fmt.Errorf("enqueue single delivery: %w", err)
	}
	return id, nil
}

// ListDue returns the next due deliveries. It joins onto webhooks so the
// dispatcher gets the endpoint URL and signing secret in the same row, and it
// skips deliveries whose webhook has been deactivated or deleted.
func (r *WebhookRepository) ListDue(ctx context.Context, limit int) ([]models.WebhookDelivery, error) {
	query := `SELECT d.id, d.webhook_id, w.organization_id, d.event, d.payload, d.status,
			d.attempts, d.max_attempts, d.next_attempt_at, w.url, w.secret
		FROM webhook_deliveries d
		JOIN webhooks w ON w.id = d.webhook_id
		WHERE w.active AND d.status IN ('pending', 'failed') AND d.next_attempt_at <= NOW()
		ORDER BY d.next_attempt_at LIMIT $1`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list due deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []models.WebhookDelivery
	for rows.Next() {
		var d models.WebhookDelivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.OrganizationID, &d.Event, &d.Payload, &d.Status,
			&d.Attempts, &d.MaxAttempts, &d.NextAttemptAt, &d.URL, &d.Secret); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, nil
}

// MarkDelivered records a successful delivery.
func (r *WebhookRepository) MarkDelivered(ctx context.Context, id int64) error {
	query := `UPDATE webhook_deliveries SET status = 'delivered', delivered_at = NOW(), last_error = NULL WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkFailed records a failed attempt, incrementing the attempt counter and
// scheduling the next attempt (or marking the delivery failed for good).
func (r *WebhookRepository) MarkFailed(ctx context.Context, id int64, status, lastErr string, nextAttemptAt time.Time) error {
	query := `UPDATE webhook_deliveries
		SET status = $2, attempts = attempts + 1, last_error = $3, next_attempt_at = $4
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, status, lastErr, nextAttemptAt)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
