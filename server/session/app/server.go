package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/infra/cache"
	orgHub "msg_server/server/orgHub"
	sessionapi "msg_server/server/session/api"
	sessionservice "msg_server/server/session/service"
	tenantHub "msg_server/server/tenantHub"
)

type Config struct {
	Port           string
	RedisAddr      string
	JWTSecret      string
	JWTTTLMinutes  int
	DBManEndpoint  string
	DBManEndpoints []string
}

type Server struct {
	HTTPServer *http.Server
	Redis      *redis.Client
	hub        *sessionservice.Hub
}

func NewServer(cfg Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	redisClient := cache.NewClient(cfg.RedisAddr)
	if err := cache.Ping(ctx, redisClient); err != nil {
		return nil, err
	}

	dbClient := sessionservice.NewDBManClient(cfg.DBManEndpoints...)
	userSvc := orgHub.NewService(dbClient)
	tenantSvc := tenantHub.NewService(dbClient)
	hub := sessionservice.NewHub()
	hub.UseRedis(redisClient)
	_ = hub.StartRedisSubscriber(context.Background())
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

	return &Server{HTTPServer: httpServer, Redis: redisClient, hub: hub}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.hub != nil {
		s.hub.StopRedisSubscriber()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	return s.HTTPServer.Shutdown(ctx)
}
