package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
)

type V2PositionHandler struct {
	Repo repository.Repository
}

func (h *V2PositionHandler) Register(r *gin.Engine) {
	p := r.Group("/api/v2/positions")
	p.GET("", h.list)
	p.GET("/summary", h.summary)
	p.GET("/:id", h.get)

	portfolio := r.Group("/api/v2/portfolio")
	portfolio.GET("/history", h.history)
}

func (h *V2PositionHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	orderBy := parseOrder(strings.TrimSpace(c.Query("order_by")), map[string]string{
		"unrealized_pnl": "unrealized_pnl",
		"cost_basis":     "cost_basis",
		"opened_at":      "opened_at",
		"created_at":     "created_at",
	})
	if orderBy == "" {
		orderBy = "opened_at"
	}
	order := strings.ToLower(strings.TrimSpace(c.Query("order")))
	asc := false
	if order == "asc" {
		asc = true
	}

	var status *string
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		status = &v
	}
	var strategyName *string
	if v := strings.TrimSpace(c.Query("strategy_name")); v != "" {
		strategyName = &v
	}
	var marketID *string
	if v := strings.TrimSpace(c.Query("market_id")); v != "" {
		marketID = &v
	}

	params := repository.ListPositionsParams{
		Limit:        limit,
		Offset:       offset,
		Status:       status,
		StrategyName: strategyName,
		MarketID:     marketID,
		OrderBy:      orderBy,
		Asc:          boolPtr(asc),
	}
	items, err := h.Repo.ListPositions(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountPositions(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, total))
}

func (h *V2PositionHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	item, err := h.Repo.GetPositionByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "position not found", nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2PositionHandler) summary(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	out, err := h.Repo.PositionsSummary(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, out, nil)
}

func (h *V2PositionHandler) history(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 168)
	offset := intQuery(c, "offset", 0)
	var since *time.Time
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			t := ts.UTC()
			since = &t
		}
	}
	var until *time.Time
	if raw := strings.TrimSpace(c.Query("until")); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			t := ts.UTC()
			until = &t
		}
	}
	items, err := h.Repo.ListPortfolioSnapshots(c.Request.Context(), repository.ListPortfolioSnapshotsParams{
		Limit:  limit,
		Offset: offset,
		Since:  since,
		Until:  until,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, int64(len(items))))
}
