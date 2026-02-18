package app

import (
	cmnenv "msg_server/server/common/env"
)

type Config struct {
	Env           string
	Port          string
	JWTSecret     string
	JWTTTLMinutes int
	UseMQ         bool

	PostgresDSN string
	RedisAddr   string
	LavinMQURL  string

	DBManEndpoint     string
	DBManEndpoints    []string
	VectormanEndpoint string

	MilvusEndpoint string
	MilvusEnabled  bool
}

func LoadConfig() Config {
	dbmanEndpoints := cmnenv.CSV("DBMAN_ENDPOINTS", []string{cmnenv.String("DBMAN_ENDPOINT", "http://localhost:8082")})
	return Config{
		Env:               cmnenv.String("APP_ENV", "dev"),
		Port:              cmnenv.String("PORT", "8080"),
		JWTSecret:         cmnenv.String("JWT_SECRET", "change-me-in-production"),
		JWTTTLMinutes:     cmnenv.Int("JWT_TTL_MINUTES", 1440),
		UseMQ:             cmnenv.Bool("CHAT_USE_MQ", true),
		PostgresDSN:       cmnenv.String("POSTGRES_DSN", "postgres://msg:msg@localhost:5432/msg?sslmode=disable"),
		RedisAddr:         cmnenv.String("REDIS_ADDR", "localhost:6379"),
		LavinMQURL:        cmnenv.String("LAVINMQ_URL", "amqp://guest:guest@localhost:5672/"),
		DBManEndpoint:     dbmanEndpoints[0],
		DBManEndpoints:    dbmanEndpoints,
		VectormanEndpoint: cmnenv.String("VECTORMAN_ENDPOINT", "http://localhost:8083"),
		MilvusEndpoint:    cmnenv.String("MILVUS_ENDPOINT", "http://localhost:9091"),
		MilvusEnabled:     cmnenv.Bool("MILVUS_ENABLED", true),
	}
}
