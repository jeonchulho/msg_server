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
	dbmanapp "msg_server/server/dbman/app"
)

func main() {
	port := os.Getenv("DBMAN_PORT")
	if port == "" {
		port = "8082"
	}

	server, err := dbmanapp.NewServer(dbmanapp.Config{
		Port:          port,
		PostgresDSN:   cmnenv.String("POSTGRES_DSN", "postgres://msg:msg@localhost:5432/msg?sslmode=disable"),
		DBManEndpoint: cmnenv.String("DBMAN_ENDPOINT", "http://localhost:8082"),
	})
	if err != nil {
		log.Fatalf("initialize dbman server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("start dbman http server on :%s", port)
		if err := server.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run dbman http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown dbman server gracefully: %v", err)
	}
}
