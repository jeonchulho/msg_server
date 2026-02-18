package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"msg_server/server/chat/api"
	"msg_server/server/chat/service"
	"msg_server/server/common/infra/cache"
	"msg_server/server/common/infra/mq"
)

type Server struct {
	HTTPServer        *http.Server
	Redis             *redis.Client
	MQConn            *amqp.Connection
	TenantRedisRouter *cache.TenantRedisRouter
	TenantMQPublisher *service.AMQPPublisher
}

func NewServer(cfg Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisClient := cache.NewClient(cfg.RedisAddr)
	if err := cache.Ping(ctx, redisClient); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	dbClient := service.NewDBManClient(cfg.DBManEndpoints...)
	tenantRedisRouter := cache.NewTenantRedisRouter(redisClient, dbClient)
	var err error

	var (
		mqConn            *amqp.Connection
		tenantMQPublisher *service.AMQPPublisher
	)
	if cfg.UseMQ {
		mqConn, err = mq.NewConnection(cfg.LavinMQURL)
		if err != nil {
			return nil, fmt.Errorf("initialize lavinmq: %w", err)
		}

		tenantMQPublisher, err = service.NewAMQPPublisher(mqConn, dbClient)
		if err != nil {
			return nil, fmt.Errorf("initialize amqp publisher: %w", err)
		}
	}

	vectorClient := service.NewVectormanClient(cfg.VectormanEndpoint, cfg.MilvusEnabled)
	chatSvc := service.NewChatService(tenantMQPublisher, dbClient, vectorClient, cfg.UseMQ)
	wsSvc := service.NewRealtimeService(tenantRedisRouter, chatSvc)

	h := api.NewHandler(chatSvc, wsSvc, cfg.JWTSecret, cfg.JWTTTLMinutes)
	r := gin.Default()
	h.RegisterRoutes(r)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		HTTPServer:        httpServer,
		Redis:             redisClient,
		MQConn:            mqConn,
		TenantRedisRouter: tenantRedisRouter,
		TenantMQPublisher: tenantMQPublisher,
	}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.TenantMQPublisher != nil {
		s.TenantMQPublisher.Close()
	}
	if s.TenantRedisRouter != nil {
		s.TenantRedisRouter.Close()
	}
	if s.MQConn != nil {
		_ = s.MQConn.Close()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	return s.HTTPServer.Shutdown(ctx)
}
