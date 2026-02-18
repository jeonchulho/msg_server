package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	vectorapi "msg_server/server/vectorman/api"
	"msg_server/server/vectorman/service"
)

type Config struct {
	Port                  string
	VectorBackend         string
	MilvusEnabled         bool
	MilvusEndpoint        string
	QdrantEnabled         bool
	QdrantEndpoint        string
	QdrantCollection      string
	ElasticsearchEnabled  bool
	ElasticsearchEndpoint string
	ElasticsearchIndex    string
}

type Server struct {
	HTTPServer *http.Server
}

func NewServer(cfg Config) (*Server, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.VectorBackend))
	if backend == "" {
		backend = "milvus"
	}

	var svc service.VectorService
	switch backend {
	case "milvus":
		svc = service.NewMilvusService(cfg.MilvusEndpoint, cfg.MilvusEnabled)
	case "qdrant":
		svc = service.NewQdrantService(cfg.QdrantEndpoint, cfg.QdrantCollection, cfg.QdrantEnabled)
	case "elasticsearch":
		svc = service.NewElasticsearchService(cfg.ElasticsearchEndpoint, cfg.ElasticsearchIndex, cfg.ElasticsearchEnabled)
	default:
		return nil, fmt.Errorf("unsupported vector backend: %s", cfg.VectorBackend)
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := svc.EnsureCollection(initCtx); err != nil {
		return nil, fmt.Errorf("ensure vector collection: %w", err)
	}

	h := vectorapi.NewHandler(svc)
	r := gin.Default()
	h.RegisterRoutes(r)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return &Server{HTTPServer: httpServer}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.HTTPServer.Shutdown(ctx)
}
