package webhooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeMembers struct {
	member *models.OrgMember
	err    error
}

func (f *fakeMembers) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.member == nil {
		return nil, database.ErrNotFound
	}
	return f.member, nil
}

type fakeStore struct {
	webhooks []models.Webhook
	nextID   int64
	created  []models.Webhook
	deleted  []int64
	updated  []models.Webhook
}

func (f *fakeStore) Create(ctx context.Context, w *models.Webhook) error {
	f.nextID++
	w.ID = f.nextID
	f.webhooks = append(f.webhooks, *w)
	f.created = append(f.created, *w)
	return nil
}

func (f *fakeStore) FindByID(ctx context.Context, id int64) (*models.Webhook, error) {
	for i := range f.webhooks {
		if f.webhooks[i].ID == id {
			return &f.webhooks[i], nil
		}
	}
	return nil, database.ErrNotFound
}

func (f *fakeStore) ListByOrg(ctx context.Context, orgID int64, limit, offset int) ([]models.Webhook, error) {
	var out []models.Webhook
	for _, w := range f.webhooks {
		if w.OrganizationID == orgID {
			out = append(out, w)
		}
	}
	return out, nil
}

func (f *fakeStore) CountByOrg(ctx context.Context, orgID int64) (int64, error) {
	var n int64
	for _, w := range f.webhooks {
		if w.OrganizationID == orgID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) Update(ctx context.Context, w *models.Webhook) error {
	for i := range f.webhooks {
		if f.webhooks[i].ID == w.ID {
			f.webhooks[i] = *w
			f.updated = append(f.updated, *w)
			return nil
		}
	}
	return database.ErrNotFound
}

func (f *fakeStore) Delete(ctx context.Context, id int64) error {
	for i, w := range f.webhooks {
		if w.ID == id {
			f.webhooks = append(f.webhooks[:i], f.webhooks[i+1:]...)
			f.deleted = append(f.deleted, id)
			return nil
		}
	}
	return database.ErrNotFound
}

type fakeDeliveryStore struct {
	deliveries []models.WebhookDelivery
}

func (f *fakeDeliveryStore) CreateDeliveries(ctx context.Context, orgID int64, event string, payload []byte) (int, error) {
	return 0, nil
}

func (f *fakeDeliveryStore) CreateSingleDelivery(ctx context.Context, webhookID int64, event string, payload []byte) (int64, error) {
	return 1, nil
}

func (f *fakeDeliveryStore) ListDue(ctx context.Context, limit int) ([]models.WebhookDelivery, error) {
	return f.deliveries, nil
}

func (f *fakeDeliveryStore) MarkDelivered(ctx context.Context, id int64) error {
	return nil
}

func (f *fakeDeliveryStore) MarkFailed(ctx context.Context, id int64, status, lastErr string, nextAttemptAt time.Time) error {
	return nil
}

func newTestService(member *models.OrgMember) (*Service, *fakeStore) {
	store := &fakeStore{}
	return NewService(store, &fakeDeliveryStore{}, &fakeMembers{member: member}), store
}

func adminMember() *models.OrgMember {
	return &models.OrgMember{Role: "admin"}
}

func TestCreateValidWebhook(t *testing.T) {
	svc, store := newTestService(adminMember())

	w, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://hooks.example.com/iaas",
		Events: []string{models.EventInstanceCreated, models.EventInstanceTerminated},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.URL != "https://hooks.example.com/iaas" {
		t.Errorf("unexpected url %q", w.URL)
	}
	if len(w.Events) != 2 {
		t.Errorf("expected 2 events, got %v", w.Events)
	}
	if len(w.Secret) != 64 {
		t.Errorf("expected generated 64-char hex secret, got %q", w.Secret)
	}
	if !w.Active {
		t.Error("expected webhook active by default")
	}
	if len(store.created) != 1 {
		t.Errorf("expected 1 created webhook, got %d", len(store.created))
	}
}

func TestCreateRejectsNonAdmin(t *testing.T) {
	svc, _ := newTestService(&models.OrgMember{Role: "member"})
	_, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://hooks.example.com/iaas",
		Events: []string{models.EventInstanceCreated},
	})
	if !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestCreateRejectsInvalidURL(t *testing.T) {
	svc, _ := newTestService(adminMember())
	_, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "not-a-url",
		Events: []string{models.EventInstanceCreated},
	})
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("expected ErrInvalidURL, got %v", err)
	}
}

func TestCreateRejectsInvalidEvent(t *testing.T) {
	svc, _ := newTestService(adminMember())
	_, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://hooks.example.com/iaas",
		Events: []string{"not.an.event"},
	})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestCreateRejectsNoEvents(t *testing.T) {
	svc, _ := newTestService(adminMember())
	_, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://hooks.example.com/iaas",
		Events: nil,
	})
	if !errors.Is(err, ErrEmptyEvents) {
		t.Fatalf("expected ErrEmptyEvents, got %v", err)
	}
}

func TestUpdateAppliesSetFields(t *testing.T) {
	svc, store := newTestService(adminMember())
	w, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://a.example.com/hook",
		Events: []string{models.EventInstanceCreated},
	})
	if err != nil {
		t.Fatal(err)
	}

	active := false
	events := []string{models.EventUsageRecorded}
	updated, err := svc.Update(context.Background(), 7, 1, w.ID, models.UpdateWebhookRequest{
		Active: &active,
		Events: &events,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Active {
		t.Error("expected webhook deactivated")
	}
	if len(updated.Events) != 1 || updated.Events[0] != models.EventUsageRecorded {
		t.Errorf("expected events replaced, got %v", updated.Events)
	}
	if updated.URL != "https://a.example.com/hook" {
		t.Errorf("unexpected url change: %q", updated.URL)
	}
	if len(store.updated) != 1 {
		t.Errorf("expected 1 update call, got %d", len(store.updated))
	}
}

func TestDeleteScopesToOrg(t *testing.T) {
	svc, store := newTestService(adminMember())
	w, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://a.example.com/hook",
		Events: []string{models.EventInstanceCreated},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A different org cannot delete this webhook.
	if err := svc.Delete(context.Background(), 99, 1, w.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong org, got %v", err)
	}
	if len(store.deleted) != 0 {
		t.Fatal("webhook must not be deleted across orgs")
	}
	if err := svc.Delete(context.Background(), 7, 1, w.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(store.deleted) != 1 {
		t.Fatal("expected the webhook to be deleted")
	}
}

func TestPingRequiresAdminAndQueuesDelivery(t *testing.T) {
	store := &fakeStore{}
	members := &fakeMembers{member: adminMember()}
	svc := NewService(store, &fakeDeliveryStore{}, members)

	w, err := svc.Create(context.Background(), 7, 1, models.CreateWebhookRequest{
		URL:    "https://a.example.com/hook",
		Events: []string{models.EventInstanceCreated},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Non-admin ping is refused.
	members.member = &models.OrgMember{Role: "member"}
	if err := svc.Ping(context.Background(), 7, 2, w.ID); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin for ping, got %v", err)
	}
	// Admin ping succeeds against the delivery store.
	members.member = adminMember()
	if err := svc.Ping(context.Background(), 7, 1, w.ID); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
