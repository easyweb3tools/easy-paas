package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	polymarketclob "polymarket/internal/client/polymarket/clob"
	"polymarket/internal/config"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type Manager struct {
	Config config.RiskConfig
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.Mutex

	lastExposureAt time.Time
	exposureCache  exposureSnapshot

	lastDailyPnLAt time.Time
	dailyPnLCache  decimal.Decimal

	lastStrategyMapAt time.Time
	strategyNameByID  map[uint64]string
}

// Filter applies cheap, deterministic checks. It does not mutate inputs.
func (m *Manager) Filter(opps []models.Opportunity) []models.Opportunity {
	if len(opps) == 0 {
		return nil
	}
	if m == nil || m.Repo == nil {
		return opps
	}
	exp := m.exposures(context.Background(), opps[0].CreatedAt)
	stratMap := m.strategyMap()
	dailyLoss := m.dailyPnL()
	out := make([]models.Opportunity, 0, len(opps))
	filtered := 0
	for _, opp := range opps {
		if m.rejectStale(opp) {
			action := strings.ToLower(strings.TrimSpace(m.Config.StaleDataAction))
			if action == "" {
				action = "block"
			}
			if action == "warn" {
				opp = appendOppWarning(opp, "stale_data")
			} else {
				filtered++
				if m.Logger != nil {
					m.Logger.Debug("risk: reject stale",
						zap.Int("data_age_ms", opp.DataAgeMs),
						zap.Int("threshold_ms", m.Config.MinDataFreshnessMs),
						zap.String("reasoning", opp.Reasoning),
					)
				}
				continue
			}
		}
		if m.rejectDailyLoss(dailyLoss) {
			filtered++
			if m.Logger != nil {
				m.Logger.Debug("risk: reject daily loss",
					zap.String("daily_pnl", dailyLoss.StringFixed(2)),
					zap.Float64("limit_usd", m.Config.MaxDailyLossUSD),
					zap.String("reasoning", opp.Reasoning),
				)
			}
			continue
		}
		if m.rejectExposure(exp, stratMap, opp) {
			filtered++
			if m.Logger != nil {
				m.Logger.Debug("risk: reject exposure",
					zap.String("total_exposure", exp.Total.StringFixed(2)),
					zap.Float64("max_total_usd", m.Config.MaxTotalExposureUSD),
					zap.String("reasoning", opp.Reasoning),
				)
			}
			continue
		}
		out = append(out, opp)
	}
	if m.Logger != nil && (filtered > 0 || len(opps) > 0) {
		m.Logger.Info("risk: filtered opportunities",
			zap.Int("filtered", filtered),
			zap.Int("total", len(opps)),
			zap.Int("passed", len(out)),
		)
	}
	return out
}

func appendOppWarning(opp models.Opportunity, warning string) models.Opportunity {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return opp
	}
	// Copy-on-write to keep Filter non-mutating for callers.
	next := opp

	var items []string
	if len(next.Warnings) > 0 {
		_ = json.Unmarshal(next.Warnings, &items)
	}
	seen := map[string]struct{}{}
	for _, it := range items {
		key := strings.TrimSpace(it)
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	if _, ok := seen[warning]; ok {
		return next
	}
	items = append(items, warning)
	raw, _ := json.Marshal(items)
	next.Warnings = raw
	return next
}

type exposureSnapshot struct {
	Total      decimal.Decimal
	ByStrategy map[string]decimal.Decimal
	ByMarket   map[string]decimal.Decimal
}

func (m *Manager) exposures(ctx context.Context, now time.Time) exposureSnapshot {
	// Cache exposure snapshot for a short window to keep Filter cheap.
	if now.IsZero() {
		now = time.Now().UTC()
	}
	m.mu.Lock()
	if !m.lastExposureAt.IsZero() && now.Sub(m.lastExposureAt) < 10*time.Second {
		c := m.exposureCache
		m.mu.Unlock()
		return c
	}
	m.mu.Unlock()

	statuses := []string{"draft", "preflight_pass", "executing", "partial"}
	if ctx == nil {
		ctx = context.Background()
	}
	plans, err := m.Repo.ListExecutionPlansByStatuses(ctx, statuses, 5000)
	if err != nil {
		return exposureSnapshot{Total: decimal.Zero, ByStrategy: map[string]decimal.Decimal{}, ByMarket: map[string]decimal.Decimal{}}
	}
	out := exposureSnapshot{
		Total:      decimal.Zero,
		ByStrategy: map[string]decimal.Decimal{},
		ByMarket:   map[string]decimal.Decimal{},
	}
	for _, p := range plans {
		out.Total = out.Total.Add(p.PlannedSizeUSD)
		if strings.TrimSpace(p.StrategyName) != "" {
			out.ByStrategy[p.StrategyName] = out.ByStrategy[p.StrategyName].Add(p.PlannedSizeUSD)
		}
		marketIDs := planMarketIDs(p.Legs)
		if len(marketIDs) == 0 {
			continue
		}
		share := p.PlannedSizeUSD.Div(decimal.NewFromInt(int64(len(marketIDs))))
		for _, mid := range marketIDs {
			out.ByMarket[mid] = out.ByMarket[mid].Add(share)
		}
	}

	m.mu.Lock()
	m.lastExposureAt = now
	m.exposureCache = out
	m.mu.Unlock()
	return out
}

func (m *Manager) dailyPnL() decimal.Decimal {
	now := time.Now().UTC()
	m.mu.Lock()
	if !m.lastDailyPnLAt.IsZero() && now.Sub(m.lastDailyPnLAt) < 60*time.Second {
		v := m.dailyPnLCache
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	sum, err := m.Repo.SumRealizedPnLSince(context.Background(), dayStart)
	if err != nil {
		return decimal.Zero
	}
	m.mu.Lock()
	m.lastDailyPnLAt = now
	m.dailyPnLCache = sum
	m.mu.Unlock()
	return sum
}

func (m *Manager) strategyMap() map[uint64]string {
	now := time.Now().UTC()
	m.mu.Lock()
	if m.strategyNameByID != nil && !m.lastStrategyMapAt.IsZero() && now.Sub(m.lastStrategyMapAt) < 5*time.Minute {
		out := m.strategyNameByID
		m.mu.Unlock()
		return out
	}
	m.mu.Unlock()
	items, err := m.Repo.ListStrategies(context.Background())
	if err != nil {
		return map[uint64]string{}
	}
	next := map[uint64]string{}
	for _, it := range items {
		if it.ID == 0 || strings.TrimSpace(it.Name) == "" {
			continue
		}
		next[it.ID] = it.Name
	}
	m.mu.Lock()
	m.lastStrategyMapAt = now
	m.strategyNameByID = next
	m.mu.Unlock()
	return next
}

func (m *Manager) rejectDailyLoss(dayPnL decimal.Decimal) bool {
	if m == nil {
		return false
	}
	if m.Config.MaxDailyLossUSD <= 0 {
		return false
	}
	limit := decimal.NewFromFloat(m.Config.MaxDailyLossUSD)
	// If pnl <= -limit, block new opportunities.
	return dayPnL.LessThanOrEqual(limit.Neg())
}

func (m *Manager) rejectExposure(exp exposureSnapshot, stratByID map[uint64]string, opp models.Opportunity) bool {
	if m == nil {
		return false
	}
	// Total exposure.
	if m.Config.MaxTotalExposureUSD > 0 {
		limit := decimal.NewFromFloat(m.Config.MaxTotalExposureUSD)
		if exp.Total.Add(opp.MaxSize).GreaterThan(limit) {
			return true
		}
	}
	// Strategy exposure (requires StrategyID).
	if m.Config.MaxPerStrategyUSD > 0 && opp.StrategyID != 0 {
		name := stratByID[opp.StrategyID]
		if strings.TrimSpace(name) != "" {
			limit := decimal.NewFromFloat(m.Config.MaxPerStrategyUSD)
			if exp.ByStrategy[name].Add(opp.MaxSize).GreaterThan(limit) {
				return true
			}
		}
	}
	// Per market exposure.
	if m.Config.MaxPerMarketUSD > 0 {
		limit := decimal.NewFromFloat(m.Config.MaxPerMarketUSD)
		marketIDs := oppMarketIDs(opp)
		if len(marketIDs) > 0 {
			share := opp.MaxSize.Div(decimal.NewFromInt(int64(len(marketIDs))))
			for _, mid := range marketIDs {
				if exp.ByMarket[mid].Add(share).GreaterThan(limit) {
					return true
				}
			}
		}
	}
	return false
}

type legMarket struct {
	MarketID string `json:"market_id"`
}

func planMarketIDs(legsJSON []byte) []string {
	if len(legsJSON) == 0 {
		return nil
	}
	var legs []legMarket
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

func oppMarketIDs(opp models.Opportunity) []string {
	if opp.PrimaryMarketID != nil && strings.TrimSpace(*opp.PrimaryMarketID) != "" {
		return []string{strings.TrimSpace(*opp.PrimaryMarketID)}
	}
	if len(opp.MarketIDs) == 0 {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(opp.MarketIDs, &ids); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
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

func (m *Manager) rejectStale(opp models.Opportunity) bool {
	// architecture-v2: MinDataFreshnessMs is a gate. Here DataAgeMs is "max age of inputs at compute time".
	if m == nil {
		return false
	}
	if m.Config.MinDataFreshnessMs <= 0 {
		return false
	}
	if opp.DataAgeMs <= 0 {
		// Unknown, be permissive for now.
		return false
	}
	return opp.DataAgeMs > m.Config.MinDataFreshnessMs
}

// SuggestPlanSizing computes a conservative execution-plan sizing from an opportunity.
// It treats MaxTotalExposureUSD as the "capital base" for DefaultKellyFraction sizing.
func (m *Manager) SuggestPlanSizing(ctx context.Context, opp models.Opportunity, strategyName string) (planned decimal.Decimal, maxLoss decimal.Decimal, kelly *float64, warnings []string) {
	planned = opp.MaxSize
	if planned.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, nil, nil
	}
	if m == nil {
		return planned, planned, nil, nil
	}
	k := m.defaultKellyFraction()
	if k != nil {
		kelly = k
		// If we have a capital base, cap planned size by kelly fraction of it.
		if m.Config.MaxTotalExposureUSD > 0 {
			base := decimal.NewFromFloat(m.Config.MaxTotalExposureUSD)
			kellyCap := base.Mul(decimal.NewFromFloat(*k))
			if kellyCap.GreaterThan(decimal.Zero) && planned.GreaterThan(kellyCap) {
				planned = kellyCap
				warnings = append(warnings, "kelly_cap")
			}
		}
	}

	marketIDs := oppMarketIDs(opp)
	exp := exposureSnapshot{Total: decimal.Zero, ByStrategy: map[string]decimal.Decimal{}, ByMarket: map[string]decimal.Decimal{}}
	if m.Repo != nil {
		exp = m.exposures(ctx, time.Now().UTC())
	}
	planned, warnings = limitPlannedSize(m.Config, exp, strings.TrimSpace(strategyName), marketIDs, planned)
	maxLoss = planned
	return planned, maxLoss, kelly, warnings
}

func (m *Manager) defaultKellyFraction() *float64 {
	if m == nil {
		return nil
	}
	k := m.Config.DefaultKellyFraction
	if k <= 0 {
		return nil
	}
	if m.Config.KellyFractionCap > 0 && k > m.Config.KellyFractionCap {
		k = m.Config.KellyFractionCap
	}
	if k <= 0 {
		return nil
	}
	return &k
}

// limitPlannedSize is a pure helper for sizing caps (testable without a repo).
func limitPlannedSize(cfg config.RiskConfig, exp exposureSnapshot, strategyName string, marketIDs []string, requested decimal.Decimal) (decimal.Decimal, []string) {
	warnings := []string{}
	planned := requested
	if planned.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, warnings
	}

	// Remaining total capacity.
	if cfg.MaxTotalExposureUSD > 0 {
		limit := decimal.NewFromFloat(cfg.MaxTotalExposureUSD)
		remaining := limit.Sub(exp.Total)
		if remaining.LessThan(decimal.Zero) {
			remaining = decimal.Zero
		}
		if planned.GreaterThan(remaining) {
			planned = remaining
			warnings = append(warnings, "total_exposure_cap")
		}
	}

	// Remaining strategy capacity.
	if cfg.MaxPerStrategyUSD > 0 && strings.TrimSpace(strategyName) != "" {
		limit := decimal.NewFromFloat(cfg.MaxPerStrategyUSD)
		remaining := limit.Sub(exp.ByStrategy[strategyName])
		if remaining.LessThan(decimal.Zero) {
			remaining = decimal.Zero
		}
		if planned.GreaterThan(remaining) {
			planned = remaining
			warnings = append(warnings, "strategy_exposure_cap")
		}
	}

	// Remaining per-market capacity (assume equal split across markets).
	if cfg.MaxPerMarketUSD > 0 && len(marketIDs) > 0 {
		limit := decimal.NewFromFloat(cfg.MaxPerMarketUSD)
		minMaxPlanned := decimal.NewFromFloat(0)
		for i, mid := range marketIDs {
			cur := exp.ByMarket[strings.TrimSpace(mid)]
			remaining := limit.Sub(cur)
			if remaining.LessThan(decimal.Zero) {
				remaining = decimal.Zero
			}
			maxPlannedForThisMarket := remaining.Mul(decimal.NewFromInt(int64(len(marketIDs))))
			if i == 0 || maxPlannedForThisMarket.LessThan(minMaxPlanned) {
				minMaxPlanned = maxPlannedForThisMarket
			}
		}
		if planned.GreaterThan(minMaxPlanned) {
			planned = minMaxPlanned
			warnings = append(warnings, "market_exposure_cap")
		}
	}

	if planned.LessThan(decimal.Zero) {
		planned = decimal.Zero
	}
	return planned, warnings
}

// CalculateKelly is kept for later; not used in MVP wiring yet.
func (m *Manager) CalculateKelly(winProb, winAmount, lossAmount float64) float64 {
	if winAmount <= 0 {
		return 0
	}
	k := (winProb*winAmount - (1.0-winProb)*lossAmount) / winAmount
	if m != nil && m.Config.KellyFractionCap > 0 && k > m.Config.KellyFractionCap {
		return m.Config.KellyFractionCap
	}
	if k < 0 {
		return 0
	}
	return k
}

// Now is factored for testability later.
func nowUTC() time.Time { return time.Now().UTC() }

type PreflightResult struct {
	Passed bool             `json:"passed"`
	Checks []PreflightCheck `json:"checks"`
}

type PreflightCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass|warn|fail
	Value  any    `json:"value,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

type planLeg struct {
	TokenID        string   `json:"token_id"`
	TargetPrice    *float64 `json:"target_price"`
	CurrentBestAsk *float64 `json:"current_best_ask"`
	SizeUSD        *float64 `json:"size_usd"`
}

func (m *Manager) PreflightPlan(ctx context.Context, planID uint64) (*PreflightResult, error) {
	if m == nil || m.Repo == nil {
		return nil, nil
	}
	plan, err := m.Repo.GetExecutionPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	result, status := m.preflight(ctx, *plan)
	raw, _ := json.Marshal(result)
	_ = m.Repo.UpdateExecutionPlanPreflight(ctx, planID, status, raw)
	return &result, nil
}

func (m *Manager) preflight(ctx context.Context, plan models.ExecutionPlan) (PreflightResult, string) {
	now := time.Now().UTC()
	res := PreflightResult{Passed: true}
	status := "preflight_pass"

	var legs []planLeg
	_ = json.Unmarshal(plan.Legs, &legs)
	tokenIDs := make([]string, 0, len(legs))
	seen := map[string]struct{}{}
	for _, leg := range legs {
		id := strings.TrimSpace(leg.TokenID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		tokenIDs = append(tokenIDs, id)
	}
	if len(tokenIDs) == 0 {
		res.Passed = false
		res.Checks = append(res.Checks, PreflightCheck{Name: "legs", Status: "fail", Msg: "no token_id in legs"})
		return res, "preflight_fail"
	}

	healthRows, _ := m.Repo.ListMarketDataHealthByTokenIDs(ctx, tokenIDs)
	bookRows, _ := m.Repo.ListOrderbookLatestByTokenIDs(ctx, tokenIDs)
	healthByID := map[string]models.MarketDataHealth{}
	for _, h := range healthRows {
		healthByID[h.TokenID] = h
	}
	bookByID := map[string]models.OrderbookLatest{}
	for _, b := range bookRows {
		bookByID[b.TokenID] = b
	}

	// Params (slippage tolerance + capital max) are optional; treat missing as defaults.
	type planParams struct {
		SlippageTolerance *float64 `json:"slippage_tolerance"`
		MaxCapital        *float64 `json:"max_capital"`
	}
	var pp planParams
	if len(plan.Params) > 0 {
		_ = json.Unmarshal(plan.Params, &pp)
	}
	slippageTol := 0.02
	if pp.SlippageTolerance != nil && *pp.SlippageTolerance >= 0 {
		slippageTol = *pp.SlippageTolerance
	}

	// Freshness check.
	maxAge := time.Duration(0)
	for _, tokenID := range tokenIDs {
		book := bookByID[tokenID]
		if book.UpdatedAt.IsZero() {
			continue
		}
		age := now.Sub(book.UpdatedAt)
		if age > maxAge {
			maxAge = age
		}
	}
	if m.Config.MinDataFreshnessMs > 0 {
		if maxAge > time.Duration(m.Config.MinDataFreshnessMs)*time.Millisecond {
			res.Passed = false
			res.Checks = append(res.Checks, PreflightCheck{
				Name:   "data_freshness",
				Status: "fail",
				Value:  maxAge.String(),
				Msg:    fmt.Sprintf("max book age %s exceeds min freshness %dms", maxAge.String(), m.Config.MinDataFreshnessMs),
			})
		} else {
			res.Checks = append(res.Checks, PreflightCheck{
				Name:   "data_freshness",
				Status: "pass",
				Value:  maxAge.String(),
			})
		}
	}

	// Jump/spread warnings.
	warned := 0
	for _, tokenID := range tokenIDs {
		h := healthByID[tokenID]
		if h.PriceJumpBps != nil && *h.PriceJumpBps > 300 {
			warned++
			res.Checks = append(res.Checks, PreflightCheck{Name: "price_jump", Status: "warn", Value: *h.PriceJumpBps, Msg: tokenID})
		}
		if h.SpreadBps != nil && *h.SpreadBps > 400 {
			warned++
			res.Checks = append(res.Checks, PreflightCheck{Name: "spread", Status: "warn", Value: *h.SpreadBps, Msg: tokenID})
		}
	}
	if warned == 0 {
		res.Checks = append(res.Checks, PreflightCheck{Name: "market_microstructure", Status: "pass"})
	}

	// Capital checks: planned size should not exceed remaining capacity.
	if pp.MaxCapital != nil && *pp.MaxCapital > 0 {
		maxCap := decimal.NewFromFloat(*pp.MaxCapital)
		if plan.PlannedSizeUSD.GreaterThan(maxCap) {
			res.Passed = false
			res.Checks = append(res.Checks, PreflightCheck{Name: "capital_limit", Status: "fail", Value: plan.PlannedSizeUSD.StringFixed(2), Msg: fmt.Sprintf("planned_size_usd exceeds max_capital %.2f", *pp.MaxCapital)})
		} else {
			res.Checks = append(res.Checks, PreflightCheck{Name: "capital_limit", Status: "pass", Value: plan.PlannedSizeUSD.StringFixed(2)})
		}
	} else if m.Config.MaxTotalExposureUSD > 0 {
		exp := m.exposures(ctx, now)
		limit := decimal.NewFromFloat(m.Config.MaxTotalExposureUSD)
		remaining := limit.Sub(exp.Total)
		if remaining.LessThan(decimal.Zero) {
			remaining = decimal.Zero
		}
		if plan.PlannedSizeUSD.GreaterThan(remaining) {
			res.Passed = false
			res.Checks = append(res.Checks, PreflightCheck{Name: "capital_limit", Status: "fail", Value: plan.PlannedSizeUSD.StringFixed(2), Msg: fmt.Sprintf("planned_size_usd exceeds remaining_total_capacity %s", remaining.StringFixed(2))})
		} else {
			res.Checks = append(res.Checks, PreflightCheck{Name: "capital_limit", Status: "pass", Value: remaining.StringFixed(2)})
		}
	}

	// Edge/slippage re-check from latest books: ensure current best ask doesn't drift beyond tolerance from leg targets.
	maxSlippage := 0.0
	failedSlippage := false
	for _, leg := range legs {
		tokenID := strings.TrimSpace(leg.TokenID)
		if tokenID == "" {
			continue
		}
		book := bookByID[tokenID]
		bestAsk, bestAskSize, ok := bestAskFromBook(book)
		if !ok {
			continue
		}
		target := leg.TargetPrice
		if target == nil {
			target = leg.CurrentBestAsk
		}
		if target == nil || *target <= 0 {
			continue
		}
		sl := (bestAsk.InexactFloat64() - *target) / *target
		if sl < 0 {
			sl = 0
		}
		if sl > maxSlippage {
			maxSlippage = sl
		}
		if slippageTol > 0 && sl > slippageTol {
			failedSlippage = true
			res.Checks = append(res.Checks, PreflightCheck{
				Name:   "edge_recheck",
				Status: "fail",
				Value:  fmt.Sprintf("%.4f", sl),
				Msg:    fmt.Sprintf("token=%s best_ask=%s target=%.4f", tokenID, bestAsk.StringFixed(4), *target),
			})
		}

		// Thin-book warning: if size_usd is present, check that the top ask can cover it.
		if leg.SizeUSD != nil && *leg.SizeUSD > 0 && bestAsk.GreaterThan(decimal.Zero) {
			needShares := decimal.NewFromFloat(*leg.SizeUSD).Div(bestAsk)
			if bestAskSize.GreaterThan(decimal.Zero) && needShares.GreaterThan(bestAskSize) {
				res.Checks = append(res.Checks, PreflightCheck{
					Name:   "thin_book",
					Status: "warn",
					Value:  needShares.StringFixed(2),
					Msg:    fmt.Sprintf("token=%s need_shares=%s best_ask_size=%s", tokenID, needShares.StringFixed(2), bestAskSize.StringFixed(2)),
				})
			}
		}
	}
	if !failedSlippage {
		res.Checks = append(res.Checks, PreflightCheck{Name: "edge_recheck", Status: "pass", Value: fmt.Sprintf("%.4f", maxSlippage)})
	}

	// MM behavior warnings based on recent signals (best-effort, cheap).
	{
		since := now.Add(-1 * time.Hour)
		sigs, err := m.Repo.ListSignals(ctx, repository.ListSignalsParams{
			Limit:   5000,
			Offset:  0,
			Type:    strPtr("mm_inventory_skew"),
			Since:   &since,
			OrderBy: "created_at",
			Asc:     boolPtr(false),
		})
		if err == nil && len(sigs) > 0 {
			hit := 0
			for _, s := range sigs {
				if s.TokenID == nil {
					continue
				}
				tid := strings.TrimSpace(*s.TokenID)
				if tid == "" {
					continue
				}
				if _, ok := seen[tid]; ok {
					hit++
				}
			}
			if hit > 0 {
				res.Checks = append(res.Checks, PreflightCheck{Name: "mm_behavior", Status: "warn", Value: hit, Msg: "mm_inventory_skew_signals_recent"})
			} else {
				res.Checks = append(res.Checks, PreflightCheck{Name: "mm_behavior", Status: "pass"})
			}
		} else {
			res.Checks = append(res.Checks, PreflightCheck{Name: "mm_behavior", Status: "pass"})
		}
	}

	if !res.Passed {
		status = "preflight_fail"
	}
	// Store status in plan even if fail; execution gating is handled by RequirePreflightPass later.
	return res, status
}

func mustJSON(v any) datatypes.JSON {
	raw, _ := json.Marshal(v)
	return datatypes.JSON(raw)
}

func bestAskFromBook(book models.OrderbookLatest) (decimal.Decimal, decimal.Decimal, bool) {
	if book.BestAsk != nil && *book.BestAsk > 0 && len(book.AsksJSON) == 0 {
		// Size unknown.
		return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
	}
	if len(book.AsksJSON) == 0 {
		if book.BestAsk != nil && *book.BestAsk > 0 {
			return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
		}
		return decimal.Zero, decimal.Zero, false
	}
	var asks []polymarketclob.Order
	if err := json.Unmarshal(book.AsksJSON, &asks); err != nil || len(asks) == 0 {
		if book.BestAsk != nil && *book.BestAsk > 0 {
			return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
		}
		return decimal.Zero, decimal.Zero, false
	}
	price := asks[0].Price
	size := asks[0].Size
	if price.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, false
	}
	return price, size, true
}

func boolPtr(v bool) *bool { return &v }

func strPtr(s string) *string {
	val := strings.TrimSpace(s)
	if val == "" {
		return nil
	}
	return &val
}
