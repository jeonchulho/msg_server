package tenanthub

import (
	"context"

	"msg_server/server/chat/domain"
	commondbman "msg_server/server/common/infra/dbman"
)

type Client struct {
	client *commondbman.Client
}

const dbmanBasePath = commondbman.BasePath

func NewDBManClient(endpoints ...string) *Client {
	return &Client{client: commondbman.NewClientWithEndpoints(endpoints...)}
}

func (c *Client) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var items []domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/list", map[string]any{}, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) CreateTenant(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	var created domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/create", item, &created); err != nil {
		return domain.Tenant{}, err
	}
	return created, nil
}

func (c *Client) UpdateTenantConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	var updated domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/update", item, &updated); err != nil {
		return domain.Tenant{}, err
	}
	return updated, nil
}

func (c *Client) post(ctx context.Context, path string, payload any, out any) error {
	return c.client.Post(ctx, path, payload, out)
}
