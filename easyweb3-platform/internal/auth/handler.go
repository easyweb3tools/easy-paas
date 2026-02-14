package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Store *FileKeyStore
	JWT   JWT
}

type loginRequest struct {
	APIKey string `json:"api_key"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec, ok := h.Store.Validate(req.APIKey)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid api key")
		return
	}

	tok, exp, err := h.JWT.Sign(Claims{
		ProjectID: rec.ProjectID,
		Role:      rec.Role,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{
		Token:     tok,
		ExpiresAt: exp.UTC().Format(time.RFC3339),
	})
}

func (h Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Refresh requires a valid Bearer token.
	c, ok := ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	tok, exp, err := h.JWT.Sign(Claims{
		ProjectID: c.ProjectID,
		Role:      c.Role,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{
		Token:     tok,
		ExpiresAt: exp.UTC().Format(time.RFC3339),
	})
}

type statusResponse struct {
	Authenticated bool   `json:"authenticated"`
	Project       string `json:"project,omitempty"`
	Role          string `json:"role,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

func (h Handler) Status(w http.ResponseWriter, r *http.Request) {
	// Public endpoint: missing/invalid token just means authenticated=false.
	tok := bearerTokenLocal(r.Header.Get("Authorization"))
	if tok == "" {
		httpx.WriteJSON(w, http.StatusOK, statusResponse{Authenticated: false})
		return
	}
	c, err := h.JWT.Verify(tok)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, statusResponse{Authenticated: false})
		return
	}
	resp := statusResponse{
		Authenticated: true,
		Project:       c.ProjectID,
		Role:          c.Role,
	}
	if c.ExpiresAt != nil {
		resp.ExpiresAt = c.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type createKeyRequest struct {
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
	Name      string `json:"name"`
}

type createKeyResponse struct {
	APIKey string `json:"api_key"`
	Key    any    `json:"key"`
}

func (h Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	c, ok := ClaimsFromContext(r.Context())
	if !ok || c.Role != "admin" {
		httpx.WriteError(w, http.StatusForbidden, "admin required")
		return
	}
	var req createKeyRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, rec, err := h.Store.Create(req.ProjectID, req.Role, req.Name)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Don't expose hash.
	respKey := map[string]any{
		"id":         rec.ID,
		"name":       rec.Name,
		"project_id": rec.ProjectID,
		"role":       rec.Role,
		"created_at": rec.CreatedAt.UTC().Format(time.RFC3339),
	}
	httpx.WriteJSON(w, http.StatusOK, createKeyResponse{APIKey: raw, Key: respKey})
}

func bearerTokenLocal(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	parts := strings.SplitN(v, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
