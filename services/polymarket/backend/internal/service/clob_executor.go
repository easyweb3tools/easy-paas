package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/repository"
	"polymarket/internal/risk"
)

type ExecutorConfig struct {
	Mode                 string
	MaxOrderSizeUSD      decimal.Decimal
	SlippageToleranceBps int
}

type SubmitResult struct {
	PlanID     uint64   `json:"plan_id"`
	OrderIDs   []uint64 `json:"order_ids"`
	Mode       string   `json:"mode"`
	PlanStatus string   `json:"plan_status"`
}

type CLOBExecutor struct {
	Repo         repository.Repository
	Risk         *risk.Manager
	Logger       *zap.Logger
	Config       ExecutorConfig
	PositionSync *PositionSyncService
}

type orderLeg struct {
	TokenID        string   `json:"token_id"`
	Direction      string   `json:"direction"`
	TargetPrice    *float64 `json:"target_price"`
	CurrentBestAsk *float64 `json:"current_best_ask"`
	SizeUSD        *float64 `json:"size_usd"`
}

func (e *CLOBExecutor) SubmitPlan(ctx context.Context, planID uint64) (*SubmitResult, error) {
	if e == nil || e.Repo == nil || planID == 0 {
		return nil, nil
	}
	plan, err := e.Repo.GetExecutionPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	if plan.Status != "preflight_pass" && plan.Status != "executing" {
		return nil, fmt.Errorf("plan status %s is not submittable", plan.Status)
	}
	if e.Risk != nil {
		res, err := e.Risk.PreflightPlan(ctx, planID)
		if err != nil {
			return nil, err
		}
		if res != nil && !res.Passed {
			return nil, fmt.Errorf("preflight failed")
		}
	}
	mode := e.resolveMode(ctx)
	legs, err := parseOrderLegs(plan.Legs)
	if err != nil {
		return nil, err
	}
	if len(legs) == 0 {
		return nil, fmt.Errorf("plan has no legs")
	}

	orderIDs := make([]uint64, 0, len(legs))
	perLeg := plan.PlannedSizeUSD.Div(decimal.NewFromInt(int64(len(legs))))
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
		sizeUSD := perLeg
		if leg.SizeUSD != nil && *leg.SizeUSD > 0 {
			sizeUSD = decimal.NewFromFloat(*leg.SizeUSD)
		}
		if e.Config.MaxOrderSizeUSD.GreaterThan(decimal.Zero) && sizeUSD.GreaterThan(e.Config.MaxOrderSizeUSD) {
			sizeUSD = e.Config.MaxOrderSizeUSD
		}
		order := &models.Order{
			PlanID:    plan.ID,
			TokenID:   tokenID,
			Side:      strings.ToUpper(strings.TrimSpace(leg.Direction)),
			OrderType: "limit",
			Price:     price,
			SizeUSD:   sizeUSD,
			FilledUSD: decimal.Zero,
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if order.Side == "" {
			order.Side = "BUY_YES"
		}
		if err := e.Repo.InsertOrder(ctx, order); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, order.ID)

		if mode == "dry-run" {
			now := time.Now().UTC()
			_ = e.Repo.UpdateOrderStatus(ctx, order.ID, "filled", map[string]any{
				"filled_usd": sizeUSD,
				"filled_at":  &now,
			})
			fillSize := decimal.Zero
			if price.GreaterThan(decimal.Zero) {
				fillSize = sizeUSD.Div(price)
			}
			fill := &models.Fill{
				PlanID:     plan.ID,
				TokenID:    tokenID,
				Direction:  order.Side,
				FilledSize: fillSize,
				AvgPrice:   price,
				Fee:        decimal.Zero,
				FilledAt:   now,
				CreatedAt:  now,
			}
			_ = e.Repo.InsertFill(ctx, fill)
			if e.PositionSync != nil {
				_ = e.PositionSync.SyncFromFill(ctx, *fill)
			}
		} else {
			now := time.Now().UTC()
			_ = e.Repo.UpdateOrderStatus(ctx, order.ID, "submitted", map[string]any{
				"submitted_at": &now,
			})
			_ = e.Repo.UpdateOrderStatus(ctx, order.ID, "failed", map[string]any{
				"failure_reason": "live order placement not integrated yet",
			})
		}
	}

	if mode == "dry-run" {
		now := time.Now().UTC()
		_ = e.Repo.UpdateExecutionPlanExecutedAt(ctx, plan.ID, "executed", &now)
		_ = e.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executed")
	} else {
		_ = e.Repo.UpdateExecutionPlanStatus(ctx, plan.ID, "executing")
		_ = e.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executing")
	}

	return &SubmitResult{
		PlanID:     plan.ID,
		OrderIDs:   orderIDs,
		Mode:       mode,
		PlanStatus: map[bool]string{true: "executed", false: "executing"}[mode == "dry-run"],
	}, nil
}

func (e *CLOBExecutor) PollOrders(ctx context.Context) error {
	if e == nil || e.Repo == nil {
		return nil
	}
	mode := e.resolveMode(ctx)
	if mode == "live" {
		if e.Logger != nil {
			e.Logger.Warn("live mode poll requested but exchange sync is not integrated")
		}
	}
	return nil
}

func (e *CLOBExecutor) CancelOrder(ctx context.Context, orderID uint64) error {
	if e == nil || e.Repo == nil || orderID == 0 {
		return nil
	}
	order, err := e.Repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}
	switch order.Status {
	case "submitted", "partial", "pending":
		now := time.Now().UTC()
		return e.Repo.UpdateOrderStatus(ctx, orderID, "cancelled", map[string]any{
			"cancelled_at": &now,
		})
	default:
		return nil
	}
}

func parseOrderLegs(raw []byte) ([]orderLeg, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []orderLeg
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *CLOBExecutor) resolveMode(ctx context.Context) string {
	mode := strings.ToLower(strings.TrimSpace(e.Config.Mode))
	if e != nil && e.Repo != nil {
		if row, err := e.Repo.GetSystemSettingByKey(ctx, "trading.executor_mode"); err == nil && row != nil && len(row.Value) > 0 {
			var v string
			if err := json.Unmarshal(row.Value, &v); err == nil {
				v = strings.ToLower(strings.TrimSpace(v))
				if v == "dry-run" || v == "live" {
					mode = v
				}
			}
		}
	}
	if mode == "" {
		return "dry-run"
	}
	return mode
}
