package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Dex        Dexscreener
	GoPlus     GoPlus
	Polymarket Polymarket
}

type QueryRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func (h Handler) Query(w http.ResponseWriter, r *http.Request, provider string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		httpx.WriteError(w, http.StatusBadRequest, "provider required")
		return
	}

	var req QueryRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Method = strings.TrimSpace(req.Method)
	if req.Params == nil {
		req.Params = map[string]any{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ctx = withBearer(ctx, r.Header.Get("Authorization"))
	if c, ok := auth.ClaimsFromContext(r.Context()); ok {
		ctx = withClaims(ctx, c.ProjectID, c.Role)
	}

	var out json.RawMessage
	var err error
	switch provider {
	case "dexscreener":
		out, err = h.Dex.Query(ctx, req.Method, req.Params)
	case "goplus":
		out, err = h.GoPlus.Query(ctx, req.Method, req.Params)
	case "polymarket":
		out, err = h.Polymarket.Query(ctx, req.Method, req.Params)
	default:
		httpx.WriteError(w, http.StatusNotFound, "unknown provider")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(bytes.TrimSpace(out)) == 0 {
		out = json.RawMessage("{}")
	}
	if !json.Valid(out) {
		httpx.WriteError(w, http.StatusBadGateway, "provider returned invalid json")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}
