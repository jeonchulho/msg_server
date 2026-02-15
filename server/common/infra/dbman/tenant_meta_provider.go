package dbman

import (
	"context"
	"strings"

	"msg_server/server/common/infra/db"
)

type TenantMetaProvider struct {
	client *Client
}

func NewTenantMetaProvider(endpoint string) *TenantMetaProvider {
	return &TenantMetaProvider{client: NewClient(endpoint)}
}

func (p *TenantMetaProvider) GetTenantDBMeta(ctx context.Context, tenantID string) (db.TenantDBMeta, error) {
	var resp struct {
		DeploymentMode string `json:"deployment_mode"`
		DedicatedDSN   string `json:"dedicated_dsn"`
		IsActive       bool   `json:"is_active"`
	}
	payload := map[string]any{"tenant_id": tenantID}
	if err := p.client.Post(ctx, BasePath+"/tenants/get", payload, &resp); err != nil {
		return db.TenantDBMeta{}, err
	}
	return db.TenantDBMeta{
		DeploymentMode: strings.ToLower(strings.TrimSpace(resp.DeploymentMode)),
		DedicatedDSN:   strings.TrimSpace(resp.DedicatedDSN),
		IsActive:       resp.IsActive,
	}, nil
}
