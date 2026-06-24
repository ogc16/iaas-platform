package models

import "time"

type UsageRecord struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	InstanceID     int64     `json:"instance_id"`
	ResourceType   string    `json:"resource_type"`
	Quantity       float64   `json:"quantity"`
	RecordedAt     time.Time `json:"recorded_at"`
}

const (
	ResourceTypeCPUHours   = "cpu_hours"
	ResourceTypeMemoryGBH  = "memory_gb_hours"
	ResourceTypeDiskGBH    = "disk_gb_hours"
)

type Invoice struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	AmountCents    int64     `json:"amount_cents"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	CreatedAt      time.Time `json:"created_at"`
}

const (
	InvoiceStatusPending = "pending"
	InvoiceStatusPaid    = "paid"
	InvoiceStatusOverdue = "overdue"
)

type InvoiceLineItem struct {
	ID            int64   `json:"id"`
	InvoiceID     int64   `json:"invoice_id"`
	Description   string  `json:"description"`
	ResourceType  string  `json:"resource_type"`
	Quantity      float64 `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	AmountCents   int64   `json:"amount_cents"`
}

type UsageSummary struct {
	CPUHours     float64 `json:"cpu_hours"`
	MemoryGBHours float64 `json:"memory_gb_hours"`
	DiskGBHours  float64 `json:"disk_gb_hours"`
}
