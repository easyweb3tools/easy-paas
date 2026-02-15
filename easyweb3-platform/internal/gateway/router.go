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
	"github.com/nicekwell/easyweb3-platform/internal/publicdocs"
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
	Docs         publicdocs.Handler

	AuthMW func(http.Handler) http.Handler
}

func (rt Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health.
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}

	// Public docs (no auth).
	if r.URL.Path == "/docs" || r.URL.Path == "/docs/" {
		rt.Docs.Index(w, r)
		return
	}
	if r.URL.Path == "/docs/architecture" || r.URL.Path == "/docs/architecture/" {
		rt.Docs.Architecture(w, r)
		return
	}
	if r.URL.Path == "/docs/openclaw" || r.URL.Path == "/docs/openclaw/" {
		rt.Docs.OpenClaw(w, r)
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
	if r.URL.Path == "/api/v1/auth/register" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.Auth.Register(w, r)
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
	if r.URL.Path == "/api/v1/auth/grants" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Auth.Grant)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/auth/users" {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(http.HandlerFunc(rt.Auth.ListUsers)).ServeHTTP(w, r)
		return
	}

	// Logging.
	if r.URL.Path == "/api/v1/logs" {
		switch r.Method {
		case http.MethodPost:
			rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Logs.Create), "agent", "admin")).ServeHTTP(w, r)
			return
		case http.MethodGet:
			rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Logs.List), "viewer", "agent", "admin")).ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Logs.Stats), "viewer", "agent", "admin")).ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Logs.Get(w, r, id)
		}), "viewer", "agent", "admin")).ServeHTTP(w, r)
		return
	}

	// Notification.
	if r.URL.Path == "/api/v1/notify/send" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Notify.Send), "agent", "admin")).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/notify/broadcast" {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Notify.Broadcast), "agent", "admin")).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/notify/config" {
		switch r.Method {
		case http.MethodGet:
			rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Notify.GetConfig), "agent", "admin")).ServeHTTP(w, r)
			return
		case http.MethodPut:
			rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Notify.PutConfig), "agent", "admin")).ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Integrations.Query(w, r, provider)
		}), "viewer", "agent", "admin")).ServeHTTP(w, r)
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
				rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rt.Cache.Get(w, r, key)
				}), "viewer", "agent", "admin").ServeHTTP(w, r)
			case http.MethodPut:
				rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rt.Cache.Put(w, r, key)
				}), "agent", "admin").ServeHTTP(w, r)
			case http.MethodDelete:
				rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rt.Cache.Delete(w, r, key)
				}), "agent", "admin").ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(rt.Service.List), "viewer", "agent", "admin")).ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Service.Health(w, r, name)
		}), "viewer", "agent", "admin")).ServeHTTP(w, r)
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
		rt.requireAuth(rt.requireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rt.Service.Docs(w, r, name)
		}), "viewer", "agent", "admin")).ServeHTTP(w, r)
		return
	}

	// Proxy business services.
	if strings.HasPrefix(r.URL.Path, "/api/v1/services/") {
		// Temporary: allow public (no-auth) read access for polymarket query endpoints,
		// so the web UI can be opened without login during early rollout.
		//
		// Note: write methods still require agent/admin. Other services remain protected.
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if name, rest, ok := parseServicePath(r.URL.Path); ok && name == "polymarket" {
				if rest == "/healthz" || strings.HasPrefix(rest, "/api/v2/") || strings.HasPrefix(rest, "/api/catalog/") {
					rt.Proxy.ServeHTTP(w, r)
					return
				}
			}
		}

		// Viewer can only read. Agent/admin can write.
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			rt.requireAuth(rt.requireRole(rt.Proxy, "viewer", "agent", "admin")).ServeHTTP(w, r)
			return
		}
		rt.requireAuth(rt.requireRole(rt.Proxy, "agent", "admin")).ServeHTTP(w, r)
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

func (rt Router) requireRole(h http.Handler, roles ...string) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[strings.TrimSpace(strings.ToLower(r))] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "missing token")
			return
		}
		role := strings.ToLower(strings.TrimSpace(c.Role))
		if !allowed[role] {
			httpx.WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		h.ServeHTTP(w, r)
	})
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
