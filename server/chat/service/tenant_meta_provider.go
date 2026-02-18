package service

import (
	"context"
	"strings"

	"msg_server/server/common/infra/cache"
)

type DBManTenantMetaProvider struct {
	dbman *DBManClient
}

func NewDBManTenantMetaProvider(dbman *DBManClient) *DBManTenantMetaProvider {
	return &DBManTenantMetaProvider{dbman: dbman}
}

func (p *DBManTenantMetaProvider) GetTenantRedisMeta(ctx context.Context, tenantID string) (cache.TenantRedisMeta, error) {
	tenant, err := p.dbman.GetTenant(ctx, tenantID)
	if err != nil {
		return cache.TenantRedisMeta{}, err
	}
	return cache.TenantRedisMeta{
		DeploymentMode:     strings.ToLower(strings.TrimSpace(tenant.DeploymentMode)),
		DedicatedRedisAddr: strings.TrimSpace(tenant.DedicatedRedisAddr),
		IsActive:           tenant.IsActive,
	}, nil
}

func (p *DBManTenantMetaProvider) GetTenantMQMeta(ctx context.Context, tenantID string) (TenantMQMeta, error) {
	tenant, err := p.dbman.GetTenant(ctx, tenantID)
	if err != nil {
		return TenantMQMeta{}, err
	}
	return TenantMQMeta{
		DeploymentMode:      strings.ToLower(strings.TrimSpace(tenant.DeploymentMode)),
		DedicatedLavinMQURL: strings.TrimSpace(tenant.DedicatedLavinMQURL),
		IsActive:            tenant.IsActive,
	}, nil
}
