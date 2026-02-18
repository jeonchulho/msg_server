package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type QdrantService struct {
	endpoint   string
	collection string
	enabled    bool
	client     *http.Client
}

func NewQdrantService(endpoint, collection string, enabled bool) *QdrantService {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	normalizedCollection := strings.TrimSpace(collection)
	if normalizedCollection == "" {
		normalizedCollection = "messages"
	}
	return &QdrantService{
		endpoint:   normalizedEndpoint,
		collection: normalizedCollection,
		enabled:    enabled,
		client:     &http.Client{Timeout: 4 * time.Second},
	}
}

func (q *QdrantService) EnsureCollection(ctx context.Context) error {
	if !q.enabled {
		return nil
	}

	statusCode, err := q.statusOnly(ctx, http.MethodGet, fmt.Sprintf("/collections/%s", q.collection))
	if err != nil {
		return err
	}
	if statusCode == http.StatusOK {
		return nil
	}
	if statusCode != http.StatusNotFound {
		return fmt.Errorf("qdrant unexpected status %d while checking collection", statusCode)
	}

	createPayload := map[string]any{
		"vectors": map[string]any{
			"size":     defaultVectorDim,
			"distance": "Cosine",
		},
	}
	_, createStatus, err := q.requestBytes(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", q.collection), createPayload)
	if err != nil {
		return err
	}
	if createStatus != http.StatusOK && createStatus != http.StatusConflict {
		return fmt.Errorf("qdrant status %d while creating collection", createStatus)
	}
	return nil
}

func (q *QdrantService) IndexMessage(ctx context.Context, messageID, roomID, text string) error {
	if !q.enabled {
		return nil
	}
	payload := map[string]any{
		"points": []map[string]any{{
			"id":     messageID,
			"vector": embed(text, defaultVectorDim),
			"payload": map[string]any{
				"id":      messageID,
				"room_id": roomID,
				"text":    text,
			},
		}},
	}
	return q.requestNoDecode(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points", q.collection), payload)
}

func (q *QdrantService) SemanticSearch(ctx context.Context, query string, roomID *string, limit int) ([]string, error) {
	if !q.enabled {
		return nil, nil
	}

	scoredFilter := map[string]any{}
	if roomID != nil {
		scoredFilter["must"] = []map[string]any{{
			"key":   "room_id",
			"match": map[string]any{"value": *roomID},
		}}
	}

	payload := map[string]any{
		"vector":       embed(query, defaultVectorDim),
		"limit":        limit,
		"with_payload": true,
	}
	if len(scoredFilter) > 0 {
		payload["filter"] = scoredFilter
	}

	resp, err := q.postSearch(ctx, fmt.Sprintf("/collections/%s/points/search", q.collection), payload)
	if err != nil {
		return nil, err
	}

	items := make([]string, 0, len(resp.Result))
	for _, row := range resp.Result {
		if payloadValue, ok := row.Payload["id"]; ok {
			if id, ok := payloadValue.(string); ok && id != "" {
				items = append(items, id)
				continue
			}
		}
		switch id := row.ID.(type) {
		case string:
			if id != "" {
				items = append(items, id)
			}
		case float64:
			items = append(items, fmt.Sprintf("%.0f", id))
		case int64:
			items = append(items, fmt.Sprintf("%d", id))
		}
	}
	return items, nil
}

type qdrantSearchResponse struct {
	Result []struct {
		ID      any            `json:"id"`
		Payload map[string]any `json:"payload"`
	} `json:"result"`
}

func (q *QdrantService) postSearch(ctx context.Context, path string, payload any) (qdrantSearchResponse, error) {
	body, statusCode, err := q.requestBytes(ctx, http.MethodPost, path, payload)
	if err != nil {
		return qdrantSearchResponse{}, err
	}
	if statusCode >= 300 {
		return qdrantSearchResponse{}, fmt.Errorf("qdrant status %d", statusCode)
	}
	var out qdrantSearchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return qdrantSearchResponse{}, err
	}
	return out, nil
}

func (q *QdrantService) statusOnly(ctx context.Context, method, path string) (int, error) {
	_, statusCode, err := q.requestBytes(ctx, method, path, nil)
	if err != nil {
		return 0, err
	}
	return statusCode, nil
}

func (q *QdrantService) requestNoDecode(ctx context.Context, method, path string, payload any) error {
	_, statusCode, err := q.requestBytes(ctx, method, path, payload)
	if err != nil {
		return err
	}
	if statusCode >= 300 {
		return fmt.Errorf("qdrant status %d", statusCode)
	}
	return nil
}

func (q *QdrantService) requestBytes(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
	} else {
		bodyBytes = []byte{}
	}

	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	req, err := http.NewRequestWithContext(ctx, method, q.endpoint+normalizedPath, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return responseBody, resp.StatusCode, nil
}
