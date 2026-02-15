package service

import (
	"context"

	chatdomain "msg_server/server/chat/domain"
	commondbman "msg_server/server/common/infra/dbman"
	"msg_server/server/common/infra/object"
	"msg_server/server/fileman/domain"
)

type DBManClient struct {
	client *commondbman.Client
}

const dbmanBasePath = commondbman.BasePath

func NewDBManClient(endpoints ...string) *DBManClient {
	return &DBManClient{
		client: commondbman.NewClientWithEndpoints(endpoints...),
	}
}

func (c *DBManClient) CreateFile(ctx context.Context, item domain.FileObject) (domain.FileObject, error) {
	var out domain.FileObject
	if err := c.client.Post(ctx, dbmanBasePath+"/files", item, &out); err != nil {
		return domain.FileObject{}, err
	}
	return out, nil
}

func (c *DBManClient) GetTenant(ctx context.Context, tenantID string) (chatdomain.Tenant, error) {
	var out chatdomain.Tenant
	payload := map[string]any{"tenant_id": tenantID}
	if err := c.client.Post(ctx, dbmanBasePath+"/tenants/get", payload, &out); err != nil {
		return chatdomain.Tenant{}, err
	}
	return out, nil
}

func (c *DBManClient) GetTenantMinIOMeta(ctx context.Context, tenantID string) (object.TenantMinIOMeta, error) {
	tenant, err := c.GetTenant(ctx, tenantID)
	if err != nil {
		return object.TenantMinIOMeta{}, err
	}
	return object.TenantMinIOMeta{
		Mode:      tenant.DeploymentMode,
		Endpoint:  tenant.DedicatedMinIOEndpoint,
		AccessKey: tenant.DedicatedMinIOAccessKey,
		SecretKey: tenant.DedicatedMinIOSecretKey,
		UseSSL:    tenant.DedicatedMinIOUseSSL,
		Bucket:    tenant.DedicatedMinIOBucket,
		IsActive:  tenant.IsActive,
	}, nil
}
