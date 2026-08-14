package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeAuditStore struct {
	events []models.AuditEvent
	listF  models.AuditFilter
}

func (f *fakeAuditStore) List(ctx context.Context, orgID int64, filter models.AuditFilter) ([]models.AuditEvent, error) {
	f.listF = filter
	return f.events, nil
}

func (f *fakeAuditStore) Count(ctx context.Context, orgID int64, filter models.AuditFilter) (int64, error) {
	return int64(len(f.events)), nil
}

type fakeMembers struct {
	member *models.OrgMember
	err    error
}

func (f *fakeMembers) FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.member == nil {
		return nil, database.ErrNotFound
	}
	return f.member, nil
}

func TestListScopesMembersToTheirOwnActions(t *testing.T) {
	store := &fakeAuditStore{}
	members := &fakeMembers{member: &models.OrgMember{Role: "member", UserID: 42}}
	svc := NewService(store, members)

	_, total, err := svc.List(context.Background(), 7, 42, Filter{Limit: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if store.listF.UserID == nil || *store.listF.UserID != 42 {
		t.Errorf("expected member scope on user 42, got %v", store.listF.UserID)
	}
	if total != 0 {
		t.Errorf("unexpected total %d", total)
	}
}

func TestListAdminsSeeWholeOrg(t *testing.T) {
	store := &fakeAuditStore{events: []models.AuditEvent{{ID: 1}, {ID: 2}}}
	members := &fakeMembers{member: &models.OrgMember{Role: "admin"}}
	svc := NewService(store, members)

	events, total, err := svc.List(context.Background(), 7, 1, Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if store.listF.UserID != nil {
		t.Errorf("expected no user scope for admin, got %v", *store.listF.UserID)
	}
	if total != 2 || len(events) != 2 {
		t.Errorf("expected 2 events, got total=%d len=%d", total, len(events))
	}
}

func TestListRejectsNonMember(t *testing.T) {
	store := &fakeAuditStore{}
	svc := NewService(store, &fakeMembers{})
	_, _, err := svc.List(context.Background(), 7, 1, Filter{})
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestListPropagatesStoreErrors(t *testing.T) {
	store := &fakeAuditStore{}
	members := &fakeMembers{err: context.DeadlineExceeded}
	svc := NewService(store, members)
	_, _, err := svc.List(context.Background(), 7, 1, Filter{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected underlying error to propagate, got %v", err)
	}
}
