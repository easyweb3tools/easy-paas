package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/config"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Services map[string]config.ServiceConfig
	Client   *http.Client
}

func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	out := make([]map[string]any, 0, len(h.Services))
	for name, sc := range h.Services {
		out = append(out, map[string]any{
			"name":        name,
			"base_url":    sc.BaseURL,
			"health_path": sc.HealthPath,
			"docs_path":   sc.DocsPath,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h Handler) Health(w http.ResponseWriter, r *http.Request, name string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	sc, ok := h.Services[name]
	if !ok || sc.BaseURL == "" {
		httpx.WriteError(w, http.StatusNotFound, "unknown service")
		return
	}

	u, err := url.Parse(sc.BaseURL)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "bad upstream")
		return
	}
	u.Path = sc.HealthPath

	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"name": name, "status": "down", "error": err.Error()})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	status := "down"
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status = "ok"
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"name": name, "status": status, "http_status": resp.StatusCode})
}

func (h Handler) Docs(w http.ResponseWriter, r *http.Request, name string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	sc, ok := h.Services[name]
	if !ok || sc.BaseURL == "" {
		httpx.WriteError(w, http.StatusNotFound, "unknown service")
		return
	}
	if strings.TrimSpace(sc.DocsPath) == "" {
		httpx.WriteError(w, http.StatusNotFound, "docs not configured")
		return
	}

	u, err := url.Parse(sc.BaseURL)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "bad upstream")
		return
	}
	u.Path = sc.DocsPath

	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	// Forward Authorization so upstream services can protect /docs as well.
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		req.Header.Set("Authorization", v)
	}
	// Mimic gateway-added headers so services can enforce access-via-gateway for docs too.
	if c, ok := auth.ClaimsFromContext(r.Context()); ok {
		req.Header.Set("X-Easyweb3-Project", c.ProjectID)
		req.Header.Set("X-Easyweb3-Role", c.Role)
	}
	resp, err := client.Do(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpx.WriteError(w, http.StatusBadGateway, "upstream docs not available")
		return
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "upstream read failed")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
