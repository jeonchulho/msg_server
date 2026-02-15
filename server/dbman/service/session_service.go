package service

import (
	"context"

	"msg_server/server/dbman/repository"
	"msg_server/server/session/domain"
)

type SessionService struct {
	repo *repository.SessionRepository
}

func NewSessionService(repo *repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) UpsertDeviceSession(ctx context.Context, session domain.DeviceSession) (domain.DeviceSession, error) {
	return s.repo.UpsertDeviceSession(ctx, session)
}

func (s *SessionService) ValidateAndTouchSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error) {
	return s.repo.ValidateAndTouchSession(ctx, tenantID, userID, sessionID, sessionToken)
}

func (s *SessionService) UpdateUserStatus(ctx context.Context, status domain.UserStatus) error {
	return s.repo.UpdateUserStatus(ctx, status)
}

func (s *SessionService) CreateNote(ctx context.Context, note domain.Note) (domain.Note, error) {
	return s.repo.CreateNote(ctx, note)
}

func (s *SessionService) ListInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error) {
	return s.repo.ListInbox(ctx, tenantID, userID, limit)
}

func (s *SessionService) MarkNoteRead(ctx context.Context, tenantID, userID, noteID string) error {
	return s.repo.MarkNoteRead(ctx, tenantID, userID, noteID)
}

func (s *SessionService) SaveChatNotifications(ctx context.Context, tenantID, senderUserID string, input domain.ChatNotifyInput) error {
	return s.repo.SaveChatNotifications(ctx, tenantID, senderUserID, input)
}
