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
	filemanapp "msg_server/server/fileman/app"
)

func main() {
	port := os.Getenv("FILEMAN_PORT")
	if port == "" {
		port = "8081"
	}
	dbmanEndpoints := cmnenv.CSV("DBMAN_ENDPOINTS", []string{cmnenv.String("DBMAN_ENDPOINT", "http://localhost:8082")})

	server, err := filemanapp.NewServer(filemanapp.Config{
		Port:           port,
		JWTSecret:      cmnenv.String("JWT_SECRET", "change-me-in-production"),
		JWTTTLMinutes:  cmnenv.Int("JWT_TTL_MINUTES", 1440),
		MinioEndpoint:  cmnenv.String("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: cmnenv.String("MINIO_ACCESS_KEY", "minio"),
		MinioSecretKey: cmnenv.String("MINIO_SECRET_KEY", "minio123"),
		MinioBucket:    cmnenv.String("MINIO_BUCKET", "chat-files"),
		MinioUseSSL:    cmnenv.Bool("MINIO_USE_SSL", false),
		DBManEndpoint:  dbmanEndpoints[0],
		DBManEndpoints: dbmanEndpoints,
	})
	if err != nil {
		log.Fatalf("initialize fileman server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		commonlog.Infof("start fileman http server on :%s", port)
		if err := server.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run fileman http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		commonlog.Errorf("shutdown fileman server gracefully: %v", err)
	}
}
