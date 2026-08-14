package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrSlugTaken         = errors.New("slug already taken")
	ErrNotFound          = errors.New("organization not found")
	ErrNotMember         = errors.New("not a member of this organization")
	ErrUserAlreadyMember = errors.New("user is already a member")
	ErrRequestNotFound   = errors.New("join request not found")
	ErrNotAdmin          = errors.New("only admins can manage members")
	ErrSelfAction        = errors.New("cannot modify your own membership")
)

type OrgStore interface {
	Create(ctx context.Context, org *models.Organization) error
	FindByID(ctx context.Context, id int64) (*models.Organization, error)
	FindBySlug(ctx context.Context, slug string) (*models.Organization, error)
	AddMember(ctx context.Context, member *models.OrgMember) error
	RemoveMember(ctx context.Context, orgID, userID int64) error
	SuspendMember(ctx context.Context, orgID, userID int64, until time.Time) error
	UnsuspendMember(ctx context.Context, orgID, userID int64) error
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
	FindMemberAny(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
	FindMembers(ctx context.Context, orgID int64, limit, offset int) ([]models.OrgMember, error)
	CountMembers(ctx context.Context, orgID int64) (int64, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.Organization, error)
	CountByUser(ctx context.Context, userID int64) (int64, error)
	AddJoinRequest(ctx context.Context, request *models.JoinRequest) error
	FindJoinRequest(ctx context.Context, orgID, userID int64) (*models.JoinRequest, error)
	FindJoinRequests(ctx context.Context, orgID int64, limit, offset int) ([]models.JoinRequest, error)
	CountJoinRequests(ctx context.Context, orgID int64) (int64, error)
	DeleteJoinRequest(ctx context.Context, orgID, userID int64) error
}

type UserStore interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
}

type Service struct {
	orgRepo  OrgStore
	userRepo UserStore
}

func NewService(orgRepo OrgStore, userRepo UserStore) *Service {
	return &Service{orgRepo: orgRepo, userRepo: userRepo}
}

func (s *Service) Create(ctx context.Context, userID int64, req models.CreateOrgRequest) (*models.Organization, error) {
	slug := strings.ToLower(strings.ReplaceAll(req.Slug, " ", "-"))
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	existing, err := s.orgRepo.FindBySlug(ctx, slug)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("check existing slug: %w", err)
	}
	if existing != nil {
		return nil, ErrSlugTaken
	}

	org := &models.Organization{Name: req.Name, Slug: slug}
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	member := &models.OrgMember{
		OrganizationID: org.ID,
		UserID:         userID,
		Role:           "admin",
	}
	if err := s.orgRepo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("add owner: %w", err)
	}

	return org, nil
}

func (s *Service) GetByID(ctx context.Context, orgID, userID int64) (*models.Organization, error) {
	member, err := s.orgRepo.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("find member: %w", err)
	}
	if member == nil {
		return nil, ErrNotMember
	}

	org, err := s.orgRepo.FindByID(ctx, orgID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find org: %w", err)
	}
	return org, nil
}

// List returns one page of the caller's organizations and the total count.
func (s *Service) List(ctx context.Context, userID int64, limit, offset int) ([]models.Organization, int64, error) {
	orgs, err := s.orgRepo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list orgs: %w", err)
	}
	total, err := s.orgRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count orgs: %w", err)
	}
	return orgs, total, nil
}

func (s *Service) InviteMember(ctx context.Context, orgID, userID int64, req models.InviteMemberRequest) (*models.OrgMember, error) {
	caller, err := s.orgRepo.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("only admins can invite members")
		}
		return nil, fmt.Errorf("find caller membership: %w", err)
	}
	if caller.Role != "admin" {
		return nil, fmt.Errorf("only admins can invite members")
	}

	targetUser, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	existing, err := s.orgRepo.FindMemberAny(ctx, orgID, targetUser.ID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("check existing member: %w", err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyMember
	}

	role := req.Role
	if role == "" {
		role = "member"
	}

	member := &models.OrgMember{
		OrganizationID: orgID,
		UserID:         targetUser.ID,
		Role:           role,
	}
	if err := s.orgRepo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	return member, nil
}

// ListMembers returns one page of an org's members and the total count.
// Admins see every member; regular members only ever see their own membership,
// so they cannot enumerate other members or their contact details.
func (s *Service) ListMembers(ctx context.Context, orgID, userID int64, limit, offset int) ([]models.OrgMember, int64, error) {
	member, err := s.orgRepo.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, 0, ErrNotMember
		}
		return nil, 0, fmt.Errorf("find member: %w", err)
	}

	if member.Role != "admin" {
		if offset > 0 {
			return []models.OrgMember{}, 1, nil
		}
		return []models.OrgMember{*member}, 1, nil
	}

	members, err := s.orgRepo.FindMembers(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list members: %w", err)
	}
	total, err := s.orgRepo.CountMembers(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count members: %w", err)
	}
	return members, total, nil
}

// requireAdmin returns the caller's membership only when they are an admin of
// the organization.
func (s *Service) requireAdmin(ctx context.Context, orgID, userID int64) (*models.OrgMember, error) {
	member, err := s.orgRepo.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotAdmin
		}
		return nil, fmt.Errorf("find caller membership: %w", err)
	}
	if member.Role != "admin" {
		return nil, ErrNotAdmin
	}
	return member, nil
}

// ListJoinRequests returns one page of pending join requests for an org and
// the total count. Admins only.
func (s *Service) ListJoinRequests(ctx context.Context, orgID, userID int64, limit, offset int) ([]models.JoinRequest, int64, error) {
	if _, err := s.requireAdmin(ctx, orgID, userID); err != nil {
		return nil, 0, err
	}

	requests, err := s.orgRepo.FindJoinRequests(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list join requests: %w", err)
	}
	total, err := s.orgRepo.CountJoinRequests(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count join requests: %w", err)
	}
	return requests, total, nil
}

// AcceptJoinRequest approves a pending join request, adding the user as a
// regular member. Admins only.
func (s *Service) AcceptJoinRequest(ctx context.Context, orgID, userID, adminID int64) (*models.OrgMember, error) {
	if _, err := s.requireAdmin(ctx, orgID, adminID); err != nil {
		return nil, err
	}

	if _, err := s.orgRepo.FindJoinRequest(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("find join request: %w", err)
	}

	if _, err := s.orgRepo.FindMemberAny(ctx, orgID, userID); err == nil {
		// Already a member; drop the stale request and report it.
		if err := s.orgRepo.DeleteJoinRequest(ctx, orgID, userID); err != nil && !errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("delete join request: %w", err)
		}
		return nil, ErrUserAlreadyMember
	} else if !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("check existing member: %w", err)
	}

	member := &models.OrgMember{OrganizationID: orgID, UserID: userID, Role: "member"}
	if err := s.orgRepo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	if err := s.orgRepo.DeleteJoinRequest(ctx, orgID, userID); err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("delete join request: %w", err)
	}
	return member, nil
}

// RevokeJoinRequest denies a pending join request. Admins only.
func (s *Service) RevokeJoinRequest(ctx context.Context, orgID, userID, adminID int64) error {
	if _, err := s.requireAdmin(ctx, orgID, adminID); err != nil {
		return err
	}

	if err := s.orgRepo.DeleteJoinRequest(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrRequestNotFound
		}
		return fmt.Errorf("delete join request: %w", err)
	}
	return nil
}

// RemoveMember revokes a member's access to an organization, and clears any
// pending join request they may still have. Admins only; an admin cannot
// revoke their own membership.
func (s *Service) RemoveMember(ctx context.Context, orgID, memberUserID, adminID int64) error {
	if _, err := s.requireAdmin(ctx, orgID, adminID); err != nil {
		return err
	}
	if memberUserID == adminID {
		return ErrSelfAction
	}

	if err := s.orgRepo.DeleteJoinRequest(ctx, orgID, memberUserID); err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("delete join request: %w", err)
	}
	if err := s.orgRepo.RemoveMember(ctx, orgID, memberUserID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrNotMember
		}
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// SuspendMember temporarily revokes a member's access for the given number of
// days. The member's rights are automatically restored once the period
// elapses (the underlying store treats an expired suspension as active). An
// admin can revoke access early with UnsuspendMember. Admins only; an admin
// cannot suspend themselves.
func (s *Service) SuspendMember(ctx context.Context, orgID, memberUserID int64, days int, adminID int64) (*models.OrgMember, error) {
	if _, err := s.requireAdmin(ctx, orgID, adminID); err != nil {
		return nil, err
	}
	if memberUserID == adminID {
		return nil, ErrSelfAction
	}

	until := time.Now().UTC().AddDate(0, 0, days)
	if err := s.orgRepo.SuspendMember(ctx, orgID, memberUserID, until); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("suspend member: %w", err)
	}
	member, err := s.orgRepo.FindMemberAny(ctx, orgID, memberUserID)
	if err != nil {
		return nil, fmt.Errorf("find member: %w", err)
	}
	return member, nil
}

// UnsuspendMember immediately restores a member whose access was suspended.
// Admins only.
func (s *Service) UnsuspendMember(ctx context.Context, orgID, memberUserID, adminID int64) (*models.OrgMember, error) {
	if _, err := s.requireAdmin(ctx, orgID, adminID); err != nil {
		return nil, err
	}

	if err := s.orgRepo.UnsuspendMember(ctx, orgID, memberUserID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("unsuspend member: %w", err)
	}
	member, err := s.orgRepo.FindMemberAny(ctx, orgID, memberUserID)
	if err != nil {
		return nil, fmt.Errorf("find member: %w", err)
	}
	return member, nil
}
