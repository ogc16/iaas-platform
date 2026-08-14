package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

// Action names recorded in the audit trail.
const (
	ActionOrgCreate         = "org.create"
	ActionMemberInvite      = "org.member.invite"
	ActionMemberRemove      = "org.member.remove"
	ActionMemberSuspend     = "org.member.suspend"
	ActionMemberUnsuspend   = "org.member.unsuspend"
	ActionJoinRequestAccept = "org.join_request.accept"
	ActionJoinRequestRevoke = "org.join_request.revoke"
	ActionInstanceCreate    = "instance.create"
	ActionInstanceStart     = "instance.start"
	ActionInstanceStop      = "instance.stop"
	ActionInstanceTerminate = "instance.terminate"
	ActionInstanceStatus    = "instance.status"
	ActionUsageRecord       = "billing.usage.record"
	ActionInvoiceGenerate   = "billing.invoice.generate"
)

var (
	ErrNotMember = errors.New("not a member of this organization")
)

// Filter is an alias for the shared model type, so callers can use either
// name.
type Filter = models.AuditFilter

// Store is the persistence boundary for the audit trail.
type Store interface {
	List(ctx context.Context, orgID int64, f models.AuditFilter) ([]models.AuditEvent, error)
	Count(ctx context.Context, orgID int64, f models.AuditFilter) (int64, error)
}

// MembershipStore resolves the caller's role so members only ever see their
// own actions while admins see the whole org.
type MembershipStore interface {
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
}

type Service struct {
	store   Store
	members MembershipStore
}

func NewService(store Store, members MembershipStore) *Service {
	return &Service{store: store, members: members}
}

// List returns one page of audit events and the total count. Admins see the
// whole organization; regular members are scoped to their own actions so they
// cannot spy on colleagues.
func (s *Service) List(ctx context.Context, orgID, userID int64, f Filter) ([]models.AuditEvent, int64, error) {
	member, err := s.members.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, 0, ErrNotMember
		}
		return nil, 0, fmt.Errorf("find member: %w", err)
	}
	if member.Role != "admin" {
		f.UserID = &userID
	}

	events, err := s.store.List(ctx, orgID, f)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit events: %w", err)
	}
	total, err := s.store.Count(ctx, orgID, f)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}
	return events, total, nil
}
