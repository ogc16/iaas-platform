package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrNotInOrg = errors.New("not a member of this organization")
	ErrBadUsage = errors.New("unknown resource type")
)

type UsageStore interface {
	Record(ctx context.Context, u *models.UsageRecord) error
	GetSummary(ctx context.Context, orgID int64, since time.Time) (*models.UsageSummary, error)
}

type InvoiceStore interface {
	Create(ctx context.Context, inv *models.Invoice) error
	AddLineItem(ctx context.Context, item *models.InvoiceLineItem) error
	ListByOrg(ctx context.Context, orgID int64) ([]models.Invoice, error)
	GetLineItems(ctx context.Context, invoiceID int64) ([]models.InvoiceLineItem, error)
}

type MembershipStore interface {
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
}

var unitPrices = map[string]int64{
	models.ResourceTypeCPUHours:  50, // $0.50 per CPU hour
	models.ResourceTypeMemoryGBH: 20, // $0.20 per GB-hour
	models.ResourceTypeDiskGBH:   1,  // $0.01 per GB-hour
}

type Service struct {
	usageRepo   UsageStore
	invoiceRepo InvoiceStore
	orgRepo     MembershipStore
}

func NewService(usageRepo UsageStore, invoiceRepo InvoiceStore, orgRepo MembershipStore) *Service {
	return &Service{usageRepo: usageRepo, invoiceRepo: invoiceRepo, orgRepo: orgRepo}
}

func (s *Service) RecordUsage(ctx context.Context, orgID, instanceID int64, resourceType string, quantity float64) error {
	if _, ok := unitPrices[resourceType]; !ok {
		return fmt.Errorf("%w: %s", ErrBadUsage, resourceType)
	}
	if quantity <= 0 {
		return errors.New("quantity must be greater than zero")
	}

	record := &models.UsageRecord{
		OrganizationID: orgID,
		InstanceID:     instanceID,
		ResourceType:   resourceType,
		Quantity:       quantity,
	}
	if err := s.usageRepo.Record(ctx, record); err != nil {
		return fmt.Errorf("record usage: %w", err)
	}
	return nil
}

func (s *Service) GetUsage(ctx context.Context, orgID, userID int64) (*models.UsageSummary, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}

	since := time.Now().UTC().AddDate(0, 0, -30)
	summary, err := s.usageRepo.GetSummary(ctx, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	return summary, nil
}

func (s *Service) GetInvoices(ctx context.Context, orgID, userID int64) ([]models.Invoice, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}
	return s.invoiceRepo.ListByOrg(ctx, orgID)
}

func (s *Service) GenerateInvoice(ctx context.Context, orgID int64) (*models.Invoice, error) {
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	usage, err := s.usageRepo.GetSummary(ctx, orgID, periodStart)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}

	items := []struct {
		rt   string
		qty  float64
		desc string
	}{
		{models.ResourceTypeCPUHours, usage.CPUHours, "CPU Hours"},
		{models.ResourceTypeMemoryGBH, usage.MemoryGBHours, "Memory GB-hours"},
		{models.ResourceTypeDiskGBH, usage.DiskGBHours, "Disk GB-hours"},
	}

	var totalCents int64
	for _, item := range items {
		if item.qty <= 0 {
			continue
		}
		totalCents += int64(item.qty * float64(unitPrices[item.rt]))
	}

	inv := &models.Invoice{
		OrganizationID: orgID,
		AmountCents:    totalCents,
		Currency:       "usd",
		Status:         models.InvoiceStatusPending,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}

	if err := s.invoiceRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	for _, item := range items {
		if item.qty <= 0 {
			continue
		}
		li := &models.InvoiceLineItem{
			InvoiceID:      inv.ID,
			Description:    item.desc,
			ResourceType:   item.rt,
			Quantity:       item.qty,
			UnitPriceCents: unitPrices[item.rt],
			AmountCents:    int64(item.qty * float64(unitPrices[item.rt])),
		}
		if err := s.invoiceRepo.AddLineItem(ctx, li); err != nil {
			return nil, fmt.Errorf("add line item: %w", err)
		}
	}

	return inv, nil
}

func (s *Service) GetInvoiceLineItems(ctx context.Context, invoiceID int64) ([]models.InvoiceLineItem, error) {
	return s.invoiceRepo.GetLineItems(ctx, invoiceID)
}
