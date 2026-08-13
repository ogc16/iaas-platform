package compute

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrNotFound = errors.New("instance not found")
	ErrNotInOrg = errors.New("not a member of this organization")
)

type InstanceStore interface {
	Create(ctx context.Context, inst *models.ComputeInstance) error
	FindByID(ctx context.Context, id int64) (*models.ComputeInstance, error)
	ListByOrg(ctx context.Context, orgID int64) ([]models.ComputeInstance, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
}

type MembershipStore interface {
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
}

type Service struct {
	repo    InstanceStore
	orgRepo MembershipStore
}

func NewService(repo InstanceStore, orgRepo MembershipStore) *Service {
	return &Service{repo: repo, orgRepo: orgRepo}
}

func (s *Service) Create(ctx context.Context, orgID, userID int64, req models.CreateInstanceRequest) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}

	instanceType := req.InstanceType
	if instanceType == "" {
		instanceType = models.InstanceTypeVM
	}

	region := req.Region
	if region == "" {
		region = "us-east"
	}

	inst := &models.ComputeInstance{
		OrganizationID: orgID,
		UserID:         userID,
		Name:           req.Name,
		InstanceType:   instanceType,
		Status:         models.InstanceStatusRunning,
		Region:         region,
		CPUCores:       req.CPUCores,
		MemoryMB:       req.MemoryMB,
		DiskGB:         req.DiskGB,
		IPAddress:      fmt.Sprintf("10.0.%d.%d", rand.Intn(255), rand.Intn(255)),
	}

	if inst.CPUCores <= 0 {
		inst.CPUCores = 1
	}
	if inst.MemoryMB <= 0 {
		inst.MemoryMB = 1024
	}
	if inst.DiskGB <= 0 {
		inst.DiskGB = 10
	}

	if err := s.repo.Create(ctx, inst); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	return inst, nil
}

func (s *Service) List(ctx context.Context, orgID, userID int64) ([]models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, instanceID, userID int64) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotInOrg
		}
		return nil, fmt.Errorf("check membership: %w", err)
	}

	inst, err := s.repo.FindByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find instance: %w", err)
	}
	if inst.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	return inst, nil
}

func (s *Service) Start(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if inst.Status == models.InstanceStatusRunning {
		return nil
	}
	if inst.Status == models.InstanceStatusTerminated {
		return fmt.Errorf("cannot start a terminated instance")
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusRunning)
}

func (s *Service) Stop(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if inst.Status == models.InstanceStatusStopped {
		return nil
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusStopped)
}

func (s *Service) Terminate(ctx context.Context, orgID, instanceID, userID int64) error {
	inst, err := s.Get(ctx, orgID, instanceID, userID)
	if err != nil {
		return err
	}
	if inst.Status == models.InstanceStatusTerminated {
		return nil
	}
	return s.repo.UpdateStatus(ctx, instanceID, models.InstanceStatusTerminated)
}
