package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"msg_server/server/session/domain"
)

type sessionServiceStore interface {
	UpsertDeviceSession(ctx context.Context, session domain.DeviceSession) (domain.DeviceSession, error)
	ValidateAndTouchSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error)
	UpdateSessionUserStatus(ctx context.Context, status domain.UserStatus) error
}

type SessionService struct {
	store sessionServiceStore
	hub   *Hub
}

func NewSessionService(store sessionServiceStore, hub *Hub) *SessionService {
	return &SessionService{store: store, hub: hub}
}

func (s *SessionService) LoginDevice(ctx context.Context, tenantID, userID, deviceID, deviceName, authToken string, allowedTenants []string) (domain.DeviceSession, error) {
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

func (s *SessionService) ValidateSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(sessionToken) == "" {
		return false, nil
	}
	return s.store.ValidateAndTouchSession(ctx, tenantID, userID, sessionID, sessionToken)
}

func (s *SessionService) UpdateStatus(ctx context.Context, tenantID, userID, status, statusNote string) error {
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
