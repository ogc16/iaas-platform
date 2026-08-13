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
	instances     map[int64]*models.ComputeInstance
	statusUpdates []string
	nextID        int64
	findErr       error
	createErr     error
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

func (f *fakeInstanceStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	if inst, ok := f.instances[id]; ok {
		inst.Status = status
		f.statusUpdates = append(f.statusUpdates, status)
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

func newTestService() (*Service, *fakeInstanceStore, *fakeMembershipStore) {
	instances := newFakeInstanceStore()
	members := newFakeMembershipStore()
	return NewService(instances, members), instances, members
}

func TestService_Create_AppliesDefaults(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}

	inst, err := svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inst.InstanceType != models.InstanceTypeVM {
		t.Fatalf("expected default instance type vm, got %q", inst.InstanceType)
	}
	if inst.Region != "us-east" {
		t.Fatalf("expected default region us-east, got %q", inst.Region)
	}
	if inst.CPUCores != 1 || inst.MemoryMB != 1024 || inst.DiskGB != 10 {
		t.Fatalf("unexpected resource defaults: %+v", inst)
	}
	if inst.Status != models.InstanceStatusRunning {
		t.Fatalf("expected status running, got %q", inst.Status)
	}
	if !strings.HasPrefix(inst.IPAddress, "10.0.") {
		t.Fatalf("expected generated IP, got %q", inst.IPAddress)
	}
	if _, ok := instances.instances[inst.ID]; !ok {
		t.Fatal("expected instance to be persisted")
	}
}

func TestService_Create_PreservesProvidedResources(t *testing.T) {
	svc, _, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}

	inst, err := svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{
		Name:         "big-1",
		InstanceType: models.InstanceTypeContainer,
		Region:       "eu-west",
		CPUCores:     8,
		MemoryMB:     16384,
		DiskGB:       500,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inst.InstanceType != models.InstanceTypeContainer || inst.Region != "eu-west" ||
		inst.CPUCores != 8 || inst.MemoryMB != 16384 || inst.DiskGB != 500 {
		t.Fatalf("expected provided resources to be preserved: %+v", inst)
	}
}

func TestService_Create_NotInOrg(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_Create_PropagatesMembershipError(t *testing.T) {
	svc, _, members := newTestService()
	members.findErr = errors.New("connection refused")

	_, err := svc.Create(context.Background(), 1, 2, models.CreateInstanceRequest{Name: "web-1"})
	if err == nil || errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_List_NotInOrg(t *testing.T) {
	svc, _, _ := newTestService()

	if _, err := svc.List(context.Background(), 1, 2); !errors.Is(err, ErrNotInOrg) {
		t.Fatalf("expected ErrNotInOrg, got %v", err)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	svc, _, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}

	if _, err := svc.Get(context.Background(), 1, 999, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Get_CrossOrgHidden(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	instances.instances[10] = &models.ComputeInstance{ID: 10, OrganizationID: 99}

	if _, err := svc.Get(context.Background(), 1, 10, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected instance from another org to be hidden, got %v", err)
	}
}

func seedInstance(t *testing.T, svc *Service, orgID, userID int64) *models.ComputeInstance {
	t.Helper()
	inst, err := svc.Create(context.Background(), orgID, userID, models.CreateInstanceRequest{Name: "inst"})
	if err != nil {
		t.Fatalf("seed instance: %v", err)
	}
	inst.Status = models.InstanceStatusStopped
	return inst
}

func TestService_Start_TransitionsStoppedToRunning(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	inst := seedInstance(t, svc, 1, 2)

	if err := svc.Start(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if instances.instances[inst.ID].Status != models.InstanceStatusRunning {
		t.Fatalf("expected status running, got %q", instances.instances[inst.ID].Status)
	}
}

func TestService_Start_IsIdempotent(t *testing.T) {
	svc, _, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	inst := seedInstance(t, svc, 1, 2)
	inst.Status = models.InstanceStatusRunning

	if err := svc.Start(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Start on running instance should be a no-op: %v", err)
	}
}

func TestService_Start_TerminatedRejected(t *testing.T) {
	svc, _, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	inst := seedInstance(t, svc, 1, 2)
	inst.Status = models.InstanceStatusTerminated

	if err := svc.Start(context.Background(), 1, inst.ID, 2); err == nil {
		t.Fatal("expected starting a terminated instance to fail")
	}
}

func TestService_Stop_TransitionsRunningToStopped(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	inst := seedInstance(t, svc, 1, 2)
	inst.Status = models.InstanceStatusRunning

	if err := svc.Stop(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if instances.instances[inst.ID].Status != models.InstanceStatusStopped {
		t.Fatalf("expected status stopped, got %q", instances.instances[inst.ID].Status)
	}
}

func TestService_Terminate_TransitionsToTerminated(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	inst := seedInstance(t, svc, 1, 2)

	if err := svc.Terminate(context.Background(), 1, inst.ID, 2); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if instances.instances[inst.ID].Status != models.InstanceStatusTerminated {
		t.Fatalf("expected status terminated, got %q", instances.instances[inst.ID].Status)
	}
}

func TestService_Get_PropagatesRepoError(t *testing.T) {
	svc, instances, members := newTestService()
	members.members[key(1, 2)] = &models.OrgMember{OrganizationID: 1, UserID: 2, Role: "member"}
	instances.findErr = errors.New("connection refused")

	if _, err := svc.Get(context.Background(), 1, 10, 2); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}
