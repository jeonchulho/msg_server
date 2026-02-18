package service

import (
	"context"
	"strings"
	"time"

	"msg_server/server/chat/domain"
	"msg_server/server/common/infra/cache"
	commondbman "msg_server/server/common/infra/dbman"
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

func (c *DBManClient) CreateRoom(ctx context.Context, tenantID string, room domain.ChatRoom, memberIDs []string) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "room": room, "member_ids": memberIDs}
	var resp struct {
		RoomID string `json:"room_id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/rooms", payload, &resp); err != nil {
		return "", err
	}
	return resp.RoomID, nil
}

func (c *DBManClient) AddMember(ctx context.Context, tenantID, roomID, userID string) error {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "user_id": userID}
	var resp map[string]any
	return c.post(ctx, dbmanBasePath+"/rooms/members", payload, &resp)
}

func (c *DBManClient) IsRoomMember(ctx context.Context, tenantID, roomID, userID string) (bool, error) {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "user_id": userID}
	var resp struct {
		OK bool `json:"ok"`
	}
	if err := c.post(ctx, dbmanBasePath+"/rooms/members/check", payload, &resp); err != nil {
		return false, err
	}
	return resp.OK, nil
}

func (c *DBManClient) CreateMessage(ctx context.Context, msg domain.Message) (domain.Message, error) {
	var out domain.Message
	if err := c.post(ctx, dbmanBasePath+"/messages", msg, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *DBManClient) MarkReadUpTo(ctx context.Context, tenantID, roomID, userID, messageID string) error {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "user_id": userID, "message_id": messageID}
	var resp map[string]any
	return c.post(ctx, dbmanBasePath+"/messages/read", payload, &resp)
}

func (c *DBManClient) SearchMessages(ctx context.Context, tenantID, q string, roomID *string, limit int, cursorID *string) ([]domain.Message, error) {
	payload := map[string]any{
		"tenant_id": tenantID,
		"q":         q,
		"room_id":   roomID,
		"limit":     limit,
		"cursor_id": cursorID,
	}
	var items []domain.Message
	if err := c.post(ctx, dbmanBasePath+"/messages/search", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) ListMessages(ctx context.Context, tenantID, roomID string, limit int, cursorID *string) ([]domain.Message, error) {
	payload := map[string]any{
		"tenant_id": tenantID,
		"room_id":   roomID,
		"limit":     limit,
		"cursor_id": cursorID,
	}
	var items []domain.Message
	if err := c.post(ctx, dbmanBasePath+"/messages/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) GetMessageReaders(ctx context.Context, tenantID, roomID, messageID string) ([]domain.MessageRead, error) {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "message_id": messageID}
	var items []domain.MessageRead
	if err := c.post(ctx, dbmanBasePath+"/messages/readers", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) GetLastReadMessageID(ctx context.Context, tenantID, roomID, userID string) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "user_id": userID}
	var resp struct {
		MessageID string `json:"message_id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/messages/last-read", payload, &resp); err != nil {
		return "", err
	}
	return resp.MessageID, nil
}

func (c *DBManClient) GetUnreadCount(ctx context.Context, tenantID, roomID, userID string) (int64, error) {
	payload := map[string]any{"tenant_id": tenantID, "room_id": roomID, "user_id": userID}
	var resp struct {
		Count int64 `json:"count"`
	}
	if err := c.post(ctx, dbmanBasePath+"/messages/unread-count", payload, &resp); err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (c *DBManClient) GetUnreadCounts(ctx context.Context, tenantID, userID string) ([]domain.RoomUnread, error) {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID}
	var items []domain.RoomUnread
	if err := c.post(ctx, dbmanBasePath+"/messages/unread-counts", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) ListMyRooms(ctx context.Context, tenantID, userID string, limit int, cursorCreatedAt *time.Time, cursorRoomID *string) ([]domain.ChatRoomSummary, error) {
	payload := map[string]any{
		"tenant_id":         tenantID,
		"user_id":           userID,
		"limit":             limit,
		"cursor_created_at": cursorCreatedAt,
		"cursor_room_id":    cursorRoomID,
	}
	var items []domain.ChatRoomSummary
	if err := c.post(ctx, dbmanBasePath+"/rooms/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "parent_id": parentID, "name": name}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/org-units/create", payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *DBManClient) ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error) {
	payload := map[string]any{"tenant_id": tenantID}
	var items []domain.OrgUnit
	if err := c.post(ctx, dbmanBasePath+"/org-units/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error) {
	payload := map[string]any{"tenant_id": tenantID, "user": user}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, dbmanBasePath+"/users/create", payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *DBManClient) UpdateUserStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "status": status, "note": note}
	var resp map[string]any
	return c.post(ctx, dbmanBasePath+"/users/status", payload, &resp)
}

func (c *DBManClient) SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error) {
	payload := map[string]any{"tenant_id": tenantID, "q": q, "limit": limit}
	var items []domain.User
	if err := c.post(ctx, dbmanBasePath+"/users/search", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) AuthenticateUser(ctx context.Context, tenantID, email, password string) (domain.User, error) {
	payload := map[string]any{"tenant_id": tenantID, "email": email, "password": password}
	var user domain.User
	if err := c.post(ctx, dbmanBasePath+"/users/authenticate", payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *DBManClient) ListAliases(ctx context.Context, tenantID, userID string) ([]string, error) {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID}
	var items []string
	if err := c.post(ctx, dbmanBasePath+"/users/aliases/list", payload, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) AddAlias(ctx context.Context, tenantID, userID, alias, ip, userAgent string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "alias": alias, "ip": ip, "user_agent": userAgent}
	var resp map[string]any
	return c.post(ctx, dbmanBasePath+"/users/aliases/add", payload, &resp)
}

func (c *DBManClient) DeleteAlias(ctx context.Context, tenantID, userID, alias, ip, userAgent string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "alias": alias, "ip": ip, "user_agent": userAgent}
	var resp map[string]any
	return c.post(ctx, dbmanBasePath+"/users/aliases/delete", payload, &resp)
}

func (c *DBManClient) ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action string, cursorCreatedAt *time.Time, cursorID *string) ([]domain.AliasAudit, error) {
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

func (c *DBManClient) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var items []domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/list", map[string]any{}, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *DBManClient) GetTenant(ctx context.Context, tenantID string) (domain.Tenant, error) {
	var item domain.Tenant
	payload := map[string]any{"tenant_id": tenantID}
	if err := c.post(ctx, dbmanBasePath+"/tenants/get", payload, &item); err != nil {
		return domain.Tenant{}, err
	}
	return item, nil
}

func (c *DBManClient) GetTenantRedisMeta(ctx context.Context, tenantID string) (cache.TenantRedisMeta, error) {
	tenant, err := c.GetTenant(ctx, tenantID)
	if err != nil {
		return cache.TenantRedisMeta{}, err
	}
	return cache.TenantRedisMeta{
		DeploymentMode:     strings.ToLower(strings.TrimSpace(tenant.DeploymentMode)),
		DedicatedRedisAddr: strings.TrimSpace(tenant.DedicatedRedisAddr),
		IsActive:           tenant.IsActive,
	}, nil
}

func (c *DBManClient) GetTenantMQMeta(ctx context.Context, tenantID string) (TenantMQMeta, error) {
	tenant, err := c.GetTenant(ctx, tenantID)
	if err != nil {
		return TenantMQMeta{}, err
	}
	return TenantMQMeta{
		DeploymentMode:      strings.ToLower(strings.TrimSpace(tenant.DeploymentMode)),
		DedicatedLavinMQURL: strings.TrimSpace(tenant.DedicatedLavinMQURL),
		IsActive:            tenant.IsActive,
	}, nil
}

func (c *DBManClient) CreateTenant(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	var created domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/create", item, &created); err != nil {
		return domain.Tenant{}, err
	}
	return created, nil
}

func (c *DBManClient) UpdateTenantConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	var updated domain.Tenant
	if err := c.post(ctx, dbmanBasePath+"/tenants/update", item, &updated); err != nil {
		return domain.Tenant{}, err
	}
	return updated, nil
}

func (c *DBManClient) post(ctx context.Context, path string, payload any, out any) error {
	return c.client.Post(ctx, path, payload, out)
}
