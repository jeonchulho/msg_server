package service

import (
	"context"

	commondbman "msg_server/server/common/infra/dbman"
	sessiondomain "msg_server/server/session/domain"
)

type DBManClient struct {
	client *commondbman.Client
}

const dbmanBasePath = commondbman.BasePath

func NewDBManClient(endpoints ...string) *DBManClient {
	return &DBManClient{client: commondbman.NewClientWithEndpoints(endpoints...)}
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
