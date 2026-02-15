package service

import (
	"context"
	"time"

	"msg_server/server/chat/domain"
	"msg_server/server/dbman/repository"
)

type ChatService struct {
	repo *repository.ChatRepository
}

func NewChatService(repo *repository.ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) CreateRoom(ctx context.Context, tenantID string, room domain.ChatRoom, memberIDs []string) (string, error) {
	return s.repo.CreateRoom(ctx, tenantID, room, memberIDs)
}

func (s *ChatService) AddMember(ctx context.Context, tenantID, roomID, userID string) error {
	return s.repo.AddMember(ctx, tenantID, roomID, userID)
}

func (s *ChatService) IsRoomMember(ctx context.Context, tenantID, roomID, userID string) (bool, error) {
	return s.repo.IsRoomMember(ctx, tenantID, roomID, userID)
}

func (s *ChatService) CreateMessage(ctx context.Context, msg domain.Message) (domain.Message, error) {
	return s.repo.CreateMessage(ctx, msg)
}

func (s *ChatService) MarkReadUpTo(ctx context.Context, tenantID, roomID, userID, messageID string) error {
	return s.repo.MarkReadUpTo(ctx, tenantID, roomID, userID, messageID)
}

func (s *ChatService) ListMessages(ctx context.Context, tenantID, roomID string, limit int, cursorID *string) ([]domain.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListMessages(ctx, tenantID, roomID, limit, cursorID)
}

func (s *ChatService) SearchMessages(ctx context.Context, tenantID, q string, roomID *string, limit int, cursorID *string) ([]domain.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	return s.repo.SearchMessages(ctx, tenantID, q, roomID, limit, cursorID)
}

func (s *ChatService) GetMessageReaders(ctx context.Context, tenantID, roomID, messageID string) ([]domain.MessageRead, error) {
	return s.repo.GetMessageReaders(ctx, tenantID, roomID, messageID)
}

func (s *ChatService) GetLastReadMessageID(ctx context.Context, tenantID, roomID, userID string) (string, error) {
	return s.repo.GetLastReadMessageID(ctx, tenantID, roomID, userID)
}

func (s *ChatService) GetUnreadCount(ctx context.Context, tenantID, roomID, userID string) (int64, error) {
	return s.repo.GetUnreadCount(ctx, tenantID, roomID, userID)
}

func (s *ChatService) GetUnreadCounts(ctx context.Context, tenantID, userID string) ([]domain.RoomUnread, error) {
	return s.repo.GetUnreadCounts(ctx, tenantID, userID)
}

func (s *ChatService) ListMyRooms(ctx context.Context, tenantID, userID string, limit int, cursorCreatedAt *time.Time, cursorRoomID *string) ([]domain.ChatRoomSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListMyRooms(ctx, tenantID, userID, limit, cursorCreatedAt, cursorRoomID)
}
