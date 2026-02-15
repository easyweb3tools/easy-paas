package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServiceConfig struct {
	// BaseURL is the upstream base URL, e.g. http://localhost:3000
	BaseURL string `json:"base_url"`
	// HealthPath is appended to BaseURL when checking service health.
	// Defaults to /health.
	HealthPath string `json:"health_path"`
	// DocsPath is appended to BaseURL when fetching docs (optional).
	DocsPath string `json:"docs_path"`
}

type Config struct {
	ListenAddr string
	JWTSecret  []byte
	TokenTTL   time.Duration

	APIKeysFile string
	UsersFile   string
	LogsFile    string
	NotifyFile  string
	DocsDir     string

	DexscreenerBaseURL string
	GoPlusBaseURL      string
	GoPlusAPIKey       string
	CacheBackend       string
	CacheDefaultTTL    time.Duration
	RedisAddr          string
	RedisPassword      string
	RedisDB            int

	Services map[string]ServiceConfig
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:         getenv("EASYWEB3_LISTEN", ":8080"),
		JWTSecret:          []byte(getenv("EASYWEB3_JWT_SECRET", "dev-secret-change-me")),
		TokenTTL:           mustDuration(getenv("EASYWEB3_TOKEN_TTL", "24h")),
		APIKeysFile:        getenv("EASYWEB3_API_KEYS_FILE", "./data/api_keys.json"),
		UsersFile:          getenv("EASYWEB3_USERS_FILE", "./data/users.json"),
		LogsFile:           getenv("EASYWEB3_LOGS_FILE", "./data/logs.jsonl"),
		NotifyFile:         getenv("EASYWEB3_NOTIFY_FILE", "./data/notify_config.json"),
		DocsDir:            strings.TrimSpace(getenv("EASYWEB3_DOCS_DIR", "")),
		DexscreenerBaseURL: getenv("EASYWEB3_DEXSCREENER_BASE_URL", "https://api.dexscreener.com"),
		GoPlusBaseURL:      getenv("EASYWEB3_GOPLUS_BASE_URL", "https://api.gopluslabs.io"),
		GoPlusAPIKey:       getenv("EASYWEB3_GOPLUS_API_KEY", ""),
		CacheBackend:       strings.ToLower(strings.TrimSpace(getenv("EASYWEB3_CACHE_BACKEND", "memory"))),
		CacheDefaultTTL:    mustDuration(getenv("EASYWEB3_CACHE_DEFAULT_TTL", "30s")),
		RedisAddr:          strings.TrimSpace(getenv("EASYWEB3_REDIS_ADDR", "")),
		RedisPassword:      getenv("EASYWEB3_REDIS_PASSWORD", ""),
		RedisDB:            mustInt(getenv("EASYWEB3_REDIS_DB", "0"), 0),
		Services:           map[string]ServiceConfig{},
	}

	if len(cfg.JWTSecret) < 16 {
		return Config{}, errors.New("EASYWEB3_JWT_SECRET must be at least 16 bytes")
	}

	// Optional JSON blob for services.
	// Example:
	//  {"meme":{"base_url":"http://localhost:8081"},"story":{"base_url":"http://localhost:3000"}}
	if raw := strings.TrimSpace(os.Getenv("EASYWEB3_SERVICES_JSON")); raw != "" {
		var m map[string]ServiceConfig
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return Config{}, fmt.Errorf("parse EASYWEB3_SERVICES_JSON: %w", err)
		}
		for name, sc := range m {
			sc = normalizeService(sc)
			cfg.Services[name] = sc
		}
	}

	// Optional file for services.
	if path := strings.TrimSpace(os.Getenv("EASYWEB3_SERVICES_FILE")); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read EASYWEB3_SERVICES_FILE: %w", err)
		}
		var m map[string]ServiceConfig
		if err := json.Unmarshal(b, &m); err != nil {
			return Config{}, fmt.Errorf("parse EASYWEB3_SERVICES_FILE: %w", err)
		}
		for name, sc := range m {
			sc = normalizeService(sc)
			cfg.Services[name] = sc
		}
	}

	// Convenience envs.
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_SERVICE_MEME_BASE_URL")); v != "" {
		sc := cfg.Services["meme"]
		sc.BaseURL = v
		sc = normalizeService(sc)
		cfg.Services["meme"] = sc
	}
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_SERVICE_STORY_BASE_URL")); v != "" {
		sc := cfg.Services["story"]
		sc.BaseURL = v
		sc = normalizeService(sc)
		cfg.Services["story"] = sc
	}

	return cfg, nil
}

func normalizeService(sc ServiceConfig) ServiceConfig {
	sc.BaseURL = strings.TrimRight(strings.TrimSpace(sc.BaseURL), "/")
	if sc.HealthPath == "" {
		sc.HealthPath = "/health"
	}
	if !strings.HasPrefix(sc.HealthPath, "/") {
		sc.HealthPath = "/" + sc.HealthPath
	}
	if sc.DocsPath != "" && !strings.HasPrefix(sc.DocsPath, "/") {
		sc.DocsPath = "/" + sc.DocsPath
	}
	return sc
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err == nil {
		return d
	}
	// Support seconds-only integer.
	if n, err2 := strconv.Atoi(v); err2 == nil {
		return time.Duration(n) * time.Second
	}
	return 24 * time.Hour
}

func mustInt(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
