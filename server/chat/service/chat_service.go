package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"msg_server/server/chat/domain"
)

type ChatService struct {
	mq     *AMQPPublisher
	dbman  *DBManClient
	vector *VectormanClient
}

func NewChatService(mq *AMQPPublisher, dbman *DBManClient, vector *VectormanClient) *ChatService {
	return &ChatService{mq: mq, dbman: dbman, vector: vector}
}

func (s *ChatService) CreateRoom(ctx context.Context, tenantID string, room domain.ChatRoom, memberIDs []string) (string, error) {
	return s.dbman.CreateRoom(ctx, tenantID, room, memberIDs)
}

func (s *ChatService) AddMember(ctx context.Context, tenantID, roomID, userID string) error {
	return s.dbman.AddMember(ctx, tenantID, roomID, userID)
}

func (s *ChatService) IsRoomMember(ctx context.Context, tenantID, roomID, userID string) (bool, error) {
	return s.dbman.IsRoomMember(ctx, tenantID, roomID, userID)
}

func (s *ChatService) CreateMessage(ctx context.Context, msg domain.Message) (domain.Message, error) {
	if msg.MetaJSON == "" {
		msg.MetaJSON = "{}"
	}
	created, err := s.dbman.CreateMessage(ctx, msg)
	if err != nil {
		return created, err
	}

	event := map[string]any{
		"event":      "message.created",
		"message_id": created.ID,
		"room_id":    created.RoomID,
		"sender_id":  created.SenderID,
		"body":       created.Body,
		"created_at": created.CreatedAt,
	}
	_ = s.mq.Publish(ctx, msg.TenantID, "message.created", event)
	_ = s.vector.IndexMessage(ctx, created.ID, created.RoomID, created.Body)
	_ = s.dbman.MarkReadUpTo(ctx, msg.TenantID, created.RoomID, created.SenderID, created.ID)

	return created, nil
}

func (s *ChatService) ListMessages(ctx context.Context, tenantID, roomID string, limit int, cursor string) ([]domain.Message, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var cursorID *string
	if strings.TrimSpace(cursor) != "" {
		parsed, err := decodeMessageCursor(cursor)
		if err != nil {
			return nil, "", errors.New("cursor is invalid")
		}
		cursorID = &parsed
	}

	items, err := s.dbman.ListMessages(ctx, tenantID, roomID, limit+1, cursorID)
	if err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeMessageCursor(last.ID)
	}
	return items, nextCursor, nil
}

func (s *ChatService) SearchMessages(ctx context.Context, tenantID string, q string, roomID *string, limit int, cursor string) ([]domain.Message, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var cursorID *string
	if strings.TrimSpace(cursor) != "" {
		parsed, err := decodeMessageCursor(cursor)
		if err != nil {
			return nil, "", errors.New("cursor is invalid")
		}
		cursorID = &parsed
	}

	items, err := s.dbman.SearchMessages(ctx, tenantID, q, roomID, limit+1, cursorID)
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeMessageCursor(last.ID)
	}

	if cursorID != nil {
		return items, nextCursor, nil
	}

	ids, err := s.vector.SemanticSearch(ctx, q, roomID, limit)
	if err != nil || len(ids) == 0 {
		return items, nextCursor, nil
	}

	prefer := map[string]int{}
	for i, id := range ids {
		prefer[id] = i
	}

	scored := make([]domain.Message, 0, len(items))
	for _, item := range items {
		if _, ok := prefer[item.ID]; ok {
			scored = append(scored, item)
		}
	}
	if len(scored) == 0 {
		return items, nextCursor, nil
	}

	for _, item := range items {
		if _, ok := prefer[item.ID]; !ok {
			scored = append(scored, item)
		}
	}
	return scored, nextCursor, nil
}

func BuildMessageMeta(fileID *string, emojis []string) string {
	meta := map[string]any{
		"emojis": emojis,
	}
	if fileID != nil {
		meta["file_id"] = *fileID
	}
	bytes, _ := json.Marshal(meta)
	return string(bytes)
}

func (s *ChatService) MarkReadUpTo(ctx context.Context, tenantID, roomID, userID, messageID string) error {
	return s.dbman.MarkReadUpTo(ctx, tenantID, roomID, userID, messageID)
}

func (s *ChatService) GetMessageReaders(ctx context.Context, tenantID, roomID, messageID string) ([]domain.MessageRead, error) {
	return s.dbman.GetMessageReaders(ctx, tenantID, roomID, messageID)
}

func (s *ChatService) GetLastReadMessageID(ctx context.Context, tenantID, roomID, userID string) (string, error) {
	return s.dbman.GetLastReadMessageID(ctx, tenantID, roomID, userID)
}

func (s *ChatService) GetUnreadCount(ctx context.Context, tenantID, roomID, userID string) (int64, error) {
	return s.dbman.GetUnreadCount(ctx, tenantID, roomID, userID)
}

func (s *ChatService) GetUnreadCounts(ctx context.Context, tenantID, userID string) ([]domain.RoomUnread, error) {
	return s.dbman.GetUnreadCounts(ctx, tenantID, userID)
}

func (s *ChatService) ListMyRooms(ctx context.Context, tenantID, userID string, limit int, cursor string) ([]domain.ChatRoomSummary, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var cursorCreatedAt *time.Time
	var cursorRoomID *string
	if strings.TrimSpace(cursor) != "" {
		createdAt, roomID, err := decodeRoomCursor(cursor)
		if err != nil {
			return nil, "", errors.New("cursor is invalid")
		}
		cursorCreatedAt = &createdAt
		cursorRoomID = &roomID
	}

	items, err := s.dbman.ListMyRooms(ctx, tenantID, userID, limit+1, cursorCreatedAt, cursorRoomID)
	if err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeRoomCursor(roomCursorTime(last), last.RoomID)
	}
	return items, nextCursor, nil
}

func encodeMessageCursor(messageID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(messageID))
}

func decodeMessageCursor(cursor string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	messageID := strings.TrimSpace(string(decoded))
	if messageID == "" {
		return "", errors.New("invalid cursor")
	}
	return messageID, nil
}

func roomCursorTime(item domain.ChatRoomSummary) time.Time {
	if item.LatestMessageAt != nil {
		return item.LatestMessageAt.UTC()
	}
	return item.CreatedAt.UTC()
}

func encodeRoomCursor(createdAt time.Time, roomID string) string {
	raw := fmt.Sprintf("%d:%s", createdAt.UnixNano(), roomID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeRoomCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor format")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}
	roomID := strings.TrimSpace(parts[1])
	if roomID == "" {
		return time.Time{}, "", errors.New("invalid cursor room id")
	}
	return time.Unix(0, nanos).UTC(), roomID, nil
}
