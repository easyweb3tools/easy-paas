package gateway

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/config"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Proxy struct {
	services map[string]config.ServiceConfig

	mu      sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
}

func NewProxy(services map[string]config.ServiceConfig) *Proxy {
	// Copy to avoid external mutation.
	m := make(map[string]config.ServiceConfig, len(services))
	for k, v := range services {
		m[k] = v
	}
	return &Proxy{services: m, proxies: map[string]*httputil.ReverseProxy{}}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name, rest, ok := parseServicePath(r.URL.Path)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	cfg, ok := p.services[name]
	if !ok || cfg.BaseURL == "" {
		httpx.WriteError(w, http.StatusNotFound, "unknown service")
		return
	}

	proxy, err := p.getProxy(name, cfg)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "bad upstream")
		return
	}

	// Rewrite path to upstream path.
	r.URL.Path = rest
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	// Inject some context headers for debugging / future use.
	if c, ok := auth.ClaimsFromContext(r.Context()); ok {
		r.Header.Set("X-Easyweb3-Project", c.ProjectID)
		r.Header.Set("X-Easyweb3-Role", c.Role)
	}

	proxy.ServeHTTP(w, r)
}

func (p *Proxy) getProxy(name string, cfg config.ServiceConfig) (*httputil.ReverseProxy, error) {
	p.mu.RLock()
	if rp := p.proxies[name]; rp != nil {
		p.mu.RUnlock()
		return rp, nil
	}
	p.mu.RUnlock()

	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid base_url")
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		// Keep the upstream host as Host header.
		req.Host = u.Host
	}

	p.mu.Lock()
	p.proxies[name] = rp
	p.mu.Unlock()
	return rp, nil
}

// parseServicePath extracts {name} and rest from:
//
//	/api/v1/services/{name}/... => name, /...
//	/api/v1/services/{name}     => name, /
func parseServicePath(path string) (name string, rest string, ok bool) {
	const prefix = "/api/v1/services/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	remaining := strings.TrimPrefix(path, prefix)
	if remaining == "" {
		return "", "", false
	}
	parts := strings.SplitN(remaining, "/", 2)
	name = parts[0]
	if name == "" {
		return "", "", false
	}
	if len(parts) == 1 {
		return name, "/", true
	}
	rest = "/" + parts[1]
	return name, rest, true
}
