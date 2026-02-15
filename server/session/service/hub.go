package service

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	TenantID  string
	UserID    string
	SessionID string
	Conn      *websocket.Conn
	mu        sync.Mutex
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[string]*WSClient
}

func NewHub() *Hub {
	return &Hub{clients: map[string]map[string]*WSClient{}}
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
	key := userKey(tenantID, userID)
	h.mu.RLock()
	sessions := h.clients[key]
	h.mu.RUnlock()

	for _, client := range sessions {
		client.WriteJSON(payload)
	}
}

func (h *Hub) NotifyUsers(tenantID string, userIDs []string, payloadBuilder func(string) any) {
	for _, userID := range userIDs {
		h.NotifyUser(tenantID, userID, payloadBuilder(userID))
	}
}

func (h *Hub) BroadcastTenant(tenantID string, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	prefix := tenantID + ":"
	for key, sessions := range h.clients {
		if len(key) < len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		for _, client := range sessions {
			client.WriteJSON(payload)
		}
	}
}

func (c *WSClient) WriteJSON(payload any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = c.Conn.WriteJSON(payload)
}
