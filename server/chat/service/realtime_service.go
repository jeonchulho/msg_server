package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"msg_server/server/chat/domain"
	"msg_server/server/common/infra/cache"
)

type wsEnvelope struct {
	Type     string `json:"type"`
	RoomID   string `json:"room_id"`
	UserID   string `json:"user_id"`
	TargetID string `json:"target_id"`
	Payload  any    `json:"payload"`
}

type RealtimeService struct {
	tenantRedisRouter *cache.TenantRedisRouter
	chat              *ChatService
	mu                sync.RWMutex
	rooms             map[string]*roomState
}

type roomState struct {
	conns  map[*websocket.Conn]struct{}
	cancel context.CancelFunc
}

func NewRealtimeService(tenantRedisRouter *cache.TenantRedisRouter, chat *ChatService) *RealtimeService {
	return &RealtimeService{
		tenantRedisRouter: tenantRedisRouter,
		chat:              chat,
		rooms:             map[string]*roomState{},
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *RealtimeService) HandleWS(c *gin.Context) {
	tenantID := strings.TrimSpace(c.Query("tenant_id"))
	if rawTenantID, ok := c.Get("auth_tenant_id"); ok {
		if authTenantID, ok := rawTenantID.(string); ok && strings.TrimSpace(authTenantID) != "" {
			tenantID = strings.TrimSpace(authTenantID)
		}
	}
	if tenantID == "" {
		tenantID = "default"
	}
	authUserID := ""
	if rawUserID, ok := c.Get("auth_user_id"); ok {
		if userID, ok := rawUserID.(string); ok {
			authUserID = strings.TrimSpace(userID)
		}
	}
	roomID := parseInt64(c.Query("room_id"))
	if strings.TrimSpace(roomID) == "" {
		c.JSON(400, gin.H{"error": "room_id required"})
		return
	}
	redisClient, err := s.tenantRedisRouter.ClientForTenant(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	roomKey := fmt.Sprintf("%s:%s", tenantID, roomID)
	channel := fmt.Sprintf("tenant:%s:room:%s", tenantID, roomID)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s.join(roomKey, channel, redisClient, conn)
	defer s.leave(roomKey, conn)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		env.RoomID = roomID
		if authUserID != "" {
			env.UserID = authUserID
		}
		if env.Type == "message" {
			if strings.TrimSpace(env.UserID) == "" {
				writeWSError(conn, "unauthorized")
				continue
			}
			parsed, err := parseWSMessagePayload(env.Payload)
			if err != nil {
				writeWSError(conn, err.Error())
				continue
			}
			created, err := s.chat.CreateMessage(ctx, domain.Message{
				TenantID: tenantID,
				RoomID:   roomID,
				SenderID: env.UserID,
				Body:     parsed.Body,
				MetaJSON: BuildMessageMeta(parsed.FileID, parsed.Emojis),
			})
			if err != nil {
				writeWSError(conn, "failed to persist message")
				continue
			}
			env.Payload = created
		}
		if env.Type == "webrtc_offer" || env.Type == "webrtc_answer" || env.Type == "webrtc_ice" {
			env.Type = "signal_" + env.Type
		}
		b, _ := json.Marshal(env)
		_ = redisClient.Publish(ctx, channel, b).Err()
	}
}

type wsMessagePayload struct {
	Body   string   `json:"body"`
	FileID *string  `json:"file_id"`
	Emojis []string `json:"emojis"`
}

func parseWSMessagePayload(payload any) (wsMessagePayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return wsMessagePayload{}, errors.New("invalid message payload")
	}
	var out wsMessagePayload
	if err := json.Unmarshal(b, &out); err != nil {
		return wsMessagePayload{}, errors.New("invalid message payload")
	}
	if strings.TrimSpace(out.Body) == "" {
		return wsMessagePayload{}, errors.New("body required")
	}
	return out, nil
}

func writeWSError(conn *websocket.Conn, message string) {
	b, _ := json.Marshal(gin.H{"type": "error", "error": message})
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = conn.WriteMessage(websocket.TextMessage, b)
}

func (s *RealtimeService) consumeRedis(ctx context.Context, roomKey, channel string, redisClient *redis.Client) {
	pubsub := redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			return
		}
		s.mu.RLock()
		state := s.rooms[roomKey]
		if state == nil {
			s.mu.RUnlock()
			continue
		}
		for conn := range state.conns {
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		}
		s.mu.RUnlock()
	}
}

func (s *RealtimeService) join(roomKey, channel string, redisClient *redis.Client, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.rooms[roomKey]
	if !ok {
		roomCtx, cancel := context.WithCancel(context.Background())
		state = &roomState{conns: map[*websocket.Conn]struct{}{}, cancel: cancel}
		s.rooms[roomKey] = state
		go s.consumeRedis(roomCtx, roomKey, channel, redisClient)
	}
	state.conns[conn] = struct{}{}
}

func (s *RealtimeService) leave(roomKey string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.rooms[roomKey]; ok {
		delete(state.conns, conn)
		if len(state.conns) == 0 {
			state.cancel()
			delete(s.rooms, roomKey)
		}
	}
	_ = conn.Close()
}

func parseInt64(v string) string {
	return strings.TrimSpace(v)
}
