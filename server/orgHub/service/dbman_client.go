package orghub

import (
	"context"
	"time"

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

func (c *Client) CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "parent_id": parentID, "name": name}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/org-units/create", payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error) {
	payload := map[string]any{"tenant_id": tenantID}
	var items []domain.OrgUnit
	if err := c.post(ctx, dbmanBasePath+"/org-units/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "user": user}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/users/create", payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) UpdateUserStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "status": status, "note": note}
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/users/status", payload, &out)
}

func (c *Client) SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error) {
	payload := map[string]any{"tenant_id": tenantID, "q": q, "limit": limit}
	var items []domain.User
	if err := c.post(ctx, dbmanBasePath+"/users/search", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) AuthenticateUser(ctx context.Context, tenantID, email, password string) (domain.User, error) {
	payload := map[string]any{"tenant_id": tenantID, "email": email, "password": password}
	var user domain.User
	if err := c.post(ctx, dbmanBasePath+"/users/authenticate", payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) ListAliases(ctx context.Context, tenantID, userID string) ([]string, error) {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID}
	var items []string
	if err := c.post(ctx, dbmanBasePath+"/users/aliases/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) AddAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "alias": alias, "ip": ip, "user_agent": userAgent}
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/users/aliases/add", payload, &out)
}

func (c *Client) DeleteAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "alias": alias, "ip": ip, "user_agent": userAgent}
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/users/aliases/delete", payload, &out)
}

func (c *Client) ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action string, cursorCreatedAt *time.Time, cursorID *string) ([]domain.AliasAudit, error) {
	payload := map[string]any{
		"tenant_id":         tenantID,
		"user_id":           userID,
		"limit":             limit,
		"from":              from,
		"to":                to,
		"action":            action,
		"cursor_created_at": cursorCreatedAt,
		"cursor_id":         cursorID,
	}
	var items []domain.AliasAudit
	if err := c.post(ctx, dbmanBasePath+"/users/aliases/audit", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) post(ctx context.Context, path string, payload any, out any) error {
	return c.client.Post(ctx, path, payload, out)
}
