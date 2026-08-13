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
	FindMembers(ctx context.Context, orgID int64) ([]models.OrgMember, error)
	ListByUser(ctx context.Context, userID int64) ([]models.Organization, error)
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

func (s *Service) List(ctx context.Context, userID int64) ([]models.Organization, error) {
	return s.orgRepo.ListByUser(ctx, userID)
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

func (s *Service) ListMembers(ctx context.Context, orgID, userID int64) ([]models.OrgMember, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("find member: %w", err)
	}
	return s.orgRepo.FindMembers(ctx, orgID)
}
