package service

import "context"

type VectorService interface {
	EnsureCollection(ctx context.Context) error
	IndexMessage(ctx context.Context, messageID, roomID, text string) error
	SemanticSearch(ctx context.Context, query string, roomID *string, limit int) ([]string, error)
}
