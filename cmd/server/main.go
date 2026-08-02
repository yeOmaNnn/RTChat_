package main 

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"github.com/joho/godotenv"

	"RTChat/internal/config"
	"RTChat/internal/httpapi"
	"RTChat/internal/pubsub"
	"RTChat/internal/ratelimit"
	"RTChat/internal/storage"
	"RTChat/internal/ws"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load() 
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() 

	store, err := storage.NewStore(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("подключение к бд: %v", err) 
	}

	defer store.Close() 

	var bus pubsub.Broadcaster
	if cfg.RedisAddr != "" {
		rb, err := pubsub.NewRedis(cfg.RedisAddr) 
		if err != nil {
			log.Fatalf("Подключение к редису: %v", err) 
		}

		defer rb.Close()
		bus = rb 
		log.Printf("pub/sub: используем редис по адресу %s", cfg.RedisAddr) 
	} else {
		bus = pubsub.NewNoOp()
		log.Printf("pub/sub: редис не настроен, работаем в режиме одного инстанста")
	}

	limiter := ratelimit.New(cfg.RateLimitRPS, cfg.RateLimitBurst)
	hub := ws.NewHub(store, limiter, bus)
	server := httpapi.NewServer(hub, store, cfg.HistoryLimit) 

	httpServer := &http.Server {
		Addr : cfg.HTTPAddr,
		Handler: server.Routes(),
	}

	go func() {
		log.Printf("сервер слушает на %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	<-ctx.Done() 
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel() 

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("ошибка при остановке http сервера: %v", err) 
	}

	hub.Shutdown()
	
}