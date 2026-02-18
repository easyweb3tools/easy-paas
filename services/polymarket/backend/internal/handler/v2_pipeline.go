package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
)

type V2PipelineHandler struct {
	Repo repository.Repository
}

func (h *V2PipelineHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/pipeline")
	group.GET("/health", h.health)
}

func (h *V2PipelineHandler) health(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	ctx := c.Request.Context()

	// Markets
	active := true
	closed := false
	marketsTotal, _ := h.Repo.CountMarkets(ctx, repository.ListMarketsParams{Limit: 1})
	marketsActive, _ := h.Repo.CountMarkets(ctx, repository.ListMarketsParams{Limit: 1, Active: &active, Closed: &closed})

	// Orderbook
	freshWindow := 10 * time.Minute
	obTotal, obFresh, _ := h.Repo.CountOrderbookLatest(ctx, freshWindow)

	// Labels
	labelCount, _ := h.Repo.CountMarketLabels(ctx)

	// Signals (last hour)
	since := time.Now().UTC().Add(-1 * time.Hour)
	signalsByType, _ := h.Repo.CountSignalsByType(ctx, &since)

	// Opportunities
	oppsActive, _ := h.Repo.CountActiveOpportunities(ctx)

	// Strategies
	strategies, _ := h.Repo.ListStrategies(ctx)
	strategiesEnabled := 0
	for _, s := range strategies {
		if s.Enabled {
			strategiesEnabled++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"markets_total":          marketsTotal,
		"markets_active":         marketsActive,
		"orderbook_latest_count": obTotal,
		"orderbook_fresh_count":  obFresh,
		"market_labels_count":    labelCount,
		"signals_last_hour":      signalsByType,
		"opportunities_active":   oppsActive,
		"strategies_enabled":     strategiesEnabled,
		"strategies_total":       len(strategies),
	})
}
