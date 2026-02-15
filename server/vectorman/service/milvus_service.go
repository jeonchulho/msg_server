package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type MilvusService struct {
	endpoint string
	enabled  bool
	client   *http.Client
}

func NewMilvusService(endpoint string, enabled bool) *MilvusService {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	return &MilvusService{endpoint: normalizedEndpoint, enabled: enabled, client: &http.Client{Timeout: 4 * time.Second}}
}

func (m *MilvusService) IndexMessage(ctx context.Context, messageID, roomID, text string) error {
	if !m.enabled {
		return nil
	}
	payload := map[string]any{
		"collectionName": "messages",
		"data": []map[string]any{{
			"id":      messageID,
			"room_id": roomID,
			"vector":  embed(text, 128),
			"text":    text,
		}},
	}
	_, err := m.post(ctx, "/v2/vectordb/entities/insert", payload)
	return err
}

func (m *MilvusService) SemanticSearch(ctx context.Context, query string, roomID *string, limit int) ([]string, error) {
	if !m.enabled {
		return nil, nil
	}
	filter := ""
	if roomID != nil {
		filter = fmt.Sprintf("room_id == \"%s\"", *roomID)
	}
	payload := map[string]any{
		"collectionName": "messages",
		"vector":         embed(query, 128),
		"limit":          limit,
		"outputFields":   []string{"id", "room_id"},
		"filter":         filter,
	}
	resp, err := m.post(ctx, "/v2/vectordb/entities/search", payload)
	if err != nil {
		return nil, err
	}

	items := make([]string, 0)
	for _, row := range resp.Data {
		if idValue, ok := row["id"]; ok {
			switch v := idValue.(type) {
			case float64:
				items = append(items, fmt.Sprintf("%.0f", v))
			case int64:
				items = append(items, fmt.Sprintf("%d", v))
			case string:
				items = append(items, v)
			}
		}
	}
	return items, nil
}

type milvusResponse struct {
	Data []map[string]any `json:"data"`
}

func (m *MilvusService) post(ctx context.Context, path string, payload any) (milvusResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return milvusResponse{}, err
	}
	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint+normalizedPath, bytes.NewBuffer(body))
	if err != nil {
		return milvusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return milvusResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return milvusResponse{}, fmt.Errorf("milvus status %d", resp.StatusCode)
	}
	var out milvusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return milvusResponse{}, err
	}
	return out, nil
}

func embed(text string, dim int) []float32 {
	hash := sha256.Sum256([]byte(text))
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		offset := (i * 4) % len(hash)
		chunk := binary.BigEndian.Uint32(hash[offset : offset+4])
		vec[i] = float32(chunk%1000) / 1000.0
	}
	return vec
}
