package organizations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeOrgStore struct {
	orgs         map[int64]*models.Organization
	bySlug       map[string]*models.Organization
	members      map[string]*models.OrgMember
	nextID       int64
	findErr      error
	createErr    error
	addMemberErr error
}

func newFakeOrgStore() *fakeOrgStore {
	return &fakeOrgStore{
		orgs:    map[int64]*models.Organization{},
		bySlug:  map[string]*models.Organization{},
		members: map[string]*models.OrgMember{},
	}
}

func memberKey(orgID, userID int64) string {
	return fmt.Sprintf("%d:%d", orgID, userID)
}

func (f *fakeOrgStore) Create(ctx context.Context, org *models.Organization) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	org.ID = f.nextID
	f.orgs[org.ID] = org
	f.bySlug[org.Slug] = org
	return nil
}

func (f *fakeOrgStore) FindByID(ctx context.Context, id int64) (*models.Organization, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if org, ok := f.orgs[id]; ok {
		return org, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) FindBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if org, ok := f.bySlug[slug]; ok {
		return org, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) AddMember(ctx context.Context, member *models.OrgMember) error {
	if f.addMemberErr != nil {
		return f.addMemberErr
	}
	f.nextID++
	member.ID = f.nextID
	f.members[memberKey(member.OrganizationID, member.UserID)] = member
	return nil
}

func (f *fakeOrgStore) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if m, ok := f.members[memberKey(orgID, userID)]; ok {
		return m, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) FindMembers(ctx context.Context, orgID int64) ([]models.OrgMember, error) {
	var out []models.OrgMember
	for _, m := range f.members {
		if m.OrganizationID == orgID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeOrgStore) ListByUser(ctx context.Context, userID int64) ([]models.Organization, error) {
	var out []models.Organization
	for _, m := range f.members {
		if m.UserID == userID {
			if org, ok := f.orgs[m.OrganizationID]; ok {
				out = append(out, *org)
			}
		}
	}
	return out, nil
}

type fakeUserStore struct {
	byEmail map[string]*models.User
	findErr error
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byEmail: map[string]*models.User{}}
}

func (f *fakeUserStore) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, database.ErrNotFound
}

func newTestService() (*Service, *fakeOrgStore, *fakeUserStore) {
	orgs := newFakeOrgStore()
	users := newFakeUserStore()
	return NewService(orgs, users), orgs, users
}

func TestService_Create_GeneratesSlugFromName(t *testing.T) {
	svc, orgs, _ := newTestService()

	org, err := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "My Cloud Org"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if org.Slug != "my-cloud-org" {
		t.Fatalf("expected slug %q, got %q", "my-cloud-org", org.Slug)
	}
	if org.ID == 0 {
		t.Fatal("expected org to be persisted with an ID")
	}
	if _, ok := orgs.members[memberKey(org.ID, 1)]; !ok {
		t.Fatal("expected creator to be added as a member")
	}
	if orgs.members[memberKey(org.ID, 1)].Role != "admin" {
		t.Fatal("expected creator role to be admin")
	}
}

func TestService_Create_RespectsProvidedSlug(t *testing.T) {
	svc, _, _ := newTestService()

	org, err := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "My Org", Slug: "custom-slug"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if org.Slug != "custom-slug" {
		t.Fatalf("expected slug %q, got %q", "custom-slug", org.Slug)
	}
}

func TestService_Create_SlugTaken(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Dup"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := svc.Create(context.Background(), 2, models.CreateOrgRequest{Name: "Dup"})
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("expected ErrSlugTaken, got %v", err)
	}
}

func TestService_Create_PropagatesDBError(t *testing.T) {
	svc, orgs, _ := newTestService()
	dbErr := errors.New("connection refused")
	orgs.findErr = dbErr

	_, err := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Org"})
	if err == nil || errors.Is(err, ErrSlugTaken) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_GetByID_NotAMember(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.GetByID(context.Background(), 1, 2)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestService_GetByID_PropagatesDBError(t *testing.T) {
	svc, orgs, _ := newTestService()
	dbErr := errors.New("connection refused")
	orgs.findErr = dbErr

	_, err := svc.GetByID(context.Background(), 1, 2)
	if err == nil || errors.Is(err, ErrNotMember) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_GetByID_Success(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	got, err := svc.GetByID(context.Background(), org.ID, 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Acme" {
		t.Fatalf("expected org Acme, got %q", got.Name)
	}
}

func TestService_InviteMember_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	_, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: "bob@example.com"})
	if err == nil || err.Error() != "only admins can invite members" {
		t.Fatalf("expected admin-only error, got %v", err)
	}
}

func TestService_InviteMember_UnknownTargetUser(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	_, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: "ghost@example.com"})
	if err == nil || err.Error() != "user not found" {
		t.Fatalf("expected user-not-found error, got %v", err)
	}
}

func TestService_InviteMember_AlreadyMember(t *testing.T) {
	svc, _, users := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	bob := &models.User{ID: 2, Email: "bob@example.com"}
	users.byEmail[bob.Email] = bob

	if _, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: bob.Email}); err != nil {
		t.Fatalf("first invite: %v", err)
	}

	_, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: bob.Email})
	if !errors.Is(err, ErrUserAlreadyMember) {
		t.Fatalf("expected ErrUserAlreadyMember, got %v", err)
	}
}

func TestService_InviteMember_Success(t *testing.T) {
	svc, _, users := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	bob := &models.User{ID: 2, Email: "bob@example.com"}
	users.byEmail[bob.Email] = bob

	member, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: bob.Email})
	if err != nil {
		t.Fatalf("InviteMember: %v", err)
	}
	if member.UserID != bob.ID || member.OrganizationID != org.ID {
		t.Fatalf("unexpected member: %+v", member)
	}
	if member.Role != "member" {
		t.Fatalf("expected default role member, got %q", member.Role)
	}
}

func TestService_InviteMember_PropagatesTargetLookupError(t *testing.T) {
	svc, _, users := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	users.findErr = errors.New("connection refused")

	_, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: "bob@example.com"})
	if err == nil || err.Error() == "user not found" {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_ListMembers_NotAMember(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.ListMembers(context.Background(), 1, 2)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestService_ListMembers_Success(t *testing.T) {
	svc, _, users := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	users.byEmail["bob@example.com"] = &models.User{ID: 2, Email: "bob@example.com"}
	if _, err := svc.InviteMember(context.Background(), org.ID, 1, models.InviteMemberRequest{Email: "bob@example.com"}); err != nil {
		t.Fatalf("invite: %v", err)
	}

	members, err := svc.ListMembers(context.Background(), org.ID, 1)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestService_List_ReturnsUserOrgs(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	orgs, err := svc.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != org.ID {
		t.Fatalf("unexpected orgs: %+v", orgs)
	}
}
