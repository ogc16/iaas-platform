package compute

import "context"

// Provider abstracts the underlying infrastructure that runs instances. The
// control plane holds intent and reconciles against whatever a provider
// reports as the real state, which means a provider can be a faithful
// simulation (the default) or a real backend such as Docker.
type Provider interface {
	// Name identifies the provider implementation for diagnostics.
	Name() string
	// Provision requests a new instance. It is asynchronous: the returned
	// state is the provider's initial state, not necessarily "running".
	Provision(ctx context.Context, spec ProviderSpec) (*ProviderInstance, error)
	// Start requests that a stopped instance be brought up again.
	Start(ctx context.Context, providerID string) error
	// Stop requests that a running instance be shut down.
	Stop(ctx context.Context, providerID string) error
	// Terminate requests that an instance be destroyed for good.
	Terminate(ctx context.Context, providerID string) error
	// GetState returns the provider's current view of the instance state.
	GetState(ctx context.Context, providerID string) (string, error)
}

type ProviderSpec struct {
	ProviderID     string
	OrganizationID int64
	InstanceID     int64
	Name           string
	Image          string
	Region         string
	CPUCores       int
	MemoryMB       int
	DiskGB         int
	Port           int
}

type ProviderInstance struct {
	ProviderID string
	State      string
}
