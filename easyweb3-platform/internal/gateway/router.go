package gateway

import (
	"net/http"
	"strings"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/cache"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
	"github.com/nicekwell/easyweb3-platform/internal/integration"
	"github.com/nicekwell/easyweb3-platform/internal/logging"
	"github.com/nicekwell/easyweb3-platform/internal/notification"
	"github.com/nicekwell/easyweb3-platform/internal/service"
)

type Router struct {
	Auth         auth.Handler
	Logs         *logging.Handler
	Notify       notification.Handler
	Integrations integration.Handler
	Cache        cache.Handler
	Service      service.Handler
	Proxy        *Proxy

	AuthMW func(http.Handler) http.Handler
}

func (rt Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health.
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}

	// Auth endpoints.
	if r.URL.Path == "/api/v1/auth/login" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.Auth.Login(w, r)
		return
	}
	if r.URL.Path == "/api/v1/auth/status" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.Auth.Status(w, r)
		return
	}
	if r.URL.Path == "/api/v1/auth/refresh" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Auth.Refresh)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/auth/keys" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Auth.CreateKey)).ServeHTTP(w, r)
		return
	}

	// Logging.
	if r.URL.Path == "/api/v1/logs" {
		switch r.Method {
		case http.MethodPost:
			rt.requireAuth(http.HandlerFunc(rt.Logs.Create)).ServeHTTP(w, r)
			return
		case http.MethodGet:
			rt.requireAuth(http.HandlerFunc(rt.Logs.List)).ServeHTTP(w, r)
			return
		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}
	if r.URL.Path == "/api/v1/logs/stats" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Logs.Stats)).ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/logs/") {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/logs/")
		if id == "" {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		rt.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Logs.Get(w, r, id)
		})).ServeHTTP(w, r)
		return
	}

	// Notification.
	if r.URL.Path == "/api/v1/notify/send" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Notify.Send)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/notify/broadcast" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Notify.Broadcast)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/notify/config" {
		switch r.Method {
		case http.MethodGet:
			rt.requireAuth(http.HandlerFunc(rt.Notify.GetConfig)).ServeHTTP(w, r)
			return
		case http.MethodPut:
			rt.requireAuth(http.HandlerFunc(rt.Notify.PutConfig)).ServeHTTP(w, r)
			return
		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	// Integrations.
	if strings.HasPrefix(r.URL.Path, "/api/v1/integrations/") {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		provider, ok := parseIntegrationProvider(r.URL.Path)
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		rt.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Integrations.Query(w, r, provider)
		})).ServeHTTP(w, r)
		return
	}

	// Cache.
	if strings.HasPrefix(r.URL.Path, "/api/v1/cache/") {
		if r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/api/v1/cache/")
		key = strings.TrimSpace(key)
		if key == "" || strings.Contains(key, "/") {
			httpx.WriteError(w, http.StatusBadRequest, "invalid key")
			return
		}
		rt.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				rt.Cache.Get(w, r, key)
			case http.MethodPut:
				rt.Cache.Put(w, r, key)
			case http.MethodDelete:
				rt.Cache.Delete(w, r, key)
			}
		})).ServeHTTP(w, r)
		return
	}

	// Service management endpoints for CLI.
	if r.URL.Path == "/api/v1/service/list" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Service.List)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/service/health" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			httpx.WriteError(w, http.StatusBadRequest, "name required")
			return
		}
		rt.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Service.Health(w, r, name)
		})).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/service/docs" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			httpx.WriteError(w, http.StatusBadRequest, "name required")
			return
		}
		rt.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Service.Docs(w, r, name)
		})).ServeHTTP(w, r)
		return
	}

	// Proxy business services.
	if strings.HasPrefix(r.URL.Path, "/api/v1/services/") {
		rt.requireAuth(rt.Proxy).ServeHTTP(w, r)
		return
	}

	httpx.WriteError(w, http.StatusNotFound, "not found")
}

func (rt Router) requireAuth(h http.Handler) http.Handler {
	if rt.AuthMW == nil {
		return h
	}
	return rt.AuthMW(h)
}

func parseIntegrationProvider(path string) (provider string, ok bool) {
	// /api/v1/integrations/{provider}/query
	const prefix = "/api/v1/integrations/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", false
	}
	if strings.TrimSpace(parts[0]) == "" {
		return "", false
	}
	if parts[1] != "query" {
		return "", false
	}
	return parts[0], true
}
