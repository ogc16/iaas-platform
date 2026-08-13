package models

// OrgUsage is the aggregate of resources consumed by an organization's
// non-terminal instances. Used to enforce per-org quotas.
type OrgUsage struct {
	Count    int64 `json:"count"`
	CPUCores int64 `json:"cpu_cores"`
	MemoryMB int64 `json:"memory_mb"`
	DiskGB   int64 `json:"disk_gb"`
}

// RegionUsage is the aggregate of resources consumed in a region by all
// non-terminal instances. Used to enforce regional capacity.
type RegionUsage struct {
	CPUCores int64 `json:"cpu_cores"`
	MemoryMB int64 `json:"memory_mb"`
	DiskGB   int64 `json:"disk_gb"`
}

// RegionCapacity is the total allocatable capacity of a region.
type RegionCapacity struct {
	Region   string `json:"region"`
	CPUCores int64  `json:"cpu_cores"`
	MemoryMB int64  `json:"memory_mb"`
	DiskGB   int64  `json:"disk_gb"`
}

// Quota is a per-organization resource allowance. A missing quota row
// resolves to DefaultQuota.
type Quota struct {
	OrganizationID int64 `json:"organization_id"`
	MaxInstances   int64 `json:"max_instances"`
	MaxCPUCores    int64 `json:"max_cpu_cores"`
	MaxMemoryMB    int64 `json:"max_memory_mb"`
	MaxDiskGB      int64 `json:"max_disk_gb"`
}

var DefaultQuota = Quota{
	MaxInstances: 20,
	MaxCPUCores:  16,
	MaxMemoryMB:  32768,
	MaxDiskGB:    500,
}
