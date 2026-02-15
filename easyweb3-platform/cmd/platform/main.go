package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/cache"
	"github.com/nicekwell/easyweb3-platform/internal/config"
	"github.com/nicekwell/easyweb3-platform/internal/gateway"
	"github.com/nicekwell/easyweb3-platform/internal/integration"
	"github.com/nicekwell/easyweb3-platform/internal/logging"
	"github.com/nicekwell/easyweb3-platform/internal/notification"
	"github.com/nicekwell/easyweb3-platform/internal/publicdocs"
	"github.com/nicekwell/easyweb3-platform/internal/service"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	jwt := auth.JWT{Secret: cfg.JWTSecret, TokenTTL: cfg.TokenTTL}
	ks := auth.NewFileKeyStore(cfg.APIKeysFile, os.Getenv("EASYWEB3_BOOTSTRAP_ADMIN_API_KEY"))
	if err := ks.Load(); err != nil {
		log.Fatalf("api key store: %v", err)
	}
	us := auth.NewFileUserStore(cfg.UsersFile)
	if err := us.Load(); err != nil {
		log.Fatalf("user store: %v", err)
	}

	logsStore := logging.NewFileStore(cfg.LogsFile)
	logsHandler := &logging.Handler{Store: logsStore}

	notifyStore := notification.NewFileStore(cfg.NotifyFile)
	if err := notifyStore.Load(); err != nil {
		log.Fatalf("notify store: %v", err)
	}
	notifyHandler := notification.Handler{
		Store:   notifyStore,
		Webhook: notification.WebhookSender{},
		TG:      notification.TelegramSender{},
	}

	integrationHandler := integration.Handler{
		Dex:        integration.Dexscreener{BaseURL: cfg.DexscreenerBaseURL, TTL: cfg.CacheDefaultTTL},
		GoPlus:     integration.GoPlus{BaseURL: cfg.GoPlusBaseURL, APIKey: cfg.GoPlusAPIKey, TTL: cfg.CacheDefaultTTL},
		Polymarket: integration.Polymarket{BaseURL: cfg.Services["polymarket"].BaseURL, TTL: cfg.CacheDefaultTTL},
	}

	var cacheStore cache.Store
	switch cfg.CacheBackend {
	case "", "memory":
		cacheStore = cache.NewMemoryStore()
	case "redis":
		if cfg.RedisAddr == "" {
			log.Printf("EASYWEB3_CACHE_BACKEND=redis but EASYWEB3_REDIS_ADDR is empty; falling back to memory")
			cacheStore = cache.NewMemoryStore()
		} else {
			cacheStore = cache.NewRedisStore(&redis.Options{
				Addr:     cfg.RedisAddr,
				Password: cfg.RedisPassword,
				DB:       cfg.RedisDB,
			})
		}
	default:
		log.Printf("unknown EASYWEB3_CACHE_BACKEND=%q, falling back to memory", cfg.CacheBackend)
		cacheStore = cache.NewMemoryStore()
	}
	cacheHandler := cache.Handler{Store: cacheStore, DefaultTTL: cfg.CacheDefaultTTL}

	// Wire cache into integration (best-effort).
	integrationHandler.Dex.Cache = cacheStore
	integrationHandler.GoPlus.Cache = cacheStore
	integrationHandler.Polymarket.Cache = cacheStore

	proxy := gateway.NewProxy(cfg.Services)

	authHandler := auth.Handler{Keys: ks, Users: us, JWT: jwt}
	serviceHandler := service.Handler{Services: cfg.Services}

	router := gateway.Router{
		Auth:         authHandler,
		Logs:         logsHandler,
		Notify:       notifyHandler,
		Integrations: integrationHandler,
		Cache:        cacheHandler,
		Service:      serviceHandler,
		Proxy:        proxy,
		Docs:         publicdocs.Handler{Dir: cfg.DocsDir},
		AuthMW:       auth.Middleware(jwt),
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("easyweb3-platform listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Printf("shutdown complete")
}
