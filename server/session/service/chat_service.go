package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"msg_server/server/session/domain"
)

type chatServiceStore interface {
	SaveSessionChatNotifications(ctx context.Context, tenantID, senderUserID string, input domain.ChatNotifyInput) error
}

type ChatService struct {
	store chatServiceStore
	hub   *Hub
}

func NewChatService(store chatServiceStore, hub *Hub) *ChatService {
	return &ChatService{store: store, hub: hub}
}

func (s *ChatService) NotifyChat(ctx context.Context, tenantID, senderUserID, authToken string, input domain.ChatNotifyInput) error {
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
