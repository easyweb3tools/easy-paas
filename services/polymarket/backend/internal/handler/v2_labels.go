package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"polymarket/internal/labeler"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type V2LabelHandler struct {
	Repo    repository.Repository
	Labeler *labeler.MarketLabeler
}

func (h *V2LabelHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/markets")
	group.GET("/labels", h.listLabels)
	group.POST("/:id/labels", h.addLabel)
	group.DELETE("/:id/labels/:label", h.deleteLabel)
	group.POST("/auto-label", h.autoLabel)
}

func (h *V2LabelHandler) listLabels(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	marketID := strings.TrimSpace(c.Query("market_id"))
	label := strings.TrimSpace(c.Query("label"))
	limit := intQuery(c, "limit", 200)
	offset := intQuery(c, "offset", 0)

	var marketPtr *string
	if marketID != "" {
		marketPtr = &marketID
	}
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}

	items, err := h.Repo.ListMarketLabels(c.Request.Context(), repository.ListMarketLabelsParams{
		Limit:    limit,
		Offset:   offset,
		MarketID: marketPtr,
		Label:    labelPtr,
		OrderBy:  "created_at",
		Asc:      boolPtr(false),
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, int64(len(items)))
	Ok(c, items, meta)
}

type addLabelRequest struct {
	Label       string   `json:"label"`
	SubLabel    *string  `json:"sub_label"`
	AutoLabeled *bool    `json:"auto_labeled"`
	Confidence  *float64 `json:"confidence"`
}

func (h *V2LabelHandler) addLabel(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	marketID := strings.TrimSpace(c.Param("id"))
	if marketID == "" {
		Error(c, http.StatusBadRequest, "market id required", nil)
		return
	}
	var req addLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" {
		Error(c, http.StatusBadRequest, "label required", nil)
		return
	}
	item := &models.MarketLabel{
		MarketID: marketID,
		Label:    req.Label,
		SubLabel: req.SubLabel,
	}
	if req.AutoLabeled != nil {
		item.AutoLabeled = *req.AutoLabeled
	}
	if req.Confidence != nil {
		item.Confidence = *req.Confidence
	}
	if err := h.Repo.UpsertMarketLabel(c.Request.Context(), item); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2LabelHandler) deleteLabel(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	marketID := strings.TrimSpace(c.Param("id"))
	label := strings.TrimSpace(c.Param("label"))
	if marketID == "" || label == "" {
		Error(c, http.StatusBadRequest, "market id and label required", nil)
		return
	}
	if err := h.Repo.DeleteMarketLabel(c.Request.Context(), marketID, label); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, map[string]any{"market_id": marketID, "label": label}, nil)
}

func (h *V2LabelHandler) autoLabel(c *gin.Context) {
	if h.Labeler == nil {
		Error(c, http.StatusServiceUnavailable, "labeler disabled", nil)
		return
	}
	if err := h.Labeler.LabelMarkets(c.Request.Context()); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, map[string]any{"status": "ok"}, nil)
}
