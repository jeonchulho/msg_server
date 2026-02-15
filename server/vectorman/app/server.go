package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	vectorapi "msg_server/server/vectorman/api"
	"msg_server/server/vectorman/service"
)

type Config struct {
	Port           string
	MilvusEnabled  bool
	MilvusEndpoint string
}

type Server struct {
	HTTPServer *http.Server
}

func NewServer(cfg Config) (*Server, error) {
	svc := service.NewMilvusService(cfg.MilvusEndpoint, cfg.MilvusEnabled)
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
