package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type VectormanClient struct {
	endpoint string
	enabled  bool
	client   *http.Client
}

func NewVectormanClient(endpoint string, enabled bool) *VectormanClient {
	return &VectormanClient{
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		enabled:  enabled,
		client:   &http.Client{Timeout: 4 * time.Second},
	}
}

func (m *VectormanClient) IndexMessage(ctx context.Context, messageID, roomID string, text string) error {
	if !m.enabled {
		return nil
	}
	payload := map[string]any{"message_id": messageID, "room_id": roomID, "text": text}
	_, err := m.post(ctx, "/api/v1/vectors/messages/index", payload)
	return err
}

func (m *VectormanClient) SemanticSearch(ctx context.Context, query string, roomID *string, limit int) ([]string, error) {
	if !m.enabled {
		return nil, nil
	}
	payload := map[string]any{"query": query, "room_id": roomID, "limit": limit}
	resp, err := m.post(ctx, "/api/v1/vectors/messages/search", payload)
	if err != nil {
		return nil, err
	}
	idsAny, ok := resp["ids"]
	if !ok {
		return nil, nil
	}
	rawIDs, ok := idsAny.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid ids payload")
	}
	items := make([]string, 0, len(rawIDs))
	for _, item := range rawIDs {
		if asString, ok := item.(string); ok {
			items = append(items, asString)
		}
	}
	return items, nil
}

func (m *VectormanClient) post(ctx context.Context, path string, payload any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint+path, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vectorman status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
