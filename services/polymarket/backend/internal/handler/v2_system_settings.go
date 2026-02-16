package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
	"polymarket/internal/service"
)

type V2SystemSettingsHandler struct {
	Repo     repository.Repository
	Settings *service.SystemSettingsService
}

func (h *V2SystemSettingsHandler) Register(r *gin.Engine) {
	g := r.Group("/api/v2/system-settings")
	g.GET("", h.list)
	g.GET("/switches", h.listSwitches)
	g.GET("/switches/:name", h.getSwitch)
	g.PUT("/switches/:name", h.putSwitch)
	g.GET("/:key", h.get)
	g.PUT("/:key", h.put)
}

func (h *V2SystemSettingsHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 200)
	offset := intQuery(c, "offset", 0)
	var prefix *string
	if v := strings.TrimSpace(c.Query("prefix")); v != "" {
		prefix = &v
	}
	params := repository.ListSystemSettingsParams{
		Limit:   limit,
		Offset:  offset,
		Prefix:  prefix,
		OrderBy: "key",
		Asc:     boolPtr(true),
	}
	items, err := h.Repo.ListSystemSettings(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountSystemSettings(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, total))
}

func (h *V2SystemSettingsHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		Error(c, http.StatusBadRequest, "invalid key", nil)
		return
	}
	item, err := h.Repo.GetSystemSettingByKey(c.Request.Context(), key)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "setting not found", nil)
		return
	}
	Ok(c, item, nil)
}

type putSystemSettingRequest struct {
	Value       any    `json:"value"`
	Description string `json:"description"`
}

func (h *V2SystemSettingsHandler) put(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		Error(c, http.StatusBadRequest, "invalid key", nil)
		return
	}
	var req putSystemSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	raw, err := json.Marshal(req.Value)
	if err != nil {
		Error(c, http.StatusBadRequest, "invalid value", nil)
		return
	}
	item := &models.SystemSetting{
		Key:         key,
		Value:       datatypes.JSON(raw),
		Description: strings.TrimSpace(req.Description),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := h.Repo.UpsertSystemSetting(c.Request.Context(), item); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	next, _ := h.Repo.GetSystemSettingByKey(c.Request.Context(), key)
	Ok(c, next, nil)
}

func (h *V2SystemSettingsHandler) listSwitches(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	prefix := "feature."
	params := repository.ListSystemSettingsParams{
		Limit:   intQuery(c, "limit", 200),
		Offset:  intQuery(c, "offset", 0),
		Prefix:  &prefix,
		OrderBy: "key",
		Asc:     boolPtr(true),
	}
	items, err := h.Repo.ListSystemSettings(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		enabled := false
		_ = json.Unmarshal(it.Value, &enabled)
		out = append(out, map[string]any{
			"name":        strings.TrimPrefix(it.Key, "feature."),
			"key":         it.Key,
			"enabled":     enabled,
			"description": it.Description,
			"updated_at":  it.UpdatedAt,
		})
	}
	Ok(c, out, nil)
}

func (h *V2SystemSettingsHandler) getSwitch(c *gin.Context) {
	if h.Settings == nil {
		Error(c, http.StatusInternalServerError, "settings service unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid switch name", nil)
		return
	}
	key := "feature." + name
	enabled := h.Settings.IsEnabled(c.Request.Context(), key, false)
	Ok(c, map[string]any{
		"name":    name,
		"key":     key,
		"enabled": enabled,
	}, nil)
}

type putSwitchRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *V2SystemSettingsHandler) putSwitch(c *gin.Context) {
	if h.Settings == nil {
		Error(c, http.StatusInternalServerError, "settings service unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid switch name", nil)
		return
	}
	var req putSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	key := "feature." + name
	if err := h.Settings.SetEnabled(c.Request.Context(), key, req.Enabled); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, map[string]any{
		"name":    name,
		"key":     key,
		"enabled": req.Enabled,
	}, nil)
}
