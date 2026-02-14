package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/paas"
	"polymarket/internal/repository"
)

type V2SettlementHandler struct {
	Repo repository.Repository
}

func (h *V2SettlementHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/settlements")
	group.POST("", h.upsert)
	group.GET("/label-rates", h.labelRates)
}

type upsertSettlementRequest struct {
	MarketID         string  `json:"market_id"`
	Outcome          string  `json:"outcome"` // YES|NO
	SettledAtRFC3339 string  `json:"settled_at"`
	Category         *string `json:"category"`

	InitialYesPrice *string `json:"initial_yes_price"`
	FinalYesPrice   *string `json:"final_yes_price"`
}

func (h *V2SettlementHandler) upsert(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	var req upsertSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	req.MarketID = strings.TrimSpace(req.MarketID)
	req.Outcome = strings.ToUpper(strings.TrimSpace(req.Outcome))
	if req.MarketID == "" || (req.Outcome != "YES" && req.Outcome != "NO") {
		Error(c, http.StatusBadRequest, "market_id and outcome(YES|NO) required", nil)
		return
	}
	settledAt := time.Now().UTC()
	if strings.TrimSpace(req.SettledAtRFC3339) != "" {
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(req.SettledAtRFC3339)); err == nil {
			settledAt = ts.UTC()
		}
	}
	var initial *decimal.Decimal
	if req.InitialYesPrice != nil && strings.TrimSpace(*req.InitialYesPrice) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.InitialYesPrice)); err == nil {
			initial = &v
		}
	}
	var final *decimal.Decimal
	if req.FinalYesPrice != nil && strings.TrimSpace(*req.FinalYesPrice) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.FinalYesPrice)); err == nil {
			final = &v
		}
	}

	// Resolve market metadata from catalog.
	markets, err := h.Repo.ListMarketsByIDs(c.Request.Context(), []string{req.MarketID})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if len(markets) == 0 {
		Error(c, http.StatusNotFound, "market not found", nil)
		return
	}
	market := markets[0]
	if strings.TrimSpace(market.EventID) == "" {
		Error(c, http.StatusBadRequest, "market missing event_id", nil)
		return
	}

	// Attach current labels (string list) for later grouping.
	mid := req.MarketID
	labels, _ := h.Repo.ListMarketLabels(c.Request.Context(), repository.ListMarketLabelsParams{
		Limit:    2000,
		Offset:   0,
		MarketID: &mid,
		OrderBy:  "created_at",
		Asc:      boolPtr(false),
	})
	labelNames := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, l := range labels {
		val := strings.TrimSpace(l.Label)
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		labelNames = append(labelNames, val)
	}
	labelsJSON, _ := json.Marshal(labelNames)

	item := &models.MarketSettlementHistory{
		MarketID:        req.MarketID,
		EventID:         market.EventID,
		Question:        market.Question,
		Outcome:         req.Outcome,
		Category:        "",
		Labels:          datatypes.JSON(labelsJSON),
		InitialYesPrice: initial,
		FinalYesPrice:   final,
		SettledAt:       settledAt,
		CreatedAt:       time.Now().UTC(),
	}
	if req.Category != nil {
		item.Category = strings.TrimSpace(*req.Category)
	}

	if err := h.Repo.UpsertMarketSettlementHistory(c.Request.Context(), item); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_settlement_upserted", "info", map[string]any{
		"market_id":  item.MarketID,
		"event_id":   item.EventID,
		"outcome":    item.Outcome,
		"settled_at": item.SettledAt.Format(time.RFC3339),
	})
	Ok(c, item, nil)
}

func (h *V2SettlementHandler) labelRates(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	labels := strings.TrimSpace(c.Query("labels"))
	var filter []string
	if labels != "" {
		filter = strings.Split(labels, ",")
	}
	rows, err := h.Repo.ListLabelNoRateStats(c.Request.Context(), filter)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, nil)
}
