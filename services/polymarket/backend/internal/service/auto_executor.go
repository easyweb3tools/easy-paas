package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/config"
	"polymarket/internal/models"
	"polymarket/internal/repository"
	"polymarket/internal/risk"
)

type AutoExecutorService struct {
	Repo   repository.Repository
	Risk   *risk.Manager
	Logger *zap.Logger
	Config config.AutoExecutorConfig
	Flags  *SystemSettingsService
}

func (s *AutoExecutorService) Run(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	interval := s.Config.ScanInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := s.scanOnce(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("auto executor scan failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *AutoExecutorService) scanOnce(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeatureAutoExecutor, false) {
		return nil
	}
	maxOpps := s.Config.MaxOpportunities
	if maxOpps <= 0 {
		maxOpps = 100
	}
	active := "active"
	opps, err := s.Repo.ListOpportunities(ctx, repository.ListOpportunitiesParams{
		Status:  &active,
		Limit:   maxOpps,
		Offset:  0,
		OrderBy: "created_at",
		Asc:     boolPtrAuto(true),
	})
	if err != nil {
		return err
	}
	for _, opp := range opps {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.processOpportunity(ctx, opp); err != nil && s.Logger != nil {
			s.Logger.Warn("auto executor skipped opportunity", zap.Uint64("opportunity_id", opp.ID), zap.Error(err))
		}
	}
	return nil
}

func (s *AutoExecutorService) processOpportunity(ctx context.Context, opp models.Opportunity) error {
	strategyName := strings.TrimSpace(opp.Strategy.Name)
	if strategyName == "" {
		return nil
	}
	rule, err := s.Repo.GetExecutionRuleByStrategyName(ctx, strategyName)
	if err != nil || rule == nil || !rule.AutoExecute {
		return err
	}

	minConfidence := rule.MinConfidence
	if minConfidence <= 0 {
		minConfidence = s.Config.DefaultMinConfidence
		if minConfidence <= 0 {
			minConfidence = 0.8
		}
	}
	if opp.Confidence < minConfidence {
		return nil
	}

	minEdge := rule.MinEdgePct
	if minEdge.LessThanOrEqual(decimal.Zero) {
		minEdge = decimal.NewFromFloat(s.Config.DefaultMinEdgePct)
		if minEdge.LessThanOrEqual(decimal.Zero) {
			minEdge = decimal.NewFromFloat(0.05)
		}
	}
	if opp.EdgePct.LessThan(minEdge) {
		return nil
	}

	if rule.MaxDailyTrades > 0 {
		now := time.Now().UTC()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		count, err := s.Repo.CountExecutionPlansByStrategySince(ctx, strategyName, dayStart)
		if err != nil {
			return err
		}
		if count >= int64(rule.MaxDailyTrades) {
			return nil
		}
	}

	plannedSize := opp.MaxSize
	maxLoss := plannedSize
	var kelly *float64
	if s.Risk != nil {
		ps, ml, kf, _ := s.Risk.SuggestPlanSizing(ctx, opp, strategyName)
		plannedSize = ps
		maxLoss = ml
		kelly = kf
	}
	if plannedSize.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	plan := &models.ExecutionPlan{
		OpportunityID:   opp.ID,
		Status:          "draft",
		StrategyName:    strategyName,
		PlannedSizeUSD:  plannedSize,
		MaxLossUSD:      maxLoss,
		KellyFraction:   kelly,
		Params:          datatypes.JSON([]byte(`{"slippage_tolerance":0.02,"execution_order":"sequential","limit_vs_market":"limit","time_limit_seconds":300}`)),
		PreflightResult: datatypes.JSON([]byte(`{}`)),
		Legs:            addAutoPlanLegSizing(opp.Legs, plannedSize),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := s.Repo.InsertExecutionPlan(ctx, plan); err != nil {
		return err
	}
	_ = s.Repo.UpdateOpportunityStatus(ctx, opp.ID, "executing")
	_ = s.Repo.UpsertPnLRecord(ctx, &models.PnLRecord{
		PlanID:       plan.ID,
		StrategyName: strategyName,
		ExpectedEdge: opp.EdgePct,
		Outcome:      "pending",
		CreatedAt:    time.Now().UTC(),
	})

	if s.Risk != nil {
		preflight, err := s.Risk.PreflightPlan(ctx, plan.ID)
		if err != nil {
			return err
		}
		if preflight == nil || !preflight.Passed {
			_ = s.Repo.UpdateOpportunityStatus(ctx, opp.ID, "failed")
			return nil
		}
	}

	_ = s.Repo.UpdateExecutionPlanStatus(ctx, plan.ID, "executing")
	if s.Config.DryRun {
		if err := s.insertDryRunFills(ctx, *plan); err != nil {
			return err
		}
		now := time.Now().UTC()
		_ = s.Repo.UpdateExecutionPlanExecutedAt(ctx, plan.ID, "executed", &now)
		_ = s.Repo.UpdateOpportunityStatus(ctx, opp.ID, "executed")
	} else if s.Logger != nil {
		s.Logger.Info("auto executor live mode placeholder: plan moved to executing", zap.Uint64("plan_id", plan.ID))
	}

	if s.Logger != nil {
		s.Logger.Info("auto executor processed opportunity",
			zap.Uint64("opportunity_id", opp.ID),
			zap.Uint64("plan_id", plan.ID),
			zap.String("strategy", strategyName),
			zap.Bool("dry_run", s.Config.DryRun),
		)
	}
	return nil
}

type autoPlanLeg struct {
	TokenID        string   `json:"token_id"`
	Direction      string   `json:"direction"`
	TargetPrice    *float64 `json:"target_price"`
	CurrentBestAsk *float64 `json:"current_best_ask"`
	SizeUSD        *float64 `json:"size_usd"`
}

func (s *AutoExecutorService) insertDryRunFills(ctx context.Context, plan models.ExecutionPlan) error {
	var legs []autoPlanLeg
	_ = json.Unmarshal(plan.Legs, &legs)
	if len(legs) == 0 {
		return nil
	}

	defaultSize := decimal.Zero
	if len(legs) > 0 {
		defaultSize = plan.PlannedSizeUSD.Div(decimal.NewFromInt(int64(len(legs))))
	}
	for _, leg := range legs {
		tokenID := strings.TrimSpace(leg.TokenID)
		if tokenID == "" {
			continue
		}
		price := decimal.NewFromFloat(0.5)
		if leg.TargetPrice != nil && *leg.TargetPrice > 0 {
			price = decimal.NewFromFloat(*leg.TargetPrice)
		} else if leg.CurrentBestAsk != nil && *leg.CurrentBestAsk > 0 {
			price = decimal.NewFromFloat(*leg.CurrentBestAsk)
		}
		if price.LessThan(decimal.NewFromFloat(0.01)) {
			price = decimal.NewFromFloat(0.01)
		}
		sizeUSD := defaultSize
		if leg.SizeUSD != nil && *leg.SizeUSD > 0 {
			sizeUSD = decimal.NewFromFloat(*leg.SizeUSD)
		}
		if sizeUSD.LessThanOrEqual(decimal.Zero) {
			continue
		}
		filledSize := sizeUSD.Div(price)
		dir := strings.ToUpper(strings.TrimSpace(leg.Direction))
		if dir == "" {
			dir = "BUY_YES"
		}
		item := &models.Fill{
			PlanID:     plan.ID,
			TokenID:    tokenID,
			Direction:  dir,
			FilledSize: filledSize,
			AvgPrice:   price,
			Fee:        decimal.Zero,
			FilledAt:   time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
		}
		if err := s.Repo.InsertFill(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func addAutoPlanLegSizing(legsJSON []byte, plannedSizeUSD decimal.Decimal) datatypes.JSON {
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

func boolPtrAuto(v bool) *bool { return &v }
