package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrSlugTaken         = errors.New("slug already taken")
	ErrNotFound          = errors.New("organization not found")
	ErrNotMember         = errors.New("not a member of this organization")
	ErrUserAlreadyMember = errors.New("user is already a member")
)

type OrgStore interface {
	Create(ctx context.Context, org *models.Organization) error
	FindByID(ctx context.Context, id int64) (*models.Organization, error)
	FindBySlug(ctx context.Context, slug string) (*models.Organization, error)
	AddMember(ctx context.Context, member *models.OrgMember) error
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
	FindMembers(ctx context.Context, orgID int64, limit, offset int) ([]models.OrgMember, error)
	CountMembers(ctx context.Context, orgID int64) (int64, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.Organization, error)
	CountByUser(ctx context.Context, userID int64) (int64, error)
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

	existing, err := s.orgRepo.FindMember(ctx, orgID, targetUser.ID)
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
func (s *Service) ListMembers(ctx context.Context, orgID, userID int64, limit, offset int) ([]models.OrgMember, int64, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, 0, ErrNotMember
		}
		return nil, 0, fmt.Errorf("find member: %w", err)
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
