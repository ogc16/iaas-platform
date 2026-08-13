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
	ProviderID     string    `json:"provider_id"`
	Image          string    `json:"image"`
	Port           int       `json:"port"`
	CPUCores       int       `json:"cpu_cores"`
	MemoryMB       int       `json:"memory_mb"`
	DiskGB         int       `json:"disk_gb"`
	IPAddress      string    `json:"ip_address,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Instance lifecycle states. The control plane records user intent and the
// reconciler advances transient states based on the provider's reported state.
const (
	InstanceStatusPending     = "pending"
	InstanceStatusRunning     = "running"
	InstanceStatusStopping    = "stopping"
	InstanceStatusStopped     = "stopped"
	InstanceStatusTerminating = "terminating"
	InstanceStatusTerminated  = "terminated"
	InstanceStatusFailed      = "failed"
)

const (
	InstanceTypeVM        = "vm"
	InstanceTypeContainer = "container"
)

// ActiveStates are the non-terminal states that consume quota and capacity.
var ActiveStates = []string{
	InstanceStatusPending,
	InstanceStatusRunning,
	InstanceStatusStopping,
	InstanceStatusStopped,
	InstanceStatusTerminating,
	InstanceStatusFailed,
}

type CreateInstanceRequest struct {
	Name         string `json:"name"`
	InstanceType string `json:"instance_type"`
	Region       string `json:"region"`
	Image        string `json:"image"`
	Port         int    `json:"port"`
	CPUCores     int    `json:"cpu_cores"`
	MemoryMB     int    `json:"memory_mb"`
	DiskGB       int    `json:"disk_gb"`
}
