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

type MilvusService struct {
	endpoint string
	enabled  bool
	client   *http.Client
}

const (
	defaultCollectionName = "messages"
	defaultVectorDim      = 128
)

func NewMilvusService(endpoint string, enabled bool) *MilvusService {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	return &MilvusService{endpoint: normalizedEndpoint, enabled: enabled, client: &http.Client{Timeout: 4 * time.Second}}
}

func (m *MilvusService) EnsureCollection(ctx context.Context) error {
	if !m.enabled {
		return nil
	}

	hasPayload := map[string]any{"collectionName": defaultCollectionName}
	hasResp, err := m.postRaw(ctx, "/v2/vectordb/collections/has", hasPayload)
	if err == nil {
		if hasData, ok := hasResp["data"].(map[string]any); ok {
			if has, ok := hasData["has"].(bool); ok && has {
				return nil
			}
		}
	}

	createPayload := map[string]any{
		"collectionName": defaultCollectionName,
		"schema": map[string]any{
			"autoId":             false,
			"enableDynamicField": true,
			"fields": []map[string]any{
				{"fieldName": "id", "dataType": "VarChar", "isPrimary": true, "maxLength": 128},
				{"fieldName": "room_id", "dataType": "VarChar", "maxLength": 128},
				{"fieldName": "text", "dataType": "VarChar", "maxLength": 8192},
				{"fieldName": "vector", "dataType": "FloatVector", "elementTypeParams": map[string]any{"dim": fmt.Sprintf("%d", defaultVectorDim)}},
			},
		},
	}

	if _, err := m.postRaw(ctx, "/v2/vectordb/collections/create", createPayload); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already") || strings.Contains(strings.ToLower(err.Error()), "exist") {
			return nil
		}
		return err
	}
	return nil
}

func (m *MilvusService) IndexMessage(ctx context.Context, messageID, roomID, text string) error {
	if !m.enabled {
		return nil
	}
	payload := map[string]any{
		"collectionName": defaultCollectionName,
		"data": []map[string]any{{
			"id":      messageID,
			"room_id": roomID,
			"vector":  embed(text, defaultVectorDim),
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
		"collectionName": defaultCollectionName,
		"vector":         embed(query, defaultVectorDim),
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
	raw, err := m.postRaw(ctx, path, payload)
	if err != nil {
		return milvusResponse{}, err
	}
	var out milvusResponse
	if dataRows, ok := raw["data"].([]any); ok {
		out.Data = make([]map[string]any, 0, len(dataRows))
		for _, row := range dataRows {
			if mapped, ok := row.(map[string]any); ok {
				out.Data = append(out.Data, mapped)
			}
		}
	}
	return out, nil
}

func (m *MilvusService) postRaw(ctx context.Context, path string, payload any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint+normalizedPath, bytes.NewBuffer(body))
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
		return nil, fmt.Errorf("milvus status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
