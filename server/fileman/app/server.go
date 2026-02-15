package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/infra/object"
	fileapi "msg_server/server/fileman/api"
	"msg_server/server/fileman/service"
)

type Config struct {
	Port          string
	JWTSecret     string
	JWTTTLMinutes int

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	DBManEndpoint  string
	DBManEndpoints []string
}

type Server struct {
	HTTPServer *http.Server
}

func NewServer(cfg Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	minioClient, err := object.NewClient(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		return nil, fmt.Errorf("initialize minio: %w", err)
	}
	if err := object.EnsureBucket(ctx, minioClient, cfg.MinioBucket); err != nil {
		return nil, fmt.Errorf("ensure minio bucket: %w", err)
	}

	dbmanClient := service.NewDBManClient(cfg.DBManEndpoints...)
	tenantMinIORouter := object.NewTenantMinIORouter(minioClient, cfg.MinioBucket, dbmanClient)
	fileSvc := service.NewFileService(dbmanClient, tenantMinIORouter)
	authSvc := commonauth.NewService(cfg.JWTSecret, cfg.JWTTTLMinutes)

	h := fileapi.NewHandler(fileSvc, authSvc)
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
