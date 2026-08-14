package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

// Service implements webhook endpoint management and event emission.
type Service struct {
	store   WebhookStore
	deliver DeliveryStore
	members MembershipStore
}

func NewService(store WebhookStore, deliver DeliveryStore, members MembershipStore) *Service {
	return &Service{store: store, deliver: deliver, members: members}
}

// Create registers a new webhook endpoint. When the request omits a secret a
// strong random one is generated and returned (once) in the webhook object.
func (s *Service) Create(ctx context.Context, orgID, userID int64, req models.CreateWebhookRequest) (*models.Webhook, error) {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return nil, err
	}

	events := normalizeEvents(req.Events)
	if err := validateRequest(req.URL, events); err != nil {
		return nil, err
	}

	secret := req.Secret
	if secret == "" {
		generated, err := newSecret()
		if err != nil {
			return nil, fmt.Errorf("generate secret: %w", err)
		}
		secret = generated
	}

	w := &models.Webhook{
		OrganizationID: orgID,
		URL:            req.URL,
		Secret:         secret,
		Events:         events,
		Active:         true,
	}
	if err := s.store.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}
	return w, nil
}

// List returns one page of the org's webhooks and the total count. Admins
// only, so secrets and endpoint inventory are not visible to regular members.
func (s *Service) List(ctx context.Context, orgID, userID int64, limit, offset int) ([]models.Webhook, int64, error) {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return nil, 0, err
	}

	webhooks, err := s.store.ListByOrg(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list webhooks: %w", err)
	}
	total, err := s.store.CountByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count webhooks: %w", err)
	}
	return webhooks, total, nil
}

// Get returns one webhook if it belongs to the org. Admins only.
func (s *Service) Get(ctx context.Context, orgID, userID, webhookID int64) (*models.Webhook, error) {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return nil, err
	}
	return s.requireOwned(ctx, orgID, webhookID)
}

// Update applies the set fields of an update request to a webhook.
func (s *Service) Update(ctx context.Context, orgID, userID, webhookID int64, req models.UpdateWebhookRequest) (*models.Webhook, error) {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return nil, err
	}
	w, err := s.requireOwned(ctx, orgID, webhookID)
	if err != nil {
		return nil, err
	}

	if req.URL != nil {
		if err := validateRequest(*req.URL, w.Events); err != nil {
			return nil, err
		}
		w.URL = *req.URL
	}
	if req.Events != nil {
		events := normalizeEvents(*req.Events)
		if err := validateRequest(w.URL, events); err != nil {
			return nil, err
		}
		w.Events = events
	}
	if req.Active != nil {
		w.Active = *req.Active
	}

	if err := s.store.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("update webhook: %w", err)
	}
	return w, nil
}

// Delete removes a webhook and its undelivered queue (cascade).
func (s *Service) Delete(ctx context.Context, orgID, userID, webhookID int64) error {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return err
	}
	if _, err := s.requireOwned(ctx, orgID, webhookID); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, webhookID); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return nil
}

// Ping enqueues a synthetic delivery directly to the webhook so operators can
// verify the endpoint accepts signed payloads.
func (s *Service) Ping(ctx context.Context, orgID, userID, webhookID int64) error {
	if err := requireAdmin(ctx, s.members, orgID, userID); err != nil {
		return err
	}
	w, err := s.requireOwned(ctx, orgID, webhookID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{"webhook_id": w.ID, "message": "webhook ping"})
	if err != nil {
		return fmt.Errorf("marshal ping payload: %w", err)
	}
	if _, err := s.deliver.CreateSingleDelivery(ctx, w.ID, models.EventPing, payload); err != nil {
		return fmt.Errorf("queue ping delivery: %w", err)
	}
	return nil
}

// requireOwned loads a webhook and verifies it belongs to the organization.
func (s *Service) requireOwned(ctx context.Context, orgID, webhookID int64) (*models.Webhook, error) {
	w, err := s.store.FindByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find webhook: %w", err)
	}
	if w.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	return w, nil
}

func newSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
