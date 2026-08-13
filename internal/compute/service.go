package compute

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrNotFound             = errors.New("instance not found")
	ErrNotInOrg             = errors.New("not a member of this organization")
	ErrQuotaExceeded        = errors.New("organization quota exceeded")
	ErrInsufficientCapacity = errors.New("insufficient regional capacity")
	ErrUnknownRegion        = errors.New("unknown region")
	ErrInvalidTransition    = errors.New("invalid state transition")
)

type InstanceStore interface {
	Create(ctx context.Context, inst *models.ComputeInstance) error
	FindByID(ctx context.Context, id int64) (*models.ComputeInstance, error)
	ListByOrg(ctx context.Context, orgID int64) ([]models.ComputeInstance, error)
	ListActive(ctx context.Context) ([]models.ComputeInstance, error)
	SumActiveByOrg(ctx context.Context, orgID int64) (models.OrgUsage, error)
	SumActiveByRegion(ctx context.Context, region string) (models.RegionUsage, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
}

type MembershipStore interface {
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
}

type QuotaStore interface {
	Get(ctx context.Context, orgID int64) (models.Quota, error)
}

type CapacityStore interface {
	GetRegion(ctx context.Context, region string) (models.RegionCapacity, error)
}

type Service struct {
	repo     InstanceStore
	orgRepo  MembershipStore
	provider Provider
	quotas   QuotaStore
	capacity CapacityStore
}

func NewService(repo InstanceStore, orgRepo MembershipStore, provider Provider, quotas QuotaStore, capacity CapacityStore) *Service {
	return &Service{repo: repo, orgRepo: orgRepo, provider: provider, quotas: quotas, capacity: capacity}
}

// instanceTransitions describes the instance lifecycle as a directed graph.
// User actions request transitions into the transient states; the reconciler
// advances transient states to their settled outcomes based on provider state.
var instanceTransitions = map[string]map[string]bool{
	models.InstanceStatusPending: {
		models.InstanceStatusRunning:     true, // reconciler: provisioning finished
		models.InstanceStatusTerminating: true, // user: terminate
	},
	models.InstanceStatusRunning: {
		models.InstanceStatusStopping:    true, // user: stop
		models.InstanceStatusTerminating: true, // user: terminate
	},
	models.InstanceStatusStopping: {
		models.InstanceStatusStopped:     true, // reconciler: stop finished
		models.InstanceStatusTerminating: true, // user: terminate
	},
	models.InstanceStatusStopped: {
		models.InstanceStatusPending:     true, // user: start
		models.InstanceStatusTerminating: true, // user: terminate
	},
	models.InstanceStatusTerminating: {
		models.InstanceStatusTerminated: true, // reconciler: destroy finished
	},
	models.InstanceStatusTerminated: {},
	models.InstanceStatusFailed: {
		models.InstanceStatusTerminating: true, // user: clean up a failed instance
	},
}

func canTransition(from, to string) bool {
	allowed, ok := instanceTransitions[from]
	return ok && allowed[to]
}

func (s *Service) Create(ctx context.Context, orgID, userID int64, req models.CreateInstanceRequest) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}

	instanceType := req.InstanceType
	if instanceType == "" {
		instanceType = models.InstanceTypeVM
	}
	region := req.Region
	if region == "" {
		region = "us-east-1"
	}
	image := req.Image
	if image == "" {
		image = "debian-12"
	}
	cpu, mem, disk := req.CPUCores, req.MemoryMB, req.DiskGB
	if cpu <= 0 {
		cpu = 1
	}
	if mem <= 0 {
		mem = 1024
	}
	if disk <= 0 {
		disk = 10
	}

	capacity, err := s.capacity.GetRegion(ctx, region)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrUnknownRegion
		}
		return nil, fmt.Errorf("get region capacity: %w", err)
	}

	if err := s.enforceQuota(ctx, orgID, cpu, mem, disk); err != nil {
		return nil, err
	}
	if err := s.enforceCapacity(ctx, region, capacity, cpu, mem, disk); err != nil {
		return nil, err
	}

	providerID, err := newProviderID()
	if err != nil {
		return nil, fmt.Errorf("generate provider id: %w", err)
	}

	inst := &models.ComputeInstance{
		OrganizationID: orgID,
		UserID:         userID,
		Name:           req.Name,
		InstanceType:   instanceType,
		Status:         models.InstanceStatusPending,
		Region:         region,
		ProviderID:     providerID,
		Image:          image,
		Port:           req.Port,
		CPUCores:       cpu,
		MemoryMB:       mem,
		DiskGB:         disk,
		IPAddress:      fmt.Sprintf("10.0.%d.%d", mrand.Intn(255), mrand.Intn(255)),
	}

	pi, err := s.provider.Provision(ctx, ProviderSpec{
		ProviderID:     providerID,
		OrganizationID: orgID,
		InstanceID:     inst.ID,
		Name:           req.Name,
		Image:          image,
		Region:         region,
		CPUCores:       cpu,
		MemoryMB:       mem,
		DiskGB:         disk,
		Port:           req.Port,
	})
	if err != nil {
		return nil, fmt.Errorf("provision instance: %w", err)
	}
	inst.Status = pi.State
	inst.ProviderID = pi.ProviderID

	if err := s.repo.Create(ctx, inst); err != nil {
		if terr := s.provider.Terminate(ctx, pi.ProviderID); terr != nil {
			return nil, fmt.Errorf("create instance: %w (provider cleanup failed: %v)", err, terr)
		}
		return nil, fmt.Errorf("create instance: %w", err)
	}
	return inst, nil
}

func (s *Service) List(ctx context.Context, orgID, userID int64) ([]models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, instanceID, userID int64) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}

	inst, err := s.repo.FindByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find instance: %w", err)
	}
	if inst.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	return inst, nil
}

func (s *Service) Start(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if !canTransition(inst.Status, models.InstanceStatusPending) {
		return fmt.Errorf("%w: cannot start from %q", ErrInvalidTransition, inst.Status)
	}
	if err := s.provider.Start(ctx, inst.ProviderID); err != nil {
		return fmt.Errorf("start instance: %w", err)
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusPending)
}

func (s *Service) Stop(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if !canTransition(inst.Status, models.InstanceStatusStopping) {
		return fmt.Errorf("%w: cannot stop from %q", ErrInvalidTransition, inst.Status)
	}
	if err := s.provider.Stop(ctx, inst.ProviderID); err != nil {
		return fmt.Errorf("stop instance: %w", err)
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusStopping)
}

func (s *Service) Terminate(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if inst.Status == models.InstanceStatusTerminated {
		return nil
	}
	if !canTransition(inst.Status, models.InstanceStatusTerminating) {
		return fmt.Errorf("%w: cannot terminate from %q", ErrInvalidTransition, inst.Status)
	}
	if err := s.provider.Terminate(ctx, inst.ProviderID); err != nil {
		return fmt.Errorf("terminate instance: %w", err)
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusTerminating)
}

func (s *Service) enforceQuota(ctx context.Context, orgID int64, cpu, mem, disk int) error {
	quota, err := s.quotas.Get(ctx, orgID)
	if err != nil {
		return fmt.Errorf("get quota: %w", err)
	}
	usage, err := s.repo.SumActiveByOrg(ctx, orgID)
	if err != nil {
		return fmt.Errorf("sum org usage: %w", err)
	}
	if usage.Count+1 > quota.MaxInstances {
		return fmt.Errorf("%w: instance count would exceed quota (%d)", ErrQuotaExceeded, quota.MaxInstances)
	}
	if usage.CPUCores+int64(cpu) > quota.MaxCPUCores {
		return fmt.Errorf("%w: CPU cores would exceed quota (%d)", ErrQuotaExceeded, quota.MaxCPUCores)
	}
	if usage.MemoryMB+int64(mem) > quota.MaxMemoryMB {
		return fmt.Errorf("%w: memory would exceed quota (%d MB)", ErrQuotaExceeded, quota.MaxMemoryMB)
	}
	if usage.DiskGB+int64(disk) > quota.MaxDiskGB {
		return fmt.Errorf("%w: disk would exceed quota (%d GB)", ErrQuotaExceeded, quota.MaxDiskGB)
	}
	return nil
}

func (s *Service) enforceCapacity(ctx context.Context, region string, capacity models.RegionCapacity, cpu, mem, disk int) error {
	usage, err := s.repo.SumActiveByRegion(ctx, region)
	if err != nil {
		return fmt.Errorf("sum region usage: %w", err)
	}
	if usage.CPUCores+int64(cpu) > capacity.CPUCores {
		return fmt.Errorf("%w: no CPU capacity in %s", ErrInsufficientCapacity, region)
	}
	if usage.MemoryMB+int64(mem) > capacity.MemoryMB {
		return fmt.Errorf("%w: no memory capacity in %s", ErrInsufficientCapacity, region)
	}
	if usage.DiskGB+int64(disk) > capacity.DiskGB {
		return fmt.Errorf("%w: no disk capacity in %s", ErrInsufficientCapacity, region)
	}
	return nil
}

func newProviderID() (string, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("i-%x", n), nil
}
