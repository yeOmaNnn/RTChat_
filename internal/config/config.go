package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct { 
	HTTPAddr 			string 
	PostgresDSN 		string
	RedisAddr 			string 
	RateLimitRPS 		float64
	RateLimitBurst 		int 
	HistoryLimit 		int 
	ShutdownTimeout 	time.Duration
}

func Load() Config {
	return Config {
		HTTPAddr: 			getEnv("HTTP_ADDR", ":8080"),
		PostgresDSN: 		getEnv("POSTGRES_DSN", "postgres://postgres:0000@localhost:5432/chat?sslmode=disable"),
		RedisAddr: 			getEnv("REDIS_ADDR", ""),
		RateLimitRPS: 		getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst: 	getEnvInt("RATE_LIMIT_BURST", 10),
		HistoryLimit: 		getEnvInt("HISTORY_LIMIT", 50),
		ShutdownTimeout: 	time.Duration(getEnvInt("SHUTDOWN_TIMEOUT_SEC", 10)) * time.Second,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def 
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def 
}