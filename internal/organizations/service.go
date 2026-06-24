package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/user/iaas-platform/internal/database"
	"github.com/user/iaas-platform/internal/models"
)

var (
	ErrSlugTaken   = errors.New("slug already taken")
	ErrNotFound    = errors.New("organization not found")
	ErrNotMember   = errors.New("not a member of this organization")
	ErrUserAlreadyMember = errors.New("user is already a member")
)

type Service struct {
	orgRepo  *database.OrgRepository
	userRepo *database.UserRepository
}

func NewService(orgRepo *database.OrgRepository, userRepo *database.UserRepository) *Service {
	return &Service{orgRepo: orgRepo, userRepo: userRepo}
}

func (s *Service) Create(ctx context.Context, userID int64, req models.CreateOrgRequest) (*models.Organization, error) {
	slug := strings.ToLower(strings.ReplaceAll(req.Slug, " ", "-"))
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	existing, _ := s.orgRepo.FindBySlug(ctx, slug)
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
		return nil, ErrNotMember
	}
	if member == nil {
		return nil, ErrNotMember
	}

	org, err := s.orgRepo.FindByID(ctx, orgID)
	if err != nil {
		return nil, ErrNotFound
	}
	return org, nil
}

func (s *Service) List(ctx context.Context, userID int64) ([]models.Organization, error) {
	return s.orgRepo.ListByUser(ctx, userID)
}

func (s *Service) InviteMember(ctx context.Context, orgID, userID int64, req models.InviteMemberRequest) (*models.OrgMember, error) {
	caller, err := s.orgRepo.FindMember(ctx, orgID, userID)
	if err != nil || caller.Role != "admin" {
		return nil, fmt.Errorf("only admins can invite members")
	}

	targetUser, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	existing, _ := s.orgRepo.FindMember(ctx, orgID, targetUser.ID)
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
		return nil, ErrNotMember
	}
	return s.orgRepo.FindMembers(ctx, orgID)
}
