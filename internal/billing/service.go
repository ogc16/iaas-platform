package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/user/iaas-platform/internal/database"
	"github.com/user/iaas-platform/internal/models"
)

var (
	ErrNotInOrg = errors.New("not a member of this organization")
)

var unitPrices = map[string]int64{
	models.ResourceTypeCPUHours:   50,   // $0.50 per CPU hour
	models.ResourceTypeMemoryGBH:  20,   // $0.20 per GB-hour
	models.ResourceTypeDiskGBH:    1,    // $0.01 per GB-hour
}

type Service struct {
	usageRepo   *database.UsageRepository
	invoiceRepo *database.InvoiceRepository
	orgRepo     *database.OrgRepository
}

func NewService(usageRepo *database.UsageRepository, invoiceRepo *database.InvoiceRepository, orgRepo *database.OrgRepository) *Service {
	return &Service{usageRepo: usageRepo, invoiceRepo: invoiceRepo, orgRepo: orgRepo}
}

func (s *Service) RecordUsage(ctx context.Context, orgID, instanceID int64, resourceType string, quantity float64) error {
	record := &models.UsageRecord{
		OrganizationID: orgID,
		InstanceID:     instanceID,
		ResourceType:   resourceType,
		Quantity:       quantity,
	}
	return s.usageRepo.Record(ctx, record)
}

func (s *Service) GetUsage(ctx context.Context, orgID, userID int64) (*models.UsageSummary, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		return nil, ErrNotInOrg
	}

	since := time.Now().UTC().AddDate(0, 0, -30)
	return s.usageRepo.GetSummary(ctx, orgID, since)
}

func (s *Service) GetInvoices(ctx context.Context, orgID, userID int64) ([]models.Invoice, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		return nil, ErrNotInOrg
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

	inv := &models.Invoice{
		OrganizationID: orgID,
		Currency:       "usd",
		Status:         models.InvoiceStatusPending,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}

	if err := s.invoiceRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
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
		unitPrice := unitPrices[item.rt]
		amountCents := int64(item.qty * float64(unitPrice))
		totalCents += amountCents

		li := &models.InvoiceLineItem{
			InvoiceID:      inv.ID,
			Description:    item.desc,
			ResourceType:   item.rt,
			Quantity:       item.qty,
			UnitPriceCents: unitPrice,
			AmountCents:    amountCents,
		}
		if err := s.invoiceRepo.AddLineItem(ctx, li); err != nil {
			return nil, fmt.Errorf("add line item: %w", err)
		}
	}

	inv.AmountCents = totalCents
	return inv, nil
}

func (s *Service) GetInvoiceLineItems(ctx context.Context, invoiceID int64) ([]models.InvoiceLineItem, error) {
	return s.invoiceRepo.GetLineItems(ctx, invoiceID)
}
