package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeUsageStore struct {
	records    []*models.UsageRecord
	summary    *models.UsageSummary
	summaryErr error
	recordErr  error
}

func (f *fakeUsageStore) Record(ctx context.Context, u *models.UsageRecord) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.records = append(f.records, u)
	return nil
}

func (f *fakeUsageStore) GetSummary(ctx context.Context, orgID int64, since time.Time) (*models.UsageSummary, error) {
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	if f.summary == nil {
		return &models.UsageSummary{}, nil
	}
	return f.summary, nil
}

type fakeInvoiceStore struct {
	created   []*models.Invoice
	lineItems []models.InvoiceLineItem
	createErr error
	itemErr   error
}

func (f *fakeInvoiceStore) Create(ctx context.Context, inv *models.Invoice) error {
	if f.createErr != nil {
		return f.createErr
	}
	inv.ID = int64(len(f.created) + 1)
	f.created = append(f.created, inv)
	return nil
}

func (f *fakeInvoiceStore) AddLineItem(ctx context.Context, item *models.InvoiceLineItem) error {
	if f.itemErr != nil {
		return f.itemErr
	}
	f.lineItems = append(f.lineItems, *item)
	return nil
}

func (f *fakeInvoiceStore) ListByOrg(ctx context.Context, orgID int64) ([]models.Invoice, error) {
	var out []models.Invoice
	for _, inv := range f.created {
		if inv.OrganizationID == orgID {
			out = append(out, *inv)
		}
	}
	return out, nil
}

func (f *fakeInvoiceStore) GetLineItems(ctx context.Context, invoiceID int64) ([]models.InvoiceLineItem, error) {
	return f.lineItems, nil
}

type fakeMembershipStore struct {
	ok      bool
	findErr error
}

func (f *fakeMembershipStore) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.ok {
		return &models.OrgMember{OrganizationID: orgID, UserID: userID, Role: "member"}, nil
	}
	return nil, database.ErrNotFound
}

func newTestService() (*Service, *fakeUsageStore, *fakeInvoiceStore, *fakeMembershipStore) {
	usage := &fakeUsageStore{}
	invoices := &fakeInvoiceStore{}
	members := &fakeMembershipStore{ok: true}
	return NewService(usage, invoices, members), usage, invoices, members
}

func TestService_RecordUsage_Success(t *testing.T) {
	svc, usage, _, _ := newTestService()

	if err := svc.RecordUsage(context.Background(), 1, 5, models.ResourceTypeCPUHours, 2.5); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	if len(usage.records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(usage.records))
	}
	if usage.records[0].ResourceType != models.ResourceTypeCPUHours || usage.records[0].Quantity != 2.5 {
		t.Fatalf("unexpected record: %+v", usage.records[0])
	}
}

func TestService_RecordUsage_UnknownResourceType(t *testing.T) {
	svc, _, _, _ := newTestService()

	err := svc.RecordUsage(context.Background(), 1, 5, "banana_hours", 1)
	if !errors.Is(err, ErrBadUsage) {
		t.Fatalf("expected ErrBadUsage, got %v", err)
	}
}

func TestService_RecordUsage_NonPositiveQuantity(t *testing.T) {
	svc, _, _, _ := newTestService()

	if err := svc.RecordUsage(context.Background(), 1, 5, models.ResourceTypeCPUHours, 0); err == nil {
		t.Fatal("expected an error for zero quantity")
	}
}

func TestService_RecordUsage_PropagatesRepoError(t *testing.T) {
	svc, usage, _, _ := newTestService()
	usage.recordErr = errors.New("connection refused")

	if err := svc.RecordUsage(context.Background(), 1, 5, models.ResourceTypeCPUHours, 1); err == nil {
		t.Fatal("expected a real error to propagate")
	}
}

func TestService_GetUsage_NotInOrg(t *testing.T) {
	svc, _, _, members := newTestService()
	members.ok = false

	if _, err := svc.GetUsage(context.Background(), 1, 2); !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_GetUsage_PropagatesMembershipError(t *testing.T) {
	svc, _, _, members := newTestService()
	members.findErr = errors.New("connection refused")

	if _, err := svc.GetUsage(context.Background(), 1, 2); err == nil || errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_GetUsage_Success(t *testing.T) {
	svc, usage, _, _ := newTestService()
	usage.summary = &models.UsageSummary{CPUHours: 10, MemoryGBHours: 20, DiskGBHours: 100}

	got, err := svc.GetUsage(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if got.CPUHours != 10 || got.MemoryGBHours != 20 || got.DiskGBHours != 100 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestService_GenerateInvoice_ComputesAmounts(t *testing.T) {
	svc, usage, invoices, _ := newTestService()
	usage.summary = &models.UsageSummary{CPUHours: 10, MemoryGBHours: 20, DiskGBHours: 100}

	inv, err := svc.GenerateInvoice(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateInvoice: %v", err)
	}

	// 10 * 50 + 20 * 20 + 100 * 1 = 1000 cents.
	if inv.AmountCents != 1000 {
		t.Fatalf("expected amount 1000 cents, got %d", inv.AmountCents)
	}
	if len(invoices.created) != 1 || invoices.created[0].AmountCents != 1000 {
		t.Fatalf("invoice must be persisted with the computed amount, got %+v", invoices.created)
	}
	if len(invoices.lineItems) != 3 {
		t.Fatalf("expected 3 line items, got %d", len(invoices.lineItems))
	}
	if invoices.lineItems[0].UnitPriceCents != 50 || invoices.lineItems[2].UnitPriceCents != 1 {
		t.Fatalf("unexpected unit prices: %+v", invoices.lineItems)
	}
}

func TestService_GenerateInvoice_EmptyUsage(t *testing.T) {
	svc, _, invoices, _ := newTestService()

	inv, err := svc.GenerateInvoice(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateInvoice: %v", err)
	}
	if inv.AmountCents != 0 {
		t.Fatalf("expected 0 for empty usage, got %d", inv.AmountCents)
	}
	if len(invoices.lineItems) != 0 {
		t.Fatalf("expected no line items for empty usage, got %d", len(invoices.lineItems))
	}
}

func TestService_GenerateInvoice_PropagatesUsageError(t *testing.T) {
	svc, usage, _, _ := newTestService()
	usage.summaryErr = errors.New("connection refused")

	if _, err := svc.GenerateInvoice(context.Background(), 1); err == nil {
		t.Fatal("expected a real error to propagate")
	}
}

func TestService_GetInvoices_NotInOrg(t *testing.T) {
	svc, _, _, members := newTestService()
	members.ok = false

	if _, err := svc.GetInvoices(context.Background(), 1, 2); !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_GetInvoiceLineItems(t *testing.T) {
	svc, _, invoices, _ := newTestService()
	invoices.lineItems = []models.InvoiceLineItem{{ID: 1, Description: "CPU Hours"}}

	items, err := svc.GetInvoiceLineItems(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetInvoiceLineItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(items))
	}
}
