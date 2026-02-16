package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"polymarket/internal/repository"
)

type V2ReviewHandler struct {
	Repo repository.Repository
}

func (h *V2ReviewHandler) Register(r *gin.Engine) {
	g := r.Group("/api/v2/review")
	g.GET("", h.list)
	g.GET("/missed", h.missed)
	g.GET("/regret-index", h.regretIndex)
	g.GET("/label-performance", h.labelPerformance)
	g.PUT("/:id/notes", h.putNotes)
}

func (h *V2ReviewHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	since, until := timeRangeFromQuery(c)
	var ourAction *string
	if v := strings.TrimSpace(c.Query("our_action")); v != "" {
		ourAction = &v
	}
	var strategyName *string
	if v := strings.TrimSpace(c.Query("strategy_name")); v != "" {
		strategyName = &v
	}
	params := repository.ListMarketReviewParams{
		Limit:        limit,
		Offset:       offset,
		OurAction:    ourAction,
		StrategyName: strategyName,
		Since:        since,
		Until:        until,
		OrderBy:      "hypothetical_pnl",
		Asc:          boolPtr(false),
	}
	items, err := h.Repo.ListMarketReviews(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountMarketReviews(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, total))
}

func (h *V2ReviewHandler) missed(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	min := decimal.Zero
	items, err := h.Repo.ListMarketReviews(c.Request.Context(), repository.ListMarketReviewParams{
		Limit:   intQuery(c, "limit", 100),
		Offset:  intQuery(c, "offset", 0),
		MinPnL:  &min,
		OrderBy: "hypothetical_pnl",
		Asc:     boolPtr(false),
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		switch strings.ToLower(strings.TrimSpace(it.OurAction)) {
		case "dismissed", "expired", "missed":
			if it.HypotheticalPnL.GreaterThan(decimal.Zero) {
				out = append(out, it)
			}
		}
	}
	Ok(c, out, nil)
}

func (h *V2ReviewHandler) regretIndex(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	row, err := h.Repo.MissedAlphaSummary(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, row, nil)
}

func (h *V2ReviewHandler) labelPerformance(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	rows, err := h.Repo.LabelPerformance(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, nil)
}

type putReviewNotesRequest struct {
	Notes      string   `json:"notes"`
	LessonTags []string `json:"lesson_tags"`
}

func (h *V2ReviewHandler) putNotes(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	var req putReviewNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	raw, _ := json.Marshal(req.LessonTags)
	if err := h.Repo.UpdateMarketReviewNotes(c.Request.Context(), id, req.Notes, raw); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, map[string]any{"id": id, "updated": true}, nil)
}
