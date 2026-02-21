package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	commonauth "msg_server/server/common/auth"
	orgapi "msg_server/server/orgHub/api"
	orgHub "msg_server/server/orgHub/service"
)

type Config struct {
	Port           string
	JWTSecret      string
	JWTTTLMinutes  int
	DBManEndpoints []string
}

type Server struct {
	HTTPServer *http.Server
}

func NewServer(cfg Config) (*Server, error) {
	dbClient := orgHub.NewDBManClient(cfg.DBManEndpoints...)
	userSvc := orgHub.NewService(dbClient)
	auth := commonauth.NewService(cfg.JWTSecret, cfg.JWTTTLMinutes)
	h := orgapi.NewHandler(userSvc, auth)

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
