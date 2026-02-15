package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"msg_server/server/common/infra/db"
	commondbman "msg_server/server/common/infra/dbman"
	dbapi "msg_server/server/dbman/api"
	"msg_server/server/dbman/repository"
	dbservice "msg_server/server/dbman/service"
)

type Config struct {
	Port          string
	PostgresDSN   string
	DBManEndpoint string
}

type Server struct {
	HTTPServer     *http.Server
	DB             *pgxpool.Pool
	TenantDBRouter *db.TenantDBRouter
}

func NewServer(cfg Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPool, err := db.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}

	var tenantMetaProvider db.TenantMetaProvider
	if strings.TrimSpace(cfg.DBManEndpoint) != "" {
		tenantMetaProvider = commondbman.NewTenantMetaProvider(cfg.DBManEndpoint)
	}
	tenantDBRouter := db.NewTenantDBRouterWithProvider(dbPool, tenantMetaProvider)
	fileRepo := repository.NewFileRepository(tenantDBRouter)
	chatRepo := repository.NewChatRepository(tenantDBRouter)
	userRepo := repository.NewUserRepository(tenantDBRouter)
	sessionRepo := repository.NewSessionRepository(tenantDBRouter)
	tenantRepo := repository.NewTenantRepository(dbPool)
	chatSvc := dbservice.NewChatService(chatRepo)
	userSvc := dbservice.NewUserService(userRepo)
	sessionSvc := dbservice.NewSessionService(sessionRepo)
	tenantSvc := dbservice.NewTenantService(tenantRepo, tenantDBRouter)

	h := dbapi.NewHandler(fileRepo, chatSvc, userSvc, sessionSvc, tenantSvc, dbPool.Ping)
	r := gin.Default()
	h.RegisterRoutes(r)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{HTTPServer: httpServer, DB: dbPool, TenantDBRouter: tenantDBRouter}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.TenantDBRouter != nil {
		s.TenantDBRouter.Close()
	}
	if s.DB != nil {
		s.DB.Close()
	}
	return s.HTTPServer.Shutdown(ctx)
}
