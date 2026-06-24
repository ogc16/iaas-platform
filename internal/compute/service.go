package compute

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/user/iaas-platform/internal/database"
	"github.com/user/iaas-platform/internal/models"
)

var (
	ErrNotFound = errors.New("instance not found")
	ErrNotInOrg = errors.New("not a member of this organization")
)

type Service struct {
	repo    *database.ComputeRepository
	orgRepo *database.OrgRepository
}

func NewService(repo *database.ComputeRepository, orgRepo *database.OrgRepository) *Service {
	return &Service{repo: repo, orgRepo: orgRepo}
}

func (s *Service) Create(ctx context.Context, orgID, userID int64, req models.CreateInstanceRequest) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		return nil, ErrNotInOrg
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
		return nil, ErrNotInOrg
	}
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, instanceID, userID int64) (*models.ComputeInstance, error) {
	if _, err := s.orgRepo.FindMember(ctx, orgID, userID); err != nil {
		return nil, ErrNotInOrg
	}

	inst, err := s.repo.FindByID(ctx, instanceID)
	if err != nil {
		return nil, ErrNotFound
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
