package compute

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeInstanceStore struct {
	instances map[int64]*models.ComputeInstance
	nextID    int64
	findErr   error
	createErr error
	listErr   error
}

func newFakeInstanceStore() *fakeInstanceStore {
	return &fakeInstanceStore{instances: map[int64]*models.ComputeInstance{}}
}

func (f *fakeInstanceStore) Create(ctx context.Context, inst *models.ComputeInstance) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	inst.ID = f.nextID
	f.instances[inst.ID] = inst
	return nil
}

func (f *fakeInstanceStore) FindByID(ctx context.Context, id int64) (*models.ComputeInstance, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if inst, ok := f.instances[id]; ok {
		return inst, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeInstanceStore) ListByOrg(ctx context.Context, orgID int64) ([]models.ComputeInstance, error) {
	var out []models.ComputeInstance
	for _, inst := range f.instances {
		if inst.OrganizationID == orgID {
			out = append(out, *inst)
		}
	}
	return out, nil
}

func (f *fakeInstanceStore) ListActive(ctx context.Context) ([]models.ComputeInstance, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []models.ComputeInstance
	for _, inst := range f.instances {
		if inst.Status != models.InstanceStatusTerminated {
			out = append(out, *inst)
		}
	}
	return out, nil
}

func (f *fakeInstanceStore) SumActiveByOrg(ctx context.Context, orgID int64) (models.OrgUsage, error) {
	var u models.OrgUsage
	for _, inst := range f.instances {
		if inst.OrganizationID == orgID && inst.Status != models.InstanceStatusTerminated {
			u.Count++
			u.CPUCores += int64(inst.CPUCores)
			u.MemoryMB += int64(inst.MemoryMB)
			u.DiskGB += int64(inst.DiskGB)
		}
	}
	return u, nil
}

func (f *fakeInstanceStore) SumActiveByRegion(ctx context.Context, region string) (models.RegionUsage, error) {
	var u models.RegionUsage
	for _, inst := range f.instances {
		if inst.Region == region && inst.Status != models.InstanceStatusTerminated {
			u.CPUCores += int64(inst.CPUCores)
			u.MemoryMB += int64(inst.MemoryMB)
			u.DiskGB += int64(inst.DiskGB)
		}
	}
	return u, nil
}

func (f *fakeInstanceStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	if inst, ok := f.instances[id]; ok {
		inst.Status = status
	}
	return nil
}

type fakeMembershipStore struct {
	members map[string]*models.OrgMember
	findErr error
}

func newFakeMembershipStore() *fakeMembershipStore {
	return &fakeMembershipStore{members: map[string]*models.OrgMember{}}
}

func (f *fakeMembershipStore) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if _, ok := f.members[key(orgID, userID)]; ok {
		return &models.OrgMember{OrganizationID: orgID, UserID: userID, Role: "member"}, nil
	}
	return nil, database.ErrNotFound
}

func key(orgID, userID int64) string {
	return fmt.Sprintf("%d:%d", orgID, userID)
}

type fakeQuotaStore struct {
	quota  models.Quota
	getErr error
}

func (f *fakeQuotaStore) Get(ctx context.Context, orgID int64) (models.Quota, error) {
	if f.getErr != nil {
		return models.Quota{}, f.getErr
	}
	q := f.quota
	if q.MaxInstances == 0 {
		q = models.DefaultQuota
	}
	q.OrganizationID = orgID
	return q, nil
}

type fakeCapacityStore struct {
	regions map[string]models.RegionCapacity
	getErr  error
}

func newFakeCapacityStore() *fakeCapacityStore {
	return &fakeCapacityStore{regions: map[string]models.RegionCapacity{
		"us-east-1": {Region: "us-east-1", CPUCores: 64, MemoryMB: 131072, DiskGB: 2000},
		"eu-west-1": {Region: "eu-west-1", CPUCores: 32, MemoryMB: 65536, DiskGB: 1000},
	}}
}

func (f *fakeCapacityStore) GetRegion(ctx context.Context, region string) (models.RegionCapacity, error) {
	if f.getErr != nil {
		return models.RegionCapacity{}, f.getErr
	}
	if c, ok := f.regions[region]; ok {
		return c, nil
	}
	return models.RegionCapacity{}, database.ErrNotFound
}

type fakeProvider struct {
	states       map[string]string
	provisionErr error
	startErr     error
	stopErr      error
	terminateErr error
	provisions   int
	lastSpec     ProviderSpec
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{states: map[string]string{}}
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Provision(ctx context.Context, spec ProviderSpec) (*ProviderInstance, error) {
	if f.provisionErr != nil {
		return nil, f.provisionErr
	}
	f.provisions++
	f.lastSpec = spec
	f.states[spec.ProviderID] = models.InstanceStatusPending
	return &ProviderInstance{ProviderID: spec.ProviderID, State: models.InstanceStatusPending}, nil
}

func (f *fakeProvider) Start(ctx context.Context, providerID string) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.states[providerID] = models.InstanceStatusPending
	return nil
}

func (f *fakeProvider) Stop(ctx context.Context, providerID string) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.states[providerID] = models.InstanceStatusStopping
	return nil
}

func (f *fakeProvider) Terminate(ctx context.Context, providerID string) error {
	if f.terminateErr != nil {
		return f.terminateErr
	}
	f.states[providerID] = models.InstanceStatusTerminated
	return nil
}

func (f *fakeProvider) GetState(ctx context.Context, providerID string) (string, error) {
	if s, ok := f.states[providerID]; ok {
		return s, nil
	}
	return "", ErrProviderStateNotFound
}

type testEnv struct {
	svc       *Service
	instances *fakeInstanceStore
	members   *fakeMembershipStore
	quotas    *fakeQuotaStore
	capacity  *fakeCapacityStore
	provider  *fakeProvider
}

func newTestEnv() *testEnv {
	instances := newFakeInstanceStore()
	members := newFakeMembershipStore()
	quotas := &fakeQuotaStore{}
	capacity := newFakeCapacityStore()
	provider := newFakeProvider()
	svc := NewService(instances, members, provider, quotas, capacity)
	return &testEnv{svc: svc, instances: instances, members: members, quotas: quotas, capacity: capacity, provider: provider}
}

func (e *testEnv) addMember(orgID, userID int64) {
	e.members.members[key(orgID, userID)] = &models.OrgMember{OrganizationID: orgID, UserID: userID, Role: "member"}
}

func seedInstance(t *testing.T, e *testEnv, orgID, userID int64, status string) *models.ComputeInstance {
	t.Helper()
	inst := &models.ComputeInstance{
		ID:             1,
		OrganizationID: orgID,
		UserID:         userID,
		Name:           "inst",
		Status:         status,
		Region:         "us-east-1",
		ProviderID:     "i-seed",
		CPUCores:       1,
		MemoryMB:       1024,
		DiskGB:         10,
	}
	e.instances.instances[inst.ID] = inst
	if e.provider.states == nil {
		e.provider.states = map[string]string{}
	}
	e.provider.states[inst.ProviderID] = status
	return inst
}

func TestService_Create_AppliesDefaults(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)

	inst, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inst.InstanceType != models.InstanceTypeVM {
		t.Fatalf("expected default instance type vm, got %q", inst.InstanceType)
	}
	if inst.Region != "us-east-1" {
		t.Fatalf("expected default region us-east-1, got %q", inst.Region)
	}
	if inst.Image != "debian-12" {
		t.Fatalf("expected default image debian-12, got %q", inst.Image)
	}
	if inst.CPUCores != 1 || inst.MemoryMB != 1024 || inst.DiskGB != 10 {
		t.Fatalf("unexpected resource defaults: %+v", inst)
	}
	if inst.Status != models.InstanceStatusPending {
		t.Fatalf("expected status pending, got %q", inst.Status)
	}
	if !strings.HasPrefix(inst.ProviderID, "i-") {
		t.Fatalf("expected a provider id, got %q", inst.ProviderID)
	}
	if !strings.HasPrefix(inst.IPAddress, "10.0.") {
		t.Fatalf("expected generated IP, got %q", inst.IPAddress)
	}
	if _, ok := e.instances.instances[inst.ID]; !ok {
		t.Fatal("expected instance to be persisted")
	}
	if e.provider.provisions != 1 {
		t.Fatalf("expected provider to be called once, got %d", e.provider.provisions)
	}
}

func TestService_Create_PreservesProvidedResources(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)

	inst, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{
		Name:         "big-1",
		InstanceType: models.InstanceTypeContainer,
		Region:       "eu-west-1",
		Image:        "alpine:3.20",
		Port:         8080,
		CPUCores:     8,
		MemoryMB:     16384,
		DiskGB:       500,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inst.InstanceType != models.InstanceTypeContainer || inst.Region != "eu-west-1" ||
		inst.Image != "alpine:3.20" || inst.Port != 8080 ||
		inst.CPUCores != 8 || inst.MemoryMB != 16384 || inst.DiskGB != 500 {
		t.Fatalf("expected provided resources to be preserved: %+v", inst)
	}
}

func TestService_Create_NotInOrg(t *testing.T) {
	e := newTestEnv()

	_, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_Create_PropagatesMembershipError(t *testing.T) {
	e := newTestEnv()
	e.members.findErr = errors.New("connection refused")

	_, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err == nil || errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Create_UnknownRegion(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)

	_, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{
		Name:   "web-1",
		Region: "ap-southeast-2",
	})
	if !errors.Is(err, ErrUnknownRegion) {
		t.Fatalf("expected ErrUnknownRegion, got %v", err)
	}
}

func TestService_Create_QuotaExceeded(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.quotas.quota = models.Quota{MaxInstances: 1, MaxCPUCores: 4, MaxMemoryMB: 4096, MaxDiskGB: 100}

	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "a"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "b"}); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded on second create, got %v", err)
	}
}

func TestService_Create_QuotaExceededByCPU(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.quotas.quota = models.Quota{MaxInstances: 10, MaxCPUCores: 4, MaxMemoryMB: 65536, MaxDiskGB: 1000}

	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "big", CPUCores: 8}); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestService_Create_PropagatesQuotaStoreError(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.quotas.getErr = errors.New("connection refused")

	_, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err == nil || errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Create_InsufficientCapacity(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.capacity.regions["us-east-1"] = models.RegionCapacity{Region: "us-east-1", CPUCores: 2, MemoryMB: 4096, DiskGB: 50}

	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "big", CPUCores: 8}); !errors.Is(err, ErrInsufficientCapacity) {
		t.Fatalf("expected ErrInsufficientCapacity, got %v", err)
	}
}

func TestService_Create_PropagatesCapacityStoreError(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.capacity.getErr = errors.New("connection refused")

	_, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err == nil || errors.Is(err, ErrUnknownRegion) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Create_PropagatesProvisionError(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.provider.provisionErr = errors.New("provider down")

	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"}); err == nil {
		t.Fatal("expected provision error to propagate")
	}
}

func TestService_Create_PropagatesRepoError(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.instances.createErr = errors.New("connection refused")

	if _, err := e.svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"}); err == nil {
		t.Fatal("expected repo error to propagate")
	}
}

func TestService_List_NotInOrg(t *testing.T) {
	e := newTestEnv()

	if _, err := e.svc.List(context.Background(), 1, 2); !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)

	if _, err := e.svc.Get(context.Background(), 1, 999, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Get_CrossOrgHidden(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.instances.instances[10] = &models.ComputeInstance{ID: 10, OrganizationID: 99}

	if _, err := e.svc.Get(context.Background(), 1, 10, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected instance from another org to be hidden, got %v", err)
	}
}

func TestService_Get_PropagatesRepoError(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	e.instances.findErr = errors.New("connection refused")

	if _, err := e.svc.Get(context.Background(), 1, 10, 2); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Start_FromStoppedRequestsPending(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusStopped)

	if err := e.svc.Start(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusPending {
		t.Fatalf("expected status pending, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestService_Start_FromRunningRejected(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusRunning)

	if err := e.svc.Start(context.Background(), 1, inst.ID, 2); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestService_Start_FromTerminatedRejected(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusTerminated)

	if err := e.svc.Start(context.Background(), 1, inst.ID, 2); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestService_Stop_FromRunningRequestsStopping(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusRunning)

	if err := e.svc.Stop(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusStopping {
		t.Fatalf("expected status stopping, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestService_Stop_FromStoppedRejected(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusStopped)

	if err := e.svc.Stop(context.Background(), 1, inst.ID, 2); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestService_Terminate_RequestsTerminating(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusRunning)

	if err := e.svc.Terminate(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusTerminating {
		t.Fatalf("expected status terminating, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestService_Terminate_FromTerminatedIsNoOp(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusTerminated)

	if err := e.svc.Terminate(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Terminate on terminated instance should be a no-op: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusTerminated {
		t.Fatalf("expected status to remain terminated, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestService_Terminate_FromFailedAllowed(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusFailed)

	if err := e.svc.Terminate(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Terminate from failed should allow cleanup: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusTerminating {
		t.Fatalf("expected status terminating, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestService_Terminate_FromPendingAllowed(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusPending)

	if err := e.svc.Terminate(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Terminate from pending: %v", err)
	}
}
