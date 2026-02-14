package cache

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Store      Store
	DefaultTTL time.Duration
}

type putRequest struct {
	// Either set Value (any JSON) or ValueBase64.
	Value       any    `json:"value"`
	ValueBase64 string `json:"value_base64"`
	TTLSeconds  int64  `json:"ttl_seconds"`
}

type getResponse struct {
	Key         string `json:"key"`
	Found       bool   `json:"found"`
	ValueBase64 string `json:"value_base64,omitempty"`
}

func (h Handler) Get(w http.ResponseWriter, r *http.Request, key string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	if h.Store == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "cache not configured")
		return
	}

	b, found, err := h.Store.Get(r.Context(), key)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cache get failed")
		return
	}
	resp := getResponse{Key: key, Found: found}
	if found {
		resp.ValueBase64 = base64.StdEncoding.EncodeToString(b)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h Handler) Put(w http.ResponseWriter, r *http.Request, key string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	if h.Store == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "cache not configured")
		return
	}

	var req putRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var val []byte
	if strings.TrimSpace(req.ValueBase64) != "" {
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.ValueBase64))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid value_base64")
			return
		}
		val = b
	} else {
		b, err := json.Marshal(req.Value)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid value")
			return
		}
		if len(bytes.TrimSpace(b)) == 0 {
			b = []byte("null")
		}
		val = b
	}

	ttl := h.DefaultTTL
	if req.TTLSeconds != 0 {
		if req.TTLSeconds < 0 {
			ttl = 0
		} else {
			ttl = time.Duration(req.TTLSeconds) * time.Second
		}
	}

	if err := h.Store.Set(r.Context(), key, val, ttl); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cache set failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) Delete(w http.ResponseWriter, r *http.Request, key string) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	if h.Store == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "cache not configured")
		return
	}
	if err := h.Store.Delete(r.Context(), key); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cache delete failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
