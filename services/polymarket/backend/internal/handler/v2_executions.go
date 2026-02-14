package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"polymarket/internal/models"
	"polymarket/internal/paas"
	"polymarket/internal/repository"
	"polymarket/internal/risk"
)

type V2ExecutionHandler struct {
	Repo repository.Repository
	Risk *risk.Manager
}

type planLegTarget struct {
	TokenID        string   `json:"token_id"`
	TargetPrice    *float64 `json:"target_price"`
	CurrentBestAsk *float64 `json:"current_best_ask"`
}

type planLegMarket struct {
	MarketID string `json:"market_id"`
}

func (h *V2ExecutionHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/executions")
	group.GET("", h.list)
	group.GET("/:id", h.get)
	group.GET("/:id/pnl", h.getPnL)
	group.POST("/:id/preflight", h.preflight)
	group.POST("/:id/fill", h.addFill)
	group.POST("/:id/mark-executing", h.markExecuting)
	group.POST("/:id/mark-executed", h.markExecuted)
	group.POST("/:id/cancel", h.cancel)
	group.PUT("/:id/pnl", h.upsertPnL)
	group.POST("/:id/settle", h.settle)
}

func (h *V2ExecutionHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	items, err := h.Repo.ListExecutionPlans(c.Request.Context(), repository.ListExecutionPlansParams{
		Limit:   limit,
		Offset:  offset,
		Status:  statusPtr,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountExecutionPlans(c.Request.Context(), repository.ListExecutionPlansParams{
		Status: statusPtr,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, total)
	Ok(c, items, meta)
}

func (h *V2ExecutionHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	item, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2ExecutionHandler) getPnL(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	rec, err := h.Repo.GetPnLRecordByPlanID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if rec == nil {
		Error(c, http.StatusNotFound, "pnl record not found", nil)
		return
	}
	Ok(c, rec, nil)
}

func (h *V2ExecutionHandler) preflight(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	if h.Risk == nil {
		Error(c, http.StatusServiceUnavailable, "risk manager unavailable", nil)
		return
	}
	result, err := h.Risk.PreflightPlan(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if result == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	// Best-effort failure reason journaling for analytics.
	if !result.Passed {
		plan, _ := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
		if plan != nil {
			reason := preflightFailureReason(*result)
			if reason != "" {
				rec, _ := h.Repo.GetPnLRecordByPlanID(c.Request.Context(), id)
				if rec == nil {
					rec = &models.PnLRecord{
						PlanID:       id,
						StrategyName: plan.StrategyName,
						ExpectedEdge: decimal.Zero,
						Outcome:      "pending",
						CreatedAt:    time.Now().UTC(),
					}
				}
				r := reason
				rec.FailureReason = &r
				if strings.TrimSpace(rec.Outcome) == "" {
					rec.Outcome = "pending"
				}
				_ = h.Repo.UpsertPnLRecord(c.Request.Context(), rec)
			}
		}
	}
	Ok(c, result, nil)

	paas.LogBestEffort(c, "polymarket_execution_preflight", "info", map[string]any{
		"plan_id": id,
		"passed":  result.Passed,
		"checks":  len(result.Checks),
	})
}

func (h *V2ExecutionHandler) markExecuting(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if plan == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	if h.Risk != nil && h.Risk.Config.RequirePreflightPass {
		if plan.Status != "preflight_pass" && plan.Status != "executing" && plan.Status != "partial" {
			Error(c, http.StatusConflict, "preflight required", map[string]any{"status": plan.Status})
			return
		}
	}
	_ = h.Repo.UpdateExecutionPlanStatus(c.Request.Context(), id, "executing")
	_ = h.Repo.UpdateOpportunityStatus(c.Request.Context(), plan.OpportunityID, "executing")
	paas.LogBestEffort(c, "polymarket_execution_mark_executing", "info", map[string]any{
		"plan_id":        id,
		"opportunity_id": plan.OpportunityID,
	})
	Ok(c, map[string]any{"id": id, "status": "executing"}, nil)
}

func (h *V2ExecutionHandler) markExecuted(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if plan == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	now := time.Now().UTC()
	_ = h.Repo.UpdateExecutionPlanExecutedAt(c.Request.Context(), id, "executed", &now)
	_ = h.Repo.UpdateOpportunityStatus(c.Request.Context(), plan.OpportunityID, "executed")
	paas.LogBestEffort(c, "polymarket_execution_mark_executed", "info", map[string]any{
		"plan_id":        id,
		"opportunity_id": plan.OpportunityID,
		"executed_at":    now.Format(time.RFC3339),
	})
	Ok(c, map[string]any{"id": id, "status": "executed", "executed_at": now}, nil)
}

func (h *V2ExecutionHandler) cancel(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, _ := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	_ = h.Repo.UpdateExecutionPlanStatus(c.Request.Context(), id, "cancelled")
	if plan != nil {
		_ = h.Repo.UpdateOpportunityStatus(c.Request.Context(), plan.OpportunityID, "cancelled")
	}
	details := map[string]any{"plan_id": id}
	if plan != nil {
		details["opportunity_id"] = plan.OpportunityID
	}
	paas.LogBestEffort(c, "polymarket_execution_cancelled", "info", details)
	Ok(c, map[string]any{"id": id, "status": "cancelled"}, nil)
}

type upsertPnLRequest struct {
	ExpectedEdge  *string `json:"expected_edge"`
	RealizedPnL   *string `json:"realized_pnl"`
	RealizedROI   *string `json:"realized_roi"`
	SlippageLoss  *string `json:"slippage_loss"`
	Outcome       *string `json:"outcome"`
	FailureReason *string `json:"failure_reason"`
	SettledAtRFC  *string `json:"settled_at"`
	Notes         *string `json:"notes"`
}

func (h *V2ExecutionHandler) upsertPnL(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if plan == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	var req upsertPnLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	rec, _ := h.Repo.GetPnLRecordByPlanID(c.Request.Context(), id)
	if rec == nil {
		rec = &models.PnLRecord{
			PlanID:       id,
			StrategyName: plan.StrategyName,
			ExpectedEdge: decimal.Zero,
			Outcome:      "pending",
			CreatedAt:    time.Now().UTC(),
		}
	}
	if req.ExpectedEdge != nil && strings.TrimSpace(*req.ExpectedEdge) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.ExpectedEdge)); err == nil {
			rec.ExpectedEdge = v
		}
	}
	if req.RealizedPnL != nil && strings.TrimSpace(*req.RealizedPnL) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.RealizedPnL)); err == nil {
			rec.RealizedPnL = &v
		}
	}
	if req.RealizedROI != nil && strings.TrimSpace(*req.RealizedROI) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.RealizedROI)); err == nil {
			rec.RealizedROI = &v
		}
	}
	if req.SlippageLoss != nil && strings.TrimSpace(*req.SlippageLoss) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(*req.SlippageLoss)); err == nil {
			rec.SlippageLoss = &v
		}
	}
	if req.Outcome != nil {
		rec.Outcome = strings.TrimSpace(*req.Outcome)
	}
	if req.FailureReason != nil {
		val := strings.TrimSpace(*req.FailureReason)
		if val == "" {
			rec.FailureReason = nil
		} else {
			rec.FailureReason = &val
		}
	}
	if req.SettledAtRFC != nil && strings.TrimSpace(*req.SettledAtRFC) != "" {
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.SettledAtRFC)); err == nil {
			t := ts.UTC()
			rec.SettledAt = &t
		}
	}
	if req.Notes != nil {
		val := strings.TrimSpace(*req.Notes)
		if val == "" {
			rec.Notes = nil
		} else {
			rec.Notes = &val
		}
	}
	if err := h.Repo.UpsertPnLRecord(c.Request.Context(), rec); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_pnl_upserted", "info", map[string]any{
		"plan_id": id,
		"outcome": rec.Outcome,
	})
	Ok(c, rec, nil)
}

type settleRequest struct {
	// Optional overrides if market_settlement_history is not present yet.
	MarketOutcomes map[string]string `json:"market_outcomes"` // market_id -> YES|NO
	SettledAtRFC   *string           `json:"settled_at"`
}

func (h *V2ExecutionHandler) settle(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if plan == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	var req settleRequest
	_ = c.ShouldBindJSON(&req)

	settledAt := time.Now().UTC()
	if req.SettledAtRFC != nil && strings.TrimSpace(*req.SettledAtRFC) != "" {
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.SettledAtRFC)); err == nil {
			settledAt = ts.UTC()
		}
	}

	fills, err := h.Repo.ListFillsByPlanID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if len(fills) == 0 {
		Error(c, http.StatusConflict, "no fills for plan", nil)
		return
	}

	// Resolve outcomes by market_id via:
	// 1) request overrides
	// 2) market_settlement_history lookup
	marketIDs := planMarketIDsFromLegs(plan.Legs)
	outcomes := map[string]string{}
	for k, v := range req.MarketOutcomes {
		mid := strings.TrimSpace(k)
		val := strings.ToUpper(strings.TrimSpace(v))
		if mid != "" && (val == "YES" || val == "NO") {
			outcomes[mid] = val
		}
	}
	if len(marketIDs) > 0 {
		rows, _ := h.Repo.ListMarketSettlementHistoryByMarketIDs(c.Request.Context(), marketIDs)
		for _, r := range rows {
			mid := strings.TrimSpace(r.MarketID)
			if mid == "" {
				continue
			}
			if _, ok := outcomes[mid]; ok {
				continue
			}
			val := strings.ToUpper(strings.TrimSpace(r.Outcome))
			if val == "YES" || val == "NO" {
				outcomes[mid] = val
			}
		}
	}
	missing := make([]string, 0)
	for _, mid := range marketIDs {
		if _, ok := outcomes[mid]; !ok {
			missing = append(missing, mid)
		}
	}
	if len(missing) > 0 {
		Error(c, http.StatusConflict, "missing market outcomes", map[string]any{"missing_market_ids": missing})
		return
	}

	tokenIDs := make([]string, 0, len(fills))
	for _, f := range fills {
		if strings.TrimSpace(f.TokenID) != "" {
			tokenIDs = append(tokenIDs, strings.TrimSpace(f.TokenID))
		}
	}
	toks, err := h.Repo.ListTokensByIDs(c.Request.Context(), tokenIDs)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	tokenByID := map[string]models.Token{}
	for _, t := range toks {
		tokenByID[t.ID] = t
	}

	totalCost := decimal.Zero
	totalPnL := decimal.Zero
	for _, f := range fills {
		tok := tokenByID[f.TokenID]
		mid := strings.TrimSpace(tok.MarketID)
		if mid == "" {
			continue
		}
		outcome := outcomes[mid]
		payout := decimal.Zero
		dir := strings.ToUpper(strings.TrimSpace(f.Direction))
		switch dir {
		case "BUY_YES":
			if outcome == "YES" {
				payout = decimal.NewFromInt(1)
			}
		case "BUY_NO":
			if outcome == "NO" {
				payout = decimal.NewFromInt(1)
			}
		default:
			// Ignore unknown directions for now.
			continue
		}
		cost := f.AvgPrice.Mul(f.FilledSize).Add(f.Fee)
		pnl := payout.Sub(f.AvgPrice).Mul(f.FilledSize).Sub(f.Fee)
		totalCost = totalCost.Add(cost)
		totalPnL = totalPnL.Add(pnl)
	}
	var roi *decimal.Decimal
	if totalCost.GreaterThan(decimal.Zero) {
		v := totalPnL.Div(totalCost)
		roi = &v
	}

	rec, _ := h.Repo.GetPnLRecordByPlanID(c.Request.Context(), id)
	if rec == nil {
		rec = &models.PnLRecord{
			PlanID:       id,
			StrategyName: plan.StrategyName,
			ExpectedEdge: decimal.Zero,
			Outcome:      "pending",
			CreatedAt:    time.Now().UTC(),
		}
	}
	rec.RealizedPnL = &totalPnL
	rec.RealizedROI = roi
	rec.SettledAt = &settledAt
	if totalPnL.GreaterThan(decimal.Zero) {
		rec.Outcome = "win"
	} else if totalPnL.LessThan(decimal.Zero) {
		rec.Outcome = "loss"
	} else {
		rec.Outcome = "partial"
	}
	if err := h.Repo.UpsertPnLRecord(c.Request.Context(), rec); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	// Settlement implies the plan is done from an accounting perspective.
	if plan.Status != "cancelled" && plan.Status != "failed" {
		now := time.Now().UTC()
		_ = h.Repo.UpdateExecutionPlanExecutedAt(c.Request.Context(), id, "executed", &now)
		_ = h.Repo.UpdateOpportunityStatus(c.Request.Context(), plan.OpportunityID, "executed")
	}
	paas.LogBestEffort(c, "polymarket_execution_settled", "info", map[string]any{
		"plan_id":        id,
		"opportunity_id": plan.OpportunityID,
		"outcome":        rec.Outcome,
		"settled_at":     settledAt.Format(time.RFC3339),
	})
	Ok(c, rec, nil)
}

type addFillRequest struct {
	TokenID     string `json:"token_id"`
	Direction   string `json:"direction"`
	FilledSize  string `json:"filled_size"`
	AvgPrice    string `json:"avg_price"`
	Fee         string `json:"fee"`
	Slippage    string `json:"slippage"`
	FilledAtRFC string `json:"filled_at"`
}

func (h *V2ExecutionHandler) addFill(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	plan, err := h.Repo.GetExecutionPlanByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if plan == nil {
		Error(c, http.StatusNotFound, "execution plan not found", nil)
		return
	}
	if h.Risk != nil && h.Risk.Config.RequirePreflightPass {
		switch plan.Status {
		case "preflight_pass", "executing", "partial":
			// ok
		default:
			Error(c, http.StatusConflict, "preflight required", map[string]any{"status": plan.Status})
			return
		}
	}
	var req addFillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	req.TokenID = strings.TrimSpace(req.TokenID)
	req.Direction = strings.TrimSpace(req.Direction)
	if req.TokenID == "" || req.Direction == "" {
		Error(c, http.StatusBadRequest, "token_id and direction required", nil)
		return
	}
	filledSize, err := decimal.NewFromString(strings.TrimSpace(req.FilledSize))
	if err != nil {
		Error(c, http.StatusBadRequest, "invalid filled_size", nil)
		return
	}
	avgPrice, err := decimal.NewFromString(strings.TrimSpace(req.AvgPrice))
	if err != nil {
		Error(c, http.StatusBadRequest, "invalid avg_price", nil)
		return
	}
	fee := decimal.Zero
	if strings.TrimSpace(req.Fee) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(req.Fee)); err == nil {
			fee = v
		}
	}
	var slippage *decimal.Decimal
	if strings.TrimSpace(req.Slippage) != "" {
		if v, err := decimal.NewFromString(strings.TrimSpace(req.Slippage)); err == nil {
			slippage = &v
		}
	}
	filledAt := time.Now().UTC()
	if strings.TrimSpace(req.FilledAtRFC) != "" {
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(req.FilledAtRFC)); err == nil {
			filledAt = ts.UTC()
		}
	}

	item := &models.Fill{
		PlanID:     id,
		TokenID:    req.TokenID,
		Direction:  req.Direction,
		FilledSize: filledSize,
		AvgPrice:   avgPrice,
		Fee:        fee,
		Slippage:   slippage,
		FilledAt:   filledAt,
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.Repo.InsertFill(c.Request.Context(), item); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_fill_added", "info", map[string]any{
		"plan_id":   id,
		"token_id":  item.TokenID,
		"direction": item.Direction,
		"size":      item.FilledSize.String(),
		"avg_price": item.AvgPrice.String(),
		"fee":       item.Fee.String(),
		"filled_at": item.FilledAt.Format(time.RFC3339),
	})

	// Update plan/opportunity status based on fill coverage.
	_ = h.updateStatusFromFills(c.Request.Context(), *plan)

	// Update slippage loss (MVP). If the request does not specify slippage, try computing from plan legs.
	slippageLossDelta := decimal.Zero
	if item.Slippage != nil {
		slippageLossDelta = item.Slippage.Mul(item.FilledSize)
	} else if plan != nil {
		if target := findTargetPrice(plan.Legs, item.TokenID); target != nil {
			perShare := item.AvgPrice.Sub(*target)
			slippageLossDelta = perShare.Mul(item.FilledSize)
		}
	}
	if !slippageLossDelta.IsZero() {
		rec, _ := h.Repo.GetPnLRecordByPlanID(c.Request.Context(), id)
		if rec == nil {
			strategyName := ""
			if plan != nil {
				strategyName = plan.StrategyName
			}
			rec = &models.PnLRecord{
				PlanID:       id,
				StrategyName: strategyName,
				ExpectedEdge: decimal.Zero,
				Outcome:      "pending",
				CreatedAt:    time.Now().UTC(),
			}
		}
		// Keep ExpectedEdge and realized fields as-is; only accumulate slippage loss here.
		if rec.SlippageLoss == nil {
			v := slippageLossDelta
			rec.SlippageLoss = &v
		} else {
			v := rec.SlippageLoss.Add(slippageLossDelta)
			rec.SlippageLoss = &v
		}
		if strings.TrimSpace(rec.Outcome) == "" {
			rec.Outcome = "pending"
		}
		_ = h.Repo.UpsertPnLRecord(c.Request.Context(), rec)
	}

	Ok(c, item, nil)
}

func (h *V2ExecutionHandler) updateStatusFromFills(ctx context.Context, plan models.ExecutionPlan) error {
	if h == nil || h.Repo == nil {
		return nil
	}
	switch plan.Status {
	case "cancelled", "failed":
		return nil
	}
	fills, err := h.Repo.ListFillsByPlanID(ctx, plan.ID)
	if err != nil {
		return err
	}
	totalCost := decimal.Zero
	costByToken := map[string]decimal.Decimal{}
	for _, f := range fills {
		cost := f.AvgPrice.Mul(f.FilledSize).Add(f.Fee)
		totalCost = totalCost.Add(cost)
		tid := strings.TrimSpace(f.TokenID)
		if tid != "" {
			costByToken[tid] = costByToken[tid].Add(cost)
		}
	}
	if totalCost.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	// Prefer per-leg completion semantics if legs include size_usd.
	var legs []map[string]any
	_ = json.Unmarshal(plan.Legs, &legs)
	expectedByToken := map[string]decimal.Decimal{}
	hasLegSizing := false
	for _, leg := range legs {
		rawID, _ := leg["token_id"].(string)
		tokenID := strings.TrimSpace(rawID)
		if tokenID == "" {
			continue
		}
		rawSize, ok := leg["size_usd"]
		if !ok {
			continue
		}
		switch v := rawSize.(type) {
		case float64:
			if v > 0 {
				expectedByToken[tokenID] = expectedByToken[tokenID].Add(decimal.NewFromFloat(v))
				hasLegSizing = true
			}
		case int:
			if v > 0 {
				expectedByToken[tokenID] = expectedByToken[tokenID].Add(decimal.NewFromInt(int64(v)))
				hasLegSizing = true
			}
		}
	}

	if hasLegSizing && len(expectedByToken) > 0 {
		allDone := true
		for tokenID, expected := range expectedByToken {
			if expected.LessThanOrEqual(decimal.Zero) {
				continue
			}
			filled := costByToken[tokenID]
			threshold := expected.Mul(decimal.NewFromFloat(0.98))
			if filled.LessThan(threshold) {
				allDone = false
				break
			}
		}
		if allDone {
			now := time.Now().UTC()
			_ = h.Repo.UpdateExecutionPlanExecutedAt(ctx, plan.ID, "executed", &now)
			_ = h.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executed")
			return nil
		}
		// Not all legs done => partial once any fill exists.
		if plan.Status != "partial" {
			_ = h.Repo.UpdateExecutionPlanStatus(ctx, plan.ID, "partial")
			_ = h.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executing")
		}
		return nil
	}

	// Fallback: If we have essentially the full planned stake spent, mark executed.
	planned := plan.PlannedSizeUSD
	if planned.GreaterThan(decimal.Zero) {
		threshold := planned.Mul(decimal.NewFromFloat(0.98))
		if totalCost.GreaterThanOrEqual(threshold) {
			now := time.Now().UTC()
			_ = h.Repo.UpdateExecutionPlanExecutedAt(ctx, plan.ID, "executed", &now)
			_ = h.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executed")
			return nil
		}
	}
	if plan.Status != "partial" {
		_ = h.Repo.UpdateExecutionPlanStatus(ctx, plan.ID, "partial")
		_ = h.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executing")
	}
	return nil
}

func findTargetPrice(legsJSON []byte, tokenID string) *decimal.Decimal {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" || len(legsJSON) == 0 {
		return nil
	}
	var legs []planLegTarget
	if err := json.Unmarshal(legsJSON, &legs); err != nil {
		return nil
	}
	for _, leg := range legs {
		if strings.TrimSpace(leg.TokenID) != tokenID {
			continue
		}
		if leg.TargetPrice != nil {
			v := decimal.NewFromFloat(*leg.TargetPrice)
			return &v
		}
		if leg.CurrentBestAsk != nil {
			v := decimal.NewFromFloat(*leg.CurrentBestAsk)
			return &v
		}
	}
	return nil
}

func planMarketIDsFromLegs(legsJSON []byte) []string {
	if len(legsJSON) == 0 {
		return nil
	}
	var legs []planLegMarket
	if err := json.Unmarshal(legsJSON, &legs); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(legs))
	for _, leg := range legs {
		id := strings.TrimSpace(leg.MarketID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func preflightFailureReason(res risk.PreflightResult) string {
	for _, chk := range res.Checks {
		if chk.Status != "fail" {
			continue
		}
		switch chk.Name {
		case "data_freshness":
			return "latency"
		case "edge_recheck":
			return "price_jump"
		case "capital_limit":
			return "rule_mismatch"
		default:
			// keep looking
		}
	}
	return ""
}
