package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type WSClient struct {
	TenantID  string
	UserID    string
	SessionID string
	Conn      *websocket.Conn
	mu        sync.Mutex
}

type Hub struct {
	mu        sync.RWMutex
	clients   map[string]map[string]*WSClient
	redis     *redis.Client
	redisSub  *redis.PubSub
	subCancel context.CancelFunc
}

const sessionEventsChannel = "session:events"

type hubEvent struct {
	Kind      string            `json:"kind"`
	TenantID  string            `json:"tenant_id"`
	UserID    string            `json:"user_id,omitempty"`
	UserIDs   []string          `json:"user_ids,omitempty"`
	Payload   json.RawMessage   `json:"payload,omitempty"`
	PayloadBy map[string]string `json:"payload_by,omitempty"`
}

func NewHub() *Hub {
	return &Hub{clients: map[string]map[string]*WSClient{}}
}

func (h *Hub) UseRedis(client *redis.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.redis = client
}

func (h *Hub) StartRedisSubscriber(ctx context.Context) error {
	h.mu.Lock()
	if h.redis == nil {
		h.mu.Unlock()
		return errors.New("redis client is nil")
	}
	if h.redisSub != nil {
		h.mu.Unlock()
		return nil
	}
	subCtx, cancel := context.WithCancel(ctx)
	sub := h.redis.Subscribe(subCtx, sessionEventsChannel)
	h.redisSub = sub
	h.subCancel = cancel
	h.mu.Unlock()

	go h.consumeEvents(subCtx, sub)
	return nil
}

func (h *Hub) StopRedisSubscriber() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subCancel != nil {
		h.subCancel()
		h.subCancel = nil
	}
	if h.redisSub != nil {
		_ = h.redisSub.Close()
		h.redisSub = nil
	}
}

func userKey(tenantID, userID string) string {
	return tenantID + ":" + userID
}

func (h *Hub) Register(client *WSClient) {
	key := userKey(client.TenantID, client.UserID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[key]; !ok {
		h.clients[key] = map[string]*WSClient{}
	}
	h.clients[key][client.SessionID] = client
}

func (h *Hub) Unregister(client *WSClient) {
	key := userKey(client.TenantID, client.UserID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if sessions, ok := h.clients[key]; ok {
		delete(sessions, client.SessionID)
		if len(sessions) == 0 {
			delete(h.clients, key)
		}
	}
	_ = client.Conn.Close()
}

func (h *Hub) NotifyUser(tenantID, userID string, payload any) {
	if h.publishNotifyUser(tenantID, userID, payload) {
		return
	}
	fanoutCount := h.notifyUserLocal(tenantID, userID, payload)
	log.Printf("event=session_hub action=fallback_dispatch kind=%s tenant_id=%s fanout_count=%d", "notify_user", tenantID, fanoutCount)
}

func (h *Hub) notifyUserLocal(tenantID, userID string, payload any) int {
	key := userKey(tenantID, userID)
	h.mu.RLock()
	sessions := h.clients[key]
	h.mu.RUnlock()

	count := 0
	for _, client := range sessions {
		client.WriteJSON(payload)
		count++
	}
	return count
}

func (h *Hub) NotifyUsers(tenantID string, userIDs []string, payloadBuilder func(string) any) {
	if h.publishNotifyUsers(tenantID, userIDs, payloadBuilder) {
		return
	}
	total := 0
	for _, userID := range userIDs {
		total += h.notifyUserLocal(tenantID, userID, payloadBuilder(userID))
	}
	log.Printf("event=session_hub action=fallback_dispatch kind=%s tenant_id=%s fanout_count=%d", "notify_users", tenantID, total)
}

func (h *Hub) BroadcastTenant(tenantID string, payload any) {
	if h.publishBroadcastTenant(tenantID, payload) {
		return
	}
	fanoutCount := h.broadcastTenantLocal(tenantID, payload)
	log.Printf("event=session_hub action=fallback_dispatch kind=%s tenant_id=%s fanout_count=%d", "broadcast_tenant", tenantID, fanoutCount)
}

func (h *Hub) broadcastTenantLocal(tenantID string, payload any) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	prefix := tenantID + ":"
	count := 0
	for key, sessions := range h.clients {
		if len(key) < len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		for _, client := range sessions {
			client.WriteJSON(payload)
			count++
		}
	}
	return count
}

func (h *Hub) publishNotifyUser(tenantID, userID string, payload any) bool {
	h.mu.RLock()
	redisClient := h.redis
	h.mu.RUnlock()
	if redisClient == nil {
		return false
	}
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	event := hubEvent{Kind: "notify_user", TenantID: tenantID, UserID: userID, Payload: payloadRaw}
	b, err := json.Marshal(event)
	if err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "notify_user", tenantID, err)
		return false
	}
	if err := redisClient.Publish(context.Background(), sessionEventsChannel, b).Err(); err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "notify_user", tenantID, err)
		return false
	}
	log.Printf("event=session_hub action=publish status=ok kind=%s tenant_id=%s fanout_count=%d", "notify_user", tenantID, 1)
	return true
}

func (h *Hub) publishNotifyUsers(tenantID string, userIDs []string, payloadBuilder func(string) any) bool {
	h.mu.RLock()
	redisClient := h.redis
	h.mu.RUnlock()
	if redisClient == nil {
		return false
	}
	unique := make([]string, 0, len(userIDs))
	seen := map[string]struct{}{}
	payloadBy := map[string]string{}
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		unique = append(unique, userID)
		payloadRaw, err := json.Marshal(payloadBuilder(userID))
		if err != nil {
			return false
		}
		payloadBy[userID] = string(payloadRaw)
	}
	event := hubEvent{Kind: "notify_users", TenantID: tenantID, UserIDs: unique, PayloadBy: payloadBy}
	b, err := json.Marshal(event)
	if err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "notify_users", tenantID, err)
		return false
	}
	if err := redisClient.Publish(context.Background(), sessionEventsChannel, b).Err(); err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "notify_users", tenantID, err)
		return false
	}
	log.Printf("event=session_hub action=publish status=ok kind=%s tenant_id=%s fanout_count=%d", "notify_users", tenantID, len(unique))
	return true
}

func (h *Hub) publishBroadcastTenant(tenantID string, payload any) bool {
	h.mu.RLock()
	redisClient := h.redis
	h.mu.RUnlock()
	if redisClient == nil {
		return false
	}
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	event := hubEvent{Kind: "broadcast_tenant", TenantID: tenantID, Payload: payloadRaw}
	b, err := json.Marshal(event)
	if err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "broadcast_tenant", tenantID, err)
		return false
	}
	if err := redisClient.Publish(context.Background(), sessionEventsChannel, b).Err(); err != nil {
		log.Printf("event=session_hub action=publish status=failed kind=%s tenant_id=%s error=%v", "broadcast_tenant", tenantID, err)
		return false
	}
	log.Printf("event=session_hub action=publish status=ok kind=%s tenant_id=%s fanout_count=%d", "broadcast_tenant", tenantID, h.tenantSessionCount(tenantID))
	return true
}

func (h *Hub) tenantSessionCount(tenantID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	prefix := tenantID + ":"
	count := 0
	for key, sessions := range h.clients {
		if len(key) < len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		count += len(sessions)
	}
	return count
}

func (h *Hub) consumeEvents(ctx context.Context, sub *redis.PubSub) {
	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			return
		}
		var event hubEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			continue
		}
		switch event.Kind {
		case "notify_user":
			if len(event.Payload) == 0 {
				continue
			}
			var payload any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				continue
			}
			fanoutCount := h.notifyUserLocal(event.TenantID, event.UserID, payload)
			log.Printf("event=session_hub action=consume status=ok kind=%s tenant_id=%s fanout_count=%d", event.Kind, event.TenantID, fanoutCount)
		case "notify_users":
			total := 0
			for _, userID := range event.UserIDs {
				raw, ok := event.PayloadBy[userID]
				if !ok {
					continue
				}
				var payload any
				if err := json.Unmarshal([]byte(raw), &payload); err != nil {
					continue
				}
				total += h.notifyUserLocal(event.TenantID, userID, payload)
			}
			log.Printf("event=session_hub action=consume status=ok kind=%s tenant_id=%s fanout_count=%d", event.Kind, event.TenantID, total)
		case "broadcast_tenant":
			if len(event.Payload) == 0 {
				continue
			}
			var payload any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				continue
			}
			fanoutCount := h.broadcastTenantLocal(event.TenantID, payload)
			log.Printf("event=session_hub action=consume status=ok kind=%s tenant_id=%s fanout_count=%d", event.Kind, event.TenantID, fanoutCount)
		}
	}
}

func (c *WSClient) WriteJSON(payload any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = c.Conn.WriteJSON(payload)
}
