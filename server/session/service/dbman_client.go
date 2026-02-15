package service

import (
	"context"
	"time"

	"msg_server/server/chat/domain"
	commondbman "msg_server/server/common/infra/dbman"
	sessiondomain "msg_server/server/session/domain"
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

func (c *DBManClient) UpsertDeviceSession(ctx context.Context, session sessiondomain.DeviceSession) (sessiondomain.DeviceSession, error) {
	var out sessiondomain.DeviceSession
	if err := c.post(ctx, dbmanBasePath+"/session/device/login", session, &out); err != nil {
		return sessiondomain.DeviceSession{}, err
	}
	return out, nil
}

func (c *DBManClient) ValidateAndTouchSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error) {
	payload := map[string]any{
		"tenant_id":     tenantID,
		"user_id":       userID,
		"session_id":    sessionID,
		"session_token": sessionToken,
	}
	var out struct {
		Valid bool `json:"valid"`
	}
	if err := c.post(ctx, dbmanBasePath+"/session/device/validate", payload, &out); err != nil {
		return false, err
	}
	return out.Valid, nil
}

func (c *DBManClient) UpdateSessionUserStatus(ctx context.Context, status sessiondomain.UserStatus) error {
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/session/status/update", status, &out)
}

func (c *DBManClient) CreateSessionNote(ctx context.Context, note sessiondomain.Note) (sessiondomain.Note, error) {
	var out sessiondomain.Note
	if err := c.post(ctx, dbmanBasePath+"/session/notes/create", note, &out); err != nil {
		return sessiondomain.Note{}, err
	}
	return out, nil
}

func (c *DBManClient) ListSessionInbox(ctx context.Context, tenantID, userID string, limit int) ([]sessiondomain.NoteInboxItem, error) {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "limit": limit}
	var out []sessiondomain.NoteInboxItem
	if err := c.post(ctx, dbmanBasePath+"/session/notes/inbox", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *DBManClient) MarkSessionNoteRead(ctx context.Context, tenantID, userID, noteID string) error {
	payload := map[string]any{"tenant_id": tenantID, "user_id": userID, "note_id": noteID}
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/session/notes/read", payload, &out)
}

func (c *DBManClient) SaveSessionChatNotifications(ctx context.Context, tenantID, senderUserID string, input sessiondomain.ChatNotifyInput) error {
	payload := map[string]any{"tenant_id": tenantID, "sender_user_id": senderUserID, "input": input}
	var out map[string]any
	return c.post(ctx, dbmanBasePath+"/session/chat/notify", payload, &out)
}

func (c *DBManClient) post(ctx context.Context, path string, payload any, out any) error {
	return c.client.Post(ctx, path, payload, out)
}
