package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
)

type V2SignalHandler struct {
	Repo repository.Repository
}

func (h *V2SignalHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/signals")
	group.GET("", h.listSignals)
	group.GET("/sources", h.listSources)
}

func (h *V2SignalHandler) listSignals(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	typ := strings.TrimSpace(c.Query("type"))
	source := strings.TrimSpace(c.Query("source"))
	since := strings.TrimSpace(c.Query("since"))
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)

	var sinceTime *time.Time
	if since != "" {
		if parsed, err := time.Parse(time.RFC3339, since); err == nil {
			parsed = parsed.UTC()
			sinceTime = &parsed
		}
	}

	var typePtr *string
	if typ != "" {
		typePtr = &typ
	}
	var sourcePtr *string
	if source != "" {
		sourcePtr = &source
	}

	items, err := h.Repo.ListSignals(c.Request.Context(), repository.ListSignalsParams{
		Limit:   limit,
		Offset:  offset,
		Type:    typePtr,
		Source:  sourcePtr,
		Since:   sinceTime,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, int64(len(items)))
	Ok(c, items, meta)
}

func (h *V2SignalHandler) listSources(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	items, err := h.Repo.ListSignalSources(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, nil)
}

func boolPtr(v bool) *bool { return &v }
