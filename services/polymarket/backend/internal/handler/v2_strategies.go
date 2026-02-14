package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"polymarket/internal/paas"
	"polymarket/internal/repository"
)

type V2StrategyHandler struct {
	Repo repository.Repository
}

func (h *V2StrategyHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/strategies")
	group.GET("", h.listStrategies)
	group.GET("/:name", h.getStrategy)
	group.GET("/:name/stats", h.stats)
	group.POST("/:name/enable", h.enableStrategy)
	group.POST("/:name/disable", h.disableStrategy)
	group.PUT("/:name/params", h.updateParams)
}

func (h *V2StrategyHandler) listStrategies(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	items, err := h.Repo.ListStrategies(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, nil)
}

func (h *V2StrategyHandler) getStrategy(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "name required", nil)
		return
	}
	item, err := h.Repo.GetStrategyByName(c.Request.Context(), name)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "strategy not found", nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2StrategyHandler) stats(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "name required", nil)
		return
	}
	strat, err := h.Repo.GetStrategyByName(c.Request.Context(), name)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if strat == nil {
		Error(c, http.StatusNotFound, "strategy not found", nil)
		return
	}
	active := "active"
	activeOpps, err := h.Repo.CountOpportunities(c.Request.Context(), repository.ListOpportunitiesParams{
		Status:       &active,
		StrategyName: &name,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	analyticsRows, err := h.Repo.AnalyticsByStrategy(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	var plans int64
	var totalPnLUSD float64
	var avgROI float64
	for _, row := range analyticsRows {
		if strings.EqualFold(strings.TrimSpace(row.StrategyName), name) {
			plans = row.Plans
			totalPnLUSD = row.TotalPnLUSD
			avgROI = row.AvgROI
			break
		}
	}
	Ok(c, map[string]any{
		"name":                 strat.Name,
		"enabled":              strat.Enabled,
		"priority":             strat.Priority,
		"category":             strat.Category,
		"active_opportunities": activeOpps,
		"plans":                plans,
		"total_pnl_usd":        totalPnLUSD,
		"avg_roi":              avgROI,
	}, nil)
}

func (h *V2StrategyHandler) enableStrategy(c *gin.Context) {
	h.setEnabled(c, true)
}

func (h *V2StrategyHandler) disableStrategy(c *gin.Context) {
	h.setEnabled(c, false)
}

func (h *V2StrategyHandler) setEnabled(c *gin.Context, enabled bool) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "name required", nil)
		return
	}
	if err := h.Repo.SetStrategyEnabled(c.Request.Context(), name, enabled); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	action := "polymarket_strategy_disabled"
	if enabled {
		action = "polymarket_strategy_enabled"
	}
	paas.LogBestEffort(c, action, "info", map[string]any{
		"name":    name,
		"enabled": enabled,
	})
	Ok(c, map[string]any{"name": name, "enabled": enabled}, nil)
}

func (h *V2StrategyHandler) updateParams(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "name required", nil)
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	if len(body) == 0 {
		Error(c, http.StatusBadRequest, "params required", nil)
		return
	}
	if err := h.Repo.UpdateStrategyParams(c.Request.Context(), name, body); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_strategy_params_updated", "info", map[string]any{
		"name": name,
	})
	Ok(c, map[string]any{"name": name}, nil)
}
