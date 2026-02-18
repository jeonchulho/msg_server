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
	vectormanapp "msg_server/server/vectorman/app"
)

func main() {
	port := os.Getenv("VECTORMAN_PORT")
	if port == "" {
		port = "8083"
	}

	server, err := vectormanapp.NewServer(vectormanapp.Config{
		Port:                  port,
		VectorBackend:         cmnenv.String("VECTOR_BACKEND", "milvus"),
		MilvusEnabled:         cmnenv.Bool("MILVUS_ENABLED", true),
		MilvusEndpoint:        cmnenv.String("MILVUS_ENDPOINT", "http://localhost:9091"),
		QdrantEnabled:         cmnenv.Bool("QDRANT_ENABLED", true),
		QdrantEndpoint:        cmnenv.String("QDRANT_ENDPOINT", "http://localhost:6333"),
		QdrantCollection:      cmnenv.String("QDRANT_COLLECTION", "messages"),
		ElasticsearchEnabled:  cmnenv.Bool("ELASTICSEARCH_ENABLED", true),
		ElasticsearchEndpoint: cmnenv.String("ELASTICSEARCH_ENDPOINT", "http://localhost:9200"),
		ElasticsearchIndex:    cmnenv.String("ELASTICSEARCH_INDEX", "messages"),
	})
	if err != nil {
		log.Fatalf("initialize vectorman server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		commonlog.Infof("start vectorman http server on :%s", port)
		if err := server.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run vectorman http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		commonlog.Errorf("shutdown vectorman server gracefully: %v", err)
	}
}
