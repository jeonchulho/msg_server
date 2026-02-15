package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	commonauth "msg_server/server/common/auth"
	sessionapi "msg_server/server/session/api"
	sessionservice "msg_server/server/session/service"
)

type Config struct {
	Port           string
	JWTSecret      string
	JWTTTLMinutes  int
	DBManEndpoint  string
	DBManEndpoints []string
}

type Server struct {
	HTTPServer *http.Server
}

func NewServer(cfg Config) (*Server, error) {
	dbClient := sessionservice.NewDBManClient(cfg.DBManEndpoints...)
	userSvc := sessionservice.NewUserService(dbClient)
	tenantSvc := sessionservice.NewTenantService(dbClient)
	hub := sessionservice.NewHub()
	svc := sessionservice.NewService(dbClient, hub)
	auth := commonauth.NewService(cfg.JWTSecret, cfg.JWTTTLMinutes)
	h := sessionapi.NewHandler(userSvc, tenantSvc, svc, auth, hub)

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
