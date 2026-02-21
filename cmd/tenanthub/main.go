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
	tenantapp "msg_server/server/tenantHub/app"
)

func main() {
	port := cmnenv.String("TENANTHUB_PORT", "8092")
	dbmanEndpoints := cmnenv.CSV("DBMAN_ENDPOINTS", []string{cmnenv.String("DBMAN_ENDPOINT", "http://localhost:8082")})

	server, err := tenantapp.NewServer(tenantapp.Config{
		Port:           port,
		JWTSecret:      cmnenv.String("JWT_SECRET", "change-me-in-production"),
		JWTTTLMinutes:  cmnenv.Int("JWT_TTL_MINUTES", 1440),
		DBManEndpoints: dbmanEndpoints,
	})
	if err != nil {
		log.Fatalf("initialize tenanthub server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		commonlog.Infof("start tenanthub http server on :%s", port)
		if err := server.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run tenanthub http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		commonlog.Errorf("shutdown tenanthub server gracefully: %v", err)
	}

	_ = os.Stdout.Sync()
}
