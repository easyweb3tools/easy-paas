package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Keys  *FileKeyStore
	Users *FileUserStore
	JWT   JWT
}

type loginRequest struct {
	APIKey    string `json:"api_key,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
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
	// API key login (for agents/admin).
	if strings.TrimSpace(req.APIKey) != "" {
		rec, ok := h.Keys.Validate(req.APIKey)
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
		return
	}

	// Username/password login (registered user, default has no grants).
	if h.Users == nil {
		httpx.WriteError(w, http.StatusBadRequest, "user login not enabled")
		return
	}
	_, role, err := h.Users.Authenticate(req.Username, req.Password, req.ProjectID)
	if err != nil {
		// Distinguish invalid credentials vs no grants.
		if err.Error() == "no grants" {
			httpx.WriteError(w, http.StatusForbidden, "no grants")
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	tok, exp, err := h.JWT.Sign(Claims{
		ProjectID: strings.TrimSpace(req.ProjectID),
		Role:      role,
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
	raw, rec, err := h.Keys.Create(req.ProjectID, req.Role, req.Name)
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

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	User any `json:"user"`
}

func (h Handler) Register(w http.ResponseWriter, r *http.Request) {
	if h.Users == nil {
		httpx.WriteError(w, http.StatusBadRequest, "registration not enabled")
		return
	}
	var req registerRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := h.Users.Create(req.Username, req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Users.Save(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to persist user")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, registerResponse{User: map[string]any{
		"id":         u.ID,
		"username":   u.Username,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
	}})
}

type grantRequest struct {
	User      string `json:"user"` // user_id or username
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
}

func (h Handler) Grant(w http.ResponseWriter, r *http.Request) {
	c, ok := ClaimsFromContext(r.Context())
	if !ok || (c.Role != "admin" && c.Role != "agent") {
		httpx.WriteError(w, http.StatusForbidden, "agent or admin required")
		return
	}
	if h.Users == nil {
		httpx.WriteError(w, http.StatusBadRequest, "user store not enabled")
		return
	}
	var req grantRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := h.Users.Grant(req.User, req.ProjectID, req.Role)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":       u.ID,
			"username": u.Username,
			"grants":   u.Grants,
		},
	})
}

func (h Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	c, ok := ClaimsFromContext(r.Context())
	if !ok || (c.Role != "admin" && c.Role != "agent") {
		httpx.WriteError(w, http.StatusForbidden, "agent or admin required")
		return
	}
	if h.Users == nil {
		httpx.WriteError(w, http.StatusBadRequest, "user store not enabled")
		return
	}
	users := h.Users.List()
	out := make([]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{
			"id":         u.ID,
			"username":   u.Username,
			"grants":     u.Grants,
			"disabled":   u.Disabled,
			"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": out})
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
