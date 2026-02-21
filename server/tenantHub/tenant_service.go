package tenanthub

import (
	"context"

	"msg_server/server/chat/domain"
)

type DBManClient interface {
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	CreateTenant(ctx context.Context, item domain.Tenant) (domain.Tenant, error)
	UpdateTenantConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error)
}

type Invalidator interface {
	InvalidateTenant(tenantID string)
}

type Service struct {
	dbman   DBManClient
	routers []Invalidator
}

func NewService(dbman DBManClient, invalidators ...Invalidator) *Service {
	routers := make([]Invalidator, 0, len(invalidators))
	for _, inv := range invalidators {
		if inv != nil {
			routers = append(routers, inv)
		}
	}
	return &Service{dbman: dbman, routers: routers}
}

func (s *Service) List(ctx context.Context) ([]domain.Tenant, error) {
	return s.dbman.ListTenants(ctx)
}

func (s *Service) Create(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	created, err := s.dbman.CreateTenant(ctx, item)
	if err != nil {
		return domain.Tenant{}, err
	}
	s.invalidateTenant(created.TenantID)
	return created, nil
}

func (s *Service) UpdateConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	updated, err := s.dbman.UpdateTenantConfig(ctx, item)
	if err != nil {
		return domain.Tenant{}, err
	}
	s.invalidateTenant(updated.TenantID)
	return updated, nil
}

func (s *Service) invalidateTenant(tenantID string) {
	for _, inv := range s.routers {
		inv.InvalidateTenant(tenantID)
	}
}
