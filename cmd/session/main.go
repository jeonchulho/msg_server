package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cmnenv "msg_server/server/common/env"
	commonlog "msg_server/server/common/log"
	sessionapp "msg_server/server/session/app"
)

func main() {
	port := os.Getenv("SESSION_PORT")
	if port == "" {
		port = os.Getenv("SESSIOND_PORT")
	}
	if port == "" {
		port = "8090"
	}
	dbmanEndpoints := cmnenv.CSV("DBMAN_ENDPOINTS", []string{cmnenv.String("DBMAN_ENDPOINT", "http://localhost:8082")})

	sessionServer, err := sessionapp.NewServer(sessionapp.Config{
		Port:           port,
		RedisAddr:      cmnenv.String("REDIS_ADDR", "localhost:6379"),
		JWTSecret:      cmnenv.String("JWT_SECRET", "change-me-in-production"),
		JWTTTLMinutes:  cmnenv.Int("JWT_TTL_MINUTES", 1440),
		DBManEndpoint:  dbmanEndpoints[0],
		DBManEndpoints: dbmanEndpoints,
	})
	if err != nil {
		log.Fatalf("initialize session server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		commonlog.Infof("start session http server on :%s", port)
		if err := sessionServer.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run session http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sessionServer.Shutdown(shutdownCtx); err != nil {
		commonlog.Errorf("shutdown session server gracefully: %v", err)
	}
}
