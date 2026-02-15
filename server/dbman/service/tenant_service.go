package service

import (
	"context"
	"errors"
	"strings"

	"msg_server/server/chat/domain"
	"msg_server/server/dbman/repository"
)

type TenantService struct {
	repo    *repository.TenantRepository
	routers []tenantInvalidator
}

type tenantInvalidator interface {
	InvalidateTenant(tenantID string)
}

func NewTenantService(repo *repository.TenantRepository, invalidators ...tenantInvalidator) *TenantService {
	routers := make([]tenantInvalidator, 0, len(invalidators))
	for _, inv := range invalidators {
		if inv != nil {
			routers = append(routers, inv)
		}
	}
	return &TenantService{repo: repo, routers: routers}
}

func (s *TenantService) List(ctx context.Context) ([]domain.Tenant, error) {
	return s.repo.List(ctx)
}

func (s *TenantService) GetByID(ctx context.Context, tenantID string) (domain.Tenant, error) {
	return s.repo.GetByID(ctx, tenantID)
}

func (s *TenantService) Create(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	item.DeploymentMode = strings.ToLower(strings.TrimSpace(item.DeploymentMode))
	if err := validateTenant(item); err != nil {
		return domain.Tenant{}, err
	}
	created, err := s.repo.Create(ctx, item)
	if err != nil {
		return domain.Tenant{}, err
	}
	s.invalidateTenant(created.TenantID)
	return created, nil
}

func (s *TenantService) UpdateConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	item.DeploymentMode = strings.ToLower(strings.TrimSpace(item.DeploymentMode))
	if err := validateTenant(item); err != nil {
		return domain.Tenant{}, err
	}
	updated, err := s.repo.UpdateConfig(ctx, item)
	if err != nil {
		return domain.Tenant{}, err
	}
	s.invalidateTenant(updated.TenantID)
	return updated, nil
}

func (s *TenantService) invalidateTenant(tenantID string) {
	for _, inv := range s.routers {
		inv.InvalidateTenant(tenantID)
	}
}

func validateTenant(item domain.Tenant) error {
	if strings.TrimSpace(item.TenantID) == "" {
		return errors.New("tenant_id is required")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("name is required")
	}
	if item.UserCountThreshold <= 0 {
		return errors.New("user_count_threshold must be greater than 0")
	}
	mode := strings.ToLower(strings.TrimSpace(item.DeploymentMode))
	if mode != "shared" && mode != "dedicated" {
		return errors.New("deployment_mode must be shared or dedicated")
	}
	if mode == "dedicated" && strings.TrimSpace(item.DedicatedDSN) == "" {
		return errors.New("dedicated_dsn is required for dedicated mode")
	}
	return nil
}
