package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type V2ExecutionRuleHandler struct {
	Repo repository.Repository
}

func (h *V2ExecutionRuleHandler) Register(r *gin.Engine) {
	g := r.Group("/api/v2/execution-rules")
	g.GET("", h.list)
	g.GET("/:strategy", h.get)
	g.PUT("/:strategy", h.put)
	g.DELETE("/:strategy", h.delete)
}

func (h *V2ExecutionRuleHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	items, err := h.Repo.ListExecutionRules(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, nil)
}

func (h *V2ExecutionRuleHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("strategy"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid strategy", nil)
		return
	}
	item, err := h.Repo.GetExecutionRuleByStrategyName(c.Request.Context(), name)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "execution rule not found", nil)
		return
	}
	Ok(c, item, nil)
}

type putExecutionRuleRequest struct {
	AutoExecute    *bool    `json:"auto_execute"`
	MinConfidence  *float64 `json:"min_confidence"`
	MinEdgePct     *string  `json:"min_edge_pct"`
	StopLossPct    *string  `json:"stop_loss_pct"`
	TakeProfitPct  *string  `json:"take_profit_pct"`
	MaxHoldHours   *int     `json:"max_hold_hours"`
	MaxDailyTrades *int     `json:"max_daily_trades"`
}

func (h *V2ExecutionRuleHandler) put(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("strategy"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid strategy", nil)
		return
	}
	var req putExecutionRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	item, err := h.Repo.GetExecutionRuleByStrategyName(c.Request.Context(), name)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		item = &models.ExecutionRule{
			StrategyName:   name,
			AutoExecute:    false,
			MinConfidence:  0.8,
			MinEdgePct:     decimal.NewFromFloat(0.05),
			StopLossPct:    decimal.NewFromFloat(0.10),
			TakeProfitPct:  decimal.NewFromFloat(0.20),
			MaxHoldHours:   72,
			MaxDailyTrades: 10,
			CreatedAt:      time.Now().UTC(),
		}
	}
	if req.AutoExecute != nil {
		item.AutoExecute = *req.AutoExecute
	}
	if req.MinConfidence != nil {
		item.MinConfidence = *req.MinConfidence
	}
	if req.MinEdgePct != nil {
		v, err := decimal.NewFromString(strings.TrimSpace(*req.MinEdgePct))
		if err != nil {
			Error(c, http.StatusBadRequest, "invalid min_edge_pct", nil)
			return
		}
		item.MinEdgePct = v
	}
	if req.StopLossPct != nil {
		v, err := decimal.NewFromString(strings.TrimSpace(*req.StopLossPct))
		if err != nil {
			Error(c, http.StatusBadRequest, "invalid stop_loss_pct", nil)
			return
		}
		item.StopLossPct = v
	}
	if req.TakeProfitPct != nil {
		v, err := decimal.NewFromString(strings.TrimSpace(*req.TakeProfitPct))
		if err != nil {
			Error(c, http.StatusBadRequest, "invalid take_profit_pct", nil)
			return
		}
		item.TakeProfitPct = v
	}
	if req.MaxHoldHours != nil {
		item.MaxHoldHours = *req.MaxHoldHours
	}
	if req.MaxDailyTrades != nil {
		item.MaxDailyTrades = *req.MaxDailyTrades
	}
	item.StrategyName = name
	item.UpdatedAt = time.Now().UTC()
	if err := h.Repo.UpsertExecutionRule(c.Request.Context(), item); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2ExecutionRuleHandler) delete(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("strategy"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid strategy", nil)
		return
	}
	if err := h.Repo.DeleteExecutionRuleByStrategyName(c.Request.Context(), name); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, map[string]any{"strategy": name, "deleted": true}, nil)
}
