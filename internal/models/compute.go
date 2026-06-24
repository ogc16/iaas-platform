package models

import "time"

type ComputeInstance struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`
	InstanceType   string    `json:"instance_type"`
	Status         string    `json:"status"`
	Region         string    `json:"region"`
	CPUCores       int       `json:"cpu_cores"`
	MemoryMB       int       `json:"memory_mb"`
	DiskGB         int       `json:"disk_gb"`
	IPAddress      string    `json:"ip_address,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

const (
	InstanceTypeVM        = "vm"
	InstanceTypeContainer = "container"

	InstanceStatusRunning    = "running"
	InstanceStatusStopped    = "stopped"
	InstanceStatusTerminated = "terminated"
)

type CreateInstanceRequest struct {
	Name         string `json:"name"`
	InstanceType string `json:"instance_type"`
	Region       string `json:"region"`
	CPUCores     int    `json:"cpu_cores"`
	MemoryMB     int    `json:"memory_mb"`
	DiskGB       int    `json:"disk_gb"`
}
