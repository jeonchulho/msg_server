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

type ElasticsearchService struct {
	endpoint string
	index    string
	enabled  bool
	client   *http.Client
}

func NewElasticsearchService(endpoint, index string, enabled bool) *ElasticsearchService {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	normalizedIndex := strings.TrimSpace(index)
	if normalizedIndex == "" {
		normalizedIndex = defaultCollectionName
	}
	return &ElasticsearchService{
		endpoint: normalizedEndpoint,
		index:    normalizedIndex,
		enabled:  enabled,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (e *ElasticsearchService) EnsureCollection(ctx context.Context) error {
	if !e.enabled {
		return nil
	}

	_, statusCode, err := e.requestBytes(ctx, http.MethodHead, "/"+e.index, nil)
	if err != nil {
		return err
	}
	if statusCode == http.StatusOK {
		return nil
	}
	if statusCode != http.StatusNotFound {
		return fmt.Errorf("elasticsearch unexpected status %d while checking index", statusCode)
	}

	createPayload := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"id":      map[string]any{"type": "keyword"},
				"room_id": map[string]any{"type": "keyword"},
				"text":    map[string]any{"type": "text"},
				"vector":  map[string]any{"type": "dense_vector", "dims": defaultVectorDim, "index": true, "similarity": "cosine"},
			},
		},
	}

	_, createStatus, err := e.requestBytes(ctx, http.MethodPut, "/"+e.index, createPayload)
	if err != nil {
		return err
	}
	if createStatus != http.StatusOK && createStatus != http.StatusCreated && createStatus != http.StatusBadRequest {
		return fmt.Errorf("elasticsearch status %d while creating index", createStatus)
	}
	return nil
}

func (e *ElasticsearchService) IndexMessage(ctx context.Context, messageID, roomID, text string) error {
	if !e.enabled {
		return nil
	}

	payload := map[string]any{
		"id":      messageID,
		"room_id": roomID,
		"text":    text,
		"vector":  embed(text, defaultVectorDim),
	}

	path := fmt.Sprintf("/%s/_doc/%s", e.index, messageID)
	_, statusCode, err := e.requestBytes(ctx, http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	if statusCode >= 300 {
		return fmt.Errorf("elasticsearch status %d", statusCode)
	}
	return nil
}

func (e *ElasticsearchService) SemanticSearch(ctx context.Context, query string, roomID *string, limit int) ([]string, error) {
	if !e.enabled {
		return nil, nil
	}

	innerQuery := map[string]any{"match_all": map[string]any{}}
	if roomID != nil {
		innerQuery = map[string]any{
			"bool": map[string]any{
				"filter": []map[string]any{{
					"term": map[string]any{"room_id": *roomID},
				}},
			},
		}
	}

	payload := map[string]any{
		"size":    limit,
		"_source": []string{"id", "room_id"},
		"query": map[string]any{
			"script_score": map[string]any{
				"query": innerQuery,
				"script": map[string]any{
					"source": "cosineSimilarity(params.query_vector, 'vector') + 1.0",
					"params": map[string]any{"query_vector": embed(query, defaultVectorDim)},
				},
			},
		},
	}

	body, statusCode, err := e.requestBytes(ctx, http.MethodPost, fmt.Sprintf("/%s/_search", e.index), payload)
	if err != nil {
		return nil, err
	}
	if statusCode >= 300 {
		return nil, fmt.Errorf("elasticsearch status %d", statusCode)
	}

	var resp elasticSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	items := make([]string, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		if hit.Source.ID != "" {
			items = append(items, hit.Source.ID)
			continue
		}
		if hit.ID != "" {
			items = append(items, hit.ID)
		}
	}
	return items, nil
}

type elasticSearchResponse struct {
	Hits struct {
		Hits []struct {
			ID     string `json:"_id"`
			Source struct {
				ID string `json:"id"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func (e *ElasticsearchService) requestBytes(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
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
	req, err := http.NewRequestWithContext(ctx, method, e.endpoint+normalizedPath, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
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
