package organizations

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeOrgStore struct {
	orgs             map[int64]*models.Organization
	bySlug           map[string]*models.Organization
	members          map[string]*models.OrgMember
	joinRequests     map[string]*models.JoinRequest
	nextID           int64
	findErr          error
	createErr        error
	addMemberErr     error
	joinRequestErr   error
	deleteRequestErr error
}

func newFakeOrgStore() *fakeOrgStore {
	return &fakeOrgStore{
		orgs:         map[int64]*models.Organization{},
		bySlug:       map[string]*models.Organization{},
		members:      map[string]*models.OrgMember{},
		joinRequests: map[string]*models.JoinRequest{},
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
		if m.IsSuspended(time.Now()) {
			return nil, database.ErrNotFound
		}
		return m, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) FindMemberAny(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if m, ok := f.members[memberKey(orgID, userID)]; ok {
		return m, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) SuspendMember(ctx context.Context, orgID, userID int64, until time.Time) error {
	m, ok := f.members[memberKey(orgID, userID)]
	if !ok {
		return database.ErrNotFound
	}
	u := until
	m.SuspendedUntil = &u
	return nil
}

func (f *fakeOrgStore) UnsuspendMember(ctx context.Context, orgID, userID int64) error {
	m, ok := f.members[memberKey(orgID, userID)]
	if !ok {
		return database.ErrNotFound
	}
	m.SuspendedUntil = nil
	return nil
}

func (f *fakeOrgStore) RemoveMember(ctx context.Context, orgID, userID int64) error {
	if m, ok := f.members[memberKey(orgID, userID)]; ok {
		delete(f.members, memberKey(orgID, userID))
		_ = m
		return nil
	}
	return database.ErrNotFound
}

func (f *fakeOrgStore) AddJoinRequest(ctx context.Context, request *models.JoinRequest) error {
	if f.joinRequestErr != nil {
		return f.joinRequestErr
	}
	f.nextID++
	request.ID = f.nextID
	f.joinRequests[memberKey(request.OrganizationID, request.UserID)] = request
	return nil
}

func (f *fakeOrgStore) FindJoinRequest(ctx context.Context, orgID, userID int64) (*models.JoinRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if r, ok := f.joinRequests[memberKey(orgID, userID)]; ok {
		return r, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) FindJoinRequests(ctx context.Context, orgID int64, limit, offset int) ([]models.JoinRequest, error) {
	var out []models.JoinRequest
	for _, r := range f.joinRequests {
		if r.OrganizationID == orgID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeOrgStore) CountJoinRequests(ctx context.Context, orgID int64) (int64, error) {
	var n int64
	for _, r := range f.joinRequests {
		if r.OrganizationID == orgID {
			n++
		}
	}
	return n, nil
}

func (f *fakeOrgStore) DeleteJoinRequest(ctx context.Context, orgID, userID int64) error {
	if f.deleteRequestErr != nil {
		return f.deleteRequestErr
	}
	if _, ok := f.joinRequests[memberKey(orgID, userID)]; ok {
		delete(f.joinRequests, memberKey(orgID, userID))
		return nil
	}
	return database.ErrNotFound
}

func (f *fakeOrgStore) FindMembers(ctx context.Context, orgID int64, limit, offset int) ([]models.OrgMember, error) {
	var out []models.OrgMember
	for _, m := range f.members {
		if m.OrganizationID == orgID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeOrgStore) CountMembers(ctx context.Context, orgID int64) (int64, error) {
	var n int64
	for _, m := range f.members {
		if m.OrganizationID == orgID {
			n++
		}
	}
	return n, nil
}

func (f *fakeOrgStore) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.Organization, error) {
	var out []models.Organization
	for _, m := range f.members {
		if m.UserID == userID && !m.IsSuspended(time.Now()) {
			if org, ok := f.orgs[m.OrganizationID]; ok {
				out = append(out, *org)
			}
		}
	}
	return out, nil
}

func (f *fakeOrgStore) CountByUser(ctx context.Context, userID int64) (int64, error) {
	var n int64
	for _, m := range f.members {
		if m.UserID == userID && !m.IsSuspended(time.Now()) {
			if _, ok := f.orgs[m.OrganizationID]; ok {
				n++
			}
		}
	}
	return n, nil
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

	if _, _, err := svc.ListMembers(context.Background(), 1, 2, 50, 0); !errors.Is(err, ErrNotMember) {
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

	members, total, err := svc.ListMembers(context.Background(), org.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 || total != 2 {
		t.Fatalf("expected 2 members with total 2, got %d members, total %d", len(members), total)
	}
}

func TestService_List_ReturnsUserOrgs(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	orgs, total, err := svc.List(context.Background(), 1, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != org.ID || total != 1 {
		t.Fatalf("unexpected orgs: %+v (total %d)", orgs, total)
	}
}

func TestService_ListJoinRequests_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	if _, _, err := svc.ListJoinRequests(context.Background(), org.ID, 1, 50, 0); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestService_ListJoinRequests_Success(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.joinRequests[memberKey(org.ID, 2)] = &models.JoinRequest{ID: 1, OrganizationID: org.ID, UserID: 2, Email: "bob@example.com"}

	requests, total, err := svc.ListJoinRequests(context.Background(), org.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("ListJoinRequests: %v", err)
	}
	if len(requests) != 1 || total != 1 || requests[0].Email != "bob@example.com" {
		t.Fatalf("unexpected requests: %+v (total %d)", requests, total)
	}
}

func TestService_AcceptJoinRequest_AddsMember(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.joinRequests[memberKey(org.ID, 2)] = &models.JoinRequest{ID: 1, OrganizationID: org.ID, UserID: 2}

	member, err := svc.AcceptJoinRequest(context.Background(), org.ID, 2, 1)
	if err != nil {
		t.Fatalf("AcceptJoinRequest: %v", err)
	}
	if member.OrganizationID != org.ID || member.UserID != 2 || member.Role != "member" {
		t.Fatalf("unexpected member: %+v", member)
	}
	if _, ok := orgs.members[memberKey(org.ID, 2)]; !ok {
		t.Fatal("expected user to be added as a member")
	}
	if _, ok := orgs.joinRequests[memberKey(org.ID, 2)]; ok {
		t.Fatal("expected join request to be removed after acceptance")
	}
}

func TestService_AcceptJoinRequest_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	if _, err := svc.AcceptJoinRequest(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestService_AcceptJoinRequest_NoRequest(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if _, err := svc.AcceptJoinRequest(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got %v", err)
	}
}

func TestService_AcceptJoinRequest_AlreadyMember(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "member"}
	orgs.joinRequests[memberKey(org.ID, 2)] = &models.JoinRequest{ID: 1, OrganizationID: org.ID, UserID: 2}

	_, err := svc.AcceptJoinRequest(context.Background(), org.ID, 2, 1)
	if !errors.Is(err, ErrUserAlreadyMember) {
		t.Fatalf("expected ErrUserAlreadyMember, got %v", err)
	}
	if _, ok := orgs.joinRequests[memberKey(org.ID, 2)]; ok {
		t.Fatal("expected stale join request to be cleaned up")
	}
}

func TestService_RevokeJoinRequest_Success(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.joinRequests[memberKey(org.ID, 2)] = &models.JoinRequest{ID: 1, OrganizationID: org.ID, UserID: 2}

	if err := svc.RevokeJoinRequest(context.Background(), org.ID, 2, 1); err != nil {
		t.Fatalf("RevokeJoinRequest: %v", err)
	}
	if _, ok := orgs.joinRequests[memberKey(org.ID, 2)]; ok {
		t.Fatal("expected join request to be removed")
	}
}

func TestService_RevokeJoinRequest_NoRequest(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if err := svc.RevokeJoinRequest(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got %v", err)
	}
}

func TestService_RemoveMember_Success(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "member"}

	if err := svc.RemoveMember(context.Background(), org.ID, 2, 1); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, ok := orgs.members[memberKey(org.ID, 2)]; ok {
		t.Fatal("expected member to be removed")
	}
}

func TestService_RemoveMember_SelfRejected(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	err := svc.RemoveMember(context.Background(), org.ID, 1, 1)
	if !errors.Is(err, ErrSelfAction) {
		t.Fatalf("expected ErrSelfAction, got %v", err)
	}
}

func TestService_RemoveMember_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	if err := svc.RemoveMember(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestService_RemoveMember_NotAMember(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if err := svc.RemoveMember(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestService_SuspendMember_RevokesAccessForDuration(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "member"}

	before := time.Now().UTC()
	member, err := svc.SuspendMember(context.Background(), org.ID, 2, 30, 1)
	if err != nil {
		t.Fatalf("SuspendMember: %v", err)
	}
	if member.SuspendedUntil == nil {
		t.Fatal("expected suspended_until to be set")
	}
	expected := before.AddDate(0, 0, 30)
	if member.SuspendedUntil.Before(expected.Add(-time.Minute)) || member.SuspendedUntil.After(expected.Add(time.Minute)) {
		t.Fatalf("expected suspension ~%v, got %v", expected, *member.SuspendedUntil)
	}

	if _, err := svc.GetByID(context.Background(), org.ID, 2); !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected suspended member to lose access, got %v", err)
	}
	orgsList, total, err := svc.List(context.Background(), 2, 50, 0)
	if err != nil || len(orgsList) != 0 || total != 0 {
		t.Fatalf("expected suspended user's org list to be empty, got %+v (total %d, err %v)", orgsList, total, err)
	}
}

func TestService_SuspendMember_AutoRestoresAfterExpiry(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	expired := time.Now().UTC().Add(-time.Hour)
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "member", SuspendedUntil: &expired}

	if _, err := svc.GetByID(context.Background(), org.ID, 2); err != nil {
		t.Fatalf("expected access to be restored after suspension expiry, got %v", err)
	}
	orgsList, total, err := svc.List(context.Background(), 2, 50, 0)
	if err != nil || len(orgsList) != 1 || total != 1 {
		t.Fatalf("expected org to reappear after expiry, got %+v (total %d, err %v)", orgsList, total, err)
	}
}

func TestService_SuspendMember_SelfRejected(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if _, err := svc.SuspendMember(context.Background(), org.ID, 1, 30, 1); !errors.Is(err, ErrSelfAction) {
		t.Fatalf("expected ErrSelfAction, got %v", err)
	}
}

func TestService_SuspendMember_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	if _, err := svc.SuspendMember(context.Background(), org.ID, 2, 30, 1); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestService_SuspendMember_SuspendedAdminLosesAdminPowers(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	until := time.Now().UTC().Add(24 * time.Hour)
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "admin", SuspendedUntil: &until}

	if _, err := svc.SuspendMember(context.Background(), org.ID, 2, 30, 2); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected suspended admin to be treated as non-admin, got %v", err)
	}
}

func TestService_SuspendMember_NotAMember(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if _, err := svc.SuspendMember(context.Background(), org.ID, 2, 30, 1); !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestService_UnsuspendMember_RestoresAccess(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	until := time.Now().UTC().Add(30 * 24 * time.Hour)
	orgs.members[memberKey(org.ID, 2)] = &models.OrgMember{OrganizationID: org.ID, UserID: 2, Role: "member", SuspendedUntil: &until}

	member, err := svc.UnsuspendMember(context.Background(), org.ID, 2, 1)
	if err != nil {
		t.Fatalf("UnsuspendMember: %v", err)
	}
	if member.SuspendedUntil != nil {
		t.Fatalf("expected suspension to be cleared, got %v", *member.SuspendedUntil)
	}
	if _, err := svc.GetByID(context.Background(), org.ID, 2); err != nil {
		t.Fatalf("expected access after unsuspend, got %v", err)
	}
}

func TestService_UnsuspendMember_NonAdminRejected(t *testing.T) {
	svc, orgs, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})
	orgs.members[memberKey(org.ID, 1)].Role = "member"

	if _, err := svc.UnsuspendMember(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("expected ErrNotAdmin, got %v", err)
	}
}

func TestService_UnsuspendMember_NotAMember(t *testing.T) {
	svc, _, _ := newTestService()
	org, _ := svc.Create(context.Background(), 1, models.CreateOrgRequest{Name: "Acme"})

	if _, err := svc.UnsuspendMember(context.Background(), org.ID, 2, 1); !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}
