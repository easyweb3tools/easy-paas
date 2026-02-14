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
	"polymarket/internal/risk"
)

type V2OpportunityHandler struct {
	Repo repository.Repository
	Risk *risk.Manager
}

func (h *V2OpportunityHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/opportunities")
	group.GET("", h.listOpportunities)
	group.GET("/:id", h.getOpportunity)
	group.POST("/:id/dismiss", h.dismissOpportunity)
	group.POST("/:id/execute", h.createExecutionPlan)
}

func (h *V2OpportunityHandler) listOpportunities(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	strategy := strings.TrimSpace(c.Query("strategy"))
	category := strings.TrimSpace(c.Query("category"))
	minEdge := decimalQueryPtr(c, "min_edge")
	if minEdge != nil {
		// Allow both "0.05" and "5" to mean 5%.
		if minEdge.GreaterThan(decimal.NewFromInt(1)) {
			v := minEdge.Div(decimal.NewFromInt(100))
			minEdge = &v
		}
	}
	minConfidenceRaw := strings.TrimSpace(c.Query("min_confidence"))
	var minConfidence *float64
	if minConfidenceRaw != "" {
		if f, err := decimal.NewFromString(minConfidenceRaw); err == nil {
			v, _ := f.Float64()
			minConfidence = &v
		}
	}
	sortBy := strings.TrimSpace(c.Query("sort_by"))
	order := strings.TrimSpace(strings.ToLower(c.Query("order")))
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)

	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	var strategyPtr *string
	if strategy != "" {
		strategyPtr = &strategy
	}
	var categoryPtr *string
	if category != "" {
		categoryPtr = &category
	}

	orderBy := parseOrder(sortBy, map[string]string{
		"edge_usd":   "edge_usd",
		"edge_pct":   "edge_pct",
		"confidence": "confidence",
		"risk_score": "risk_score",
		"created_at": "created_at",
		"updated_at": "updated_at",
	})
	if orderBy == "" {
		orderBy = "created_at"
	}
	asc := false
	if order == "asc" {
		asc = true
	}

	items, err := h.Repo.ListOpportunities(c.Request.Context(), repository.ListOpportunitiesParams{
		Limit:         limit,
		Offset:        offset,
		Status:        statusPtr,
		StrategyName:  strategyPtr,
		Category:      categoryPtr,
		MinEdgePct:    minEdge,
		MinConfidence: minConfidence,
		OrderBy:       orderBy,
		Asc:           boolPtr(asc),
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountOpportunities(c.Request.Context(), repository.ListOpportunitiesParams{
		Status:        statusPtr,
		StrategyName:  strategyPtr,
		Category:      categoryPtr,
		MinEdgePct:    minEdge,
		MinConfidence: minConfidence,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, total)
	Ok(c, items, meta)
}

func (h *V2OpportunityHandler) getOpportunity(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	item, err := h.Repo.GetOpportunityByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "opportunity not found", nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2OpportunityHandler) dismissOpportunity(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	if err := h.Repo.UpdateOpportunityStatus(c.Request.Context(), id, "cancelled"); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_opportunity_dismissed", "info", map[string]any{
		"opportunity_id": id,
	})
	Ok(c, map[string]any{"id": id, "status": "cancelled"}, nil)
}

func (h *V2OpportunityHandler) createExecutionPlan(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	opp, err := h.Repo.GetOpportunityByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if opp == nil {
		Error(c, http.StatusNotFound, "opportunity not found", nil)
		return
	}
	if strings.TrimSpace(opp.Status) != "" && opp.Status != "active" {
		Error(c, http.StatusConflict, "opportunity not active", map[string]any{"status": opp.Status})
		return
	}
	stratName := ""
	if opp.Strategy.Name != "" {
		stratName = opp.Strategy.Name
	}
	if stratName == "" {
		stratName = "unknown"
	}

	plannedSize := opp.MaxSize
	maxLoss := plannedSize
	var kellyFraction *float64
	warnings := []string{}
	if h.Risk != nil {
		ps, ml, kf, ws := h.Risk.SuggestPlanSizing(c.Request.Context(), *opp, stratName)
		plannedSize = ps
		maxLoss = ml
		kellyFraction = kf
		warnings = append(warnings, ws...)
	}

	plan := &models.ExecutionPlan{
		OpportunityID:   opp.ID,
		Status:          "draft",
		StrategyName:    stratName,
		PlannedSizeUSD:  plannedSize,
		MaxLossUSD:      maxLoss,
		KellyFraction:   kellyFraction,
		Params:          datatypes.JSON([]byte(`{"slippage_tolerance":0.02,"execution_order":"sequential","limit_vs_market":"limit","time_limit_seconds":300}`)),
		PreflightResult: datatypes.JSON([]byte(`{}`)),
		Legs:            addPlanLegSizing(opp.Legs, plannedSize),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	// If opportunity legs are missing, keep plan invalid but insertable.
	if len(plan.Legs) == 0 {
		legsJSON, _ := json.Marshal([]any{})
		plan.Legs = datatypes.JSON(legsJSON)
	}

	if err := h.Repo.InsertExecutionPlan(c.Request.Context(), plan); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}

	// Move opportunity into execution lifecycle once a plan exists.
	_ = h.Repo.UpdateOpportunityStatus(c.Request.Context(), opp.ID, "executing")

	// Seed a PnL record so analytics can show "planned" stats even before settlement.
	_ = h.Repo.UpsertPnLRecord(c.Request.Context(), &models.PnLRecord{
		PlanID:       plan.ID,
		StrategyName: plan.StrategyName,
		ExpectedEdge: opp.EdgePct,
		Outcome:      "pending",
		CreatedAt:    time.Now().UTC(),
	})

	paas.LogBestEffort(c, "polymarket_execution_plan_created", "info", map[string]any{
		"opportunity_id":   opp.ID,
		"plan_id":          plan.ID,
		"strategy":         plan.StrategyName,
		"planned_size_usd": plan.PlannedSizeUSD.String(),
		"max_loss_usd":     plan.MaxLossUSD.String(),
		"warnings":         warnings,
	})

	Ok(c, map[string]any{"plan": plan, "sizing_warnings": warnings}, nil)
}

func addPlanLegSizing(legsJSON []byte, plannedSizeUSD decimal.Decimal) datatypes.JSON {
	if len(legsJSON) == 0 {
		return datatypes.JSON(legsJSON)
	}
	var legs []map[string]any
	if err := json.Unmarshal(legsJSON, &legs); err != nil {
		return datatypes.JSON(legsJSON)
	}
	if len(legs) == 0 {
		return datatypes.JSON(legsJSON)
	}
	perLeg := plannedSizeUSD.Div(decimal.NewFromInt(int64(len(legs))))
	perLegF, _ := perLeg.Float64()
	for i := range legs {
		// Add sizing hints for the UI and future execution engine.
		if _, ok := legs[i]["size_usd"]; !ok {
			legs[i]["size_usd"] = perLegF
		}
		if _, ok := legs[i]["priority"]; !ok {
			legs[i]["priority"] = i + 1
		}
	}
	raw, err := json.Marshal(legs)
	if err != nil {
		return datatypes.JSON(legsJSON)
	}
	return datatypes.JSON(raw)
}

func uint64QueryParam(c *gin.Context, key string) uint64 {
	val := strings.TrimSpace(c.Param(key))
	if val == "" {
		return 0
	}
	var out uint64
	for i := 0; i < len(val); i++ {
		ch := val[i]
		if ch < '0' || ch > '9' {
			return 0
		}
		out = out*10 + uint64(ch-'0')
	}
	return out
}
