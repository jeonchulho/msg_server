package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"msg_server/server/session/domain"
)

type sessionStore interface {
	UpsertDeviceSession(ctx context.Context, session domain.DeviceSession) (domain.DeviceSession, error)
	ValidateAndTouchSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error)
	UpdateSessionUserStatus(ctx context.Context, status domain.UserStatus) error
	CreateSessionNote(ctx context.Context, note domain.Note) (domain.Note, error)
	ListSessionInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error)
	MarkSessionNoteRead(ctx context.Context, tenantID, userID, noteID string) error
	SaveSessionChatNotifications(ctx context.Context, tenantID, senderUserID string, input domain.ChatNotifyInput) error
}

type Service struct {
	store sessionStore
	hub   *Hub
}

func NewService(store sessionStore, hub *Hub) *Service {
	return &Service{store: store, hub: hub}
}

func (s *Service) LoginDevice(ctx context.Context, tenantID, userID, deviceID, deviceName, authToken string, allowedTenants []string) (domain.DeviceSession, error) {
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	authToken = strings.TrimSpace(authToken)
	if tenantID == "" || userID == "" || deviceID == "" {
		return domain.DeviceSession{}, errors.New("tenant_id, user_id, device_id are required")
	}

	if len(allowedTenants) == 0 {
		allowedTenants = []string{tenantID}
	}
	if !containsString(allowedTenants, tenantID) {
		allowedTenants = append(allowedTenants, tenantID)
	}
	token, err := randomToken(32)
	if err != nil {
		return domain.DeviceSession{}, err
	}

	session, err := s.store.UpsertDeviceSession(ctx, domain.DeviceSession{
		TenantID:       tenantID,
		UserID:         userID,
		DeviceID:       deviceID,
		DeviceName:     strings.TrimSpace(deviceName),
		SessionToken:   token,
		AllowedTenants: dedupeAndTrim(allowedTenants),
	})
	if err != nil {
		return domain.DeviceSession{}, err
	}
	session.AuthToken = authToken
	return session, nil
}

func (s *Service) ValidateSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(sessionToken) == "" {
		return false, nil
	}
	return s.store.ValidateAndTouchSession(ctx, tenantID, userID, sessionID, sessionToken)
}

func (s *Service) UpdateStatus(ctx context.Context, tenantID, userID, status, statusNote string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return errors.New("status is required")
	}
	if !containsString([]string{"online", "offline", "busy", "away", "meeting"}, status) {
		return errors.New("status must be one of online|offline|busy|away|meeting")
	}
	item := domain.UserStatus{
		TenantID:   tenantID,
		UserID:     userID,
		Status:     status,
		StatusNote: strings.TrimSpace(statusNote),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.store.UpdateSessionUserStatus(ctx, item); err != nil {
		return err
	}
	s.hub.BroadcastTenant(tenantID, map[string]any{
		"type":        "status.changed",
		"tenant_id":   tenantID,
		"user_id":     userID,
		"status":      item.Status,
		"status_note": item.StatusNote,
		"updated_at":  item.UpdatedAt,
	})
	return nil
}

func (s *Service) SendNote(ctx context.Context, tenantID, senderUserID string, input domain.NoteCreateInput) (domain.Note, error) {
	if strings.TrimSpace(input.Title) == "" {
		return domain.Note{}, errors.New("title is required")
	}
	if len(input.To)+len(input.CC)+len(input.BCC) == 0 {
		return domain.Note{}, errors.New("at least one recipient is required")
	}

	recipients := make([]domain.NoteRecipient, 0, len(input.To)+len(input.CC)+len(input.BCC))
	for _, userID := range dedupeAndTrim(input.To) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "to"})
	}
	for _, userID := range dedupeAndTrim(input.CC) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "cc"})
	}
	for _, userID := range dedupeAndTrim(input.BCC) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "bcc"})
	}

	note, err := s.store.CreateSessionNote(ctx, domain.Note{
		TenantID:     tenantID,
		SenderUserID: senderUserID,
		Title:        strings.TrimSpace(input.Title),
		Body:         input.Body,
		Recipients:   recipients,
		Files:        input.Files,
	})
	if err != nil {
		return domain.Note{}, err
	}

	recipientTypeByUser := map[string]string{}
	for _, recipient := range note.Recipients {
		if _, ok := recipientTypeByUser[recipient.UserID]; !ok {
			recipientTypeByUser[recipient.UserID] = recipient.Type
		}
	}
	userIDs := make([]string, 0, len(recipientTypeByUser))
	for userID := range recipientTypeByUser {
		userIDs = append(userIDs, userID)
	}

	s.hub.NotifyUsers(tenantID, userIDs, func(userID string) any {
		return map[string]any{
			"type":           "note.received",
			"note_id":        note.NoteID,
			"tenant_id":      tenantID,
			"sender_user_id": senderUserID,
			"recipient_type": recipientTypeByUser[userID],
			"title":          note.Title,
			"body":           note.Body,
			"files":          note.Files,
			"created_at":     note.CreatedAt,
		}
	})

	return note, nil
}

func (s *Service) NotifyChat(ctx context.Context, tenantID, senderUserID, authToken string, input domain.ChatNotifyInput) error {
	if strings.TrimSpace(input.RoomID) == "" {
		return errors.New("room_id is required")
	}
	authToken = strings.TrimSpace(authToken)
	recipients := dedupeAndTrim(input.RecipientIDs)
	if len(recipients) == 0 {
		return errors.New("recipient_ids is required")
	}
	if err := s.store.SaveSessionChatNotifications(ctx, tenantID, senderUserID, input); err != nil {
		return err
	}
	s.hub.NotifyUsers(tenantID, recipients, func(userID string) any {
		return map[string]any{
			"type":              "chat.notification",
			"tenant_id":         tenantID,
			"room_id":           input.RoomID,
			"message_id":        input.MessageID,
			"auth_token":        authToken,
			"sender_user_id":    senderUserID,
			"recipient_user_id": userID,
			"title":             input.Title,
			"body":              input.Body,
			"created_at":        time.Now().UTC(),
		}
	})
	return nil
}

func (s *Service) ListInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListSessionInbox(ctx, tenantID, userID, limit)
}

func (s *Service) MarkNoteRead(ctx context.Context, tenantID, userID, noteID string) error {
	if strings.TrimSpace(noteID) == "" {
		return errors.New("note_id is required")
	}
	return s.store.MarkSessionNoteRead(ctx, tenantID, userID, noteID)
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func dedupeAndTrim(items []string) []string {
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}
