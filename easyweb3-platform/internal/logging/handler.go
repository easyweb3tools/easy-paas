package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Store Store
	seq   int64
}

type createLogRequest struct {
	Agent      string          `json:"agent"`
	Action     string          `json:"action"`
	Level      string          `json:"level"`
	Details    json.RawMessage `json:"details"`
	SessionKey string          `json:"session_key"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	var req createLogRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Action) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "action required")
		return
	}
	if req.Level == "" {
		req.Level = "info"
	}
	if len(bytes.TrimSpace(req.Details)) == 0 {
		req.Details = json.RawMessage("{}")
	}
	if len(bytes.TrimSpace(req.Metadata)) == 0 {
		req.Metadata = json.RawMessage("{}")
	}

	now := time.Now().UTC()
	id := NewLogID(now, atomic.AddInt64(&h.seq, 1))
	l := OperationLog{
		ID:         id,
		ProjectID:  c.ProjectID,
		Agent:      strings.TrimSpace(req.Agent),
		Action:     strings.TrimSpace(req.Action),
		Level:      strings.TrimSpace(req.Level),
		Details:    req.Details,
		SessionKey: strings.TrimSpace(req.SessionKey),
		CreatedAt:  now,
		Metadata:   req.Metadata,
	}
	if err := h.Store.Create(l); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to store log")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":         l.ID,
		"created_at": l.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	q := r.URL.Query()
	flt := ListFilter{
		ProjectID: c.ProjectID,
		Action:    strings.TrimSpace(q.Get("action")),
		Level:     strings.TrimSpace(q.Get("level")),
		Limit:     atoiDefault(q.Get("limit"), 100),
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			flt.From = &t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			flt.To = &t
		}
	}

	logs, err := h.Store.List(flt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to list logs")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, logs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request, id string) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	l, found, err := h.Store.Get(id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to get log")
		return
	}
	if !found || l.ProjectID != c.ProjectID {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, l)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	q := r.URL.Query()
	flt := ListFilter{
		ProjectID: c.ProjectID,
		From:      nil,
		To:        nil,
		Action:    strings.TrimSpace(q.Get("action")),
		Level:     strings.TrimSpace(q.Get("level")),
	}
	m, err := h.Store.Stats(flt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to compute stats")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, m)
}

func atoiDefault(v string, def int) int {
	if v = strings.TrimSpace(v); v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
