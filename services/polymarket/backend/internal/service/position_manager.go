package service

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type PositionManager struct {
	Repo   repository.Repository
	Logger *zap.Logger
	Flags  *SystemSettingsService
}

func (m *PositionManager) Run(ctx context.Context, interval time.Duration) error {
	if m == nil || m.Repo == nil {
		return nil
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		if err := m.RunOnce(ctx); err != nil && m.Logger != nil {
			m.Logger.Warn("position manager run failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (m *PositionManager) RunOnce(ctx context.Context) error {
	if m == nil || m.Repo == nil {
		return nil
	}
	if m.Flags != nil && !m.Flags.IsEnabled(ctx, FeaturePositionManager, false) {
		return nil
	}
	items, err := m.Repo.ListOpenPositions(ctx)
	if err != nil || len(items) == 0 {
		return err
	}

	eventIDs := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, p := range items {
		id := strings.TrimSpace(p.EventID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		eventIDs = append(eventIDs, id)
	}
	events, _ := m.Repo.ListEventsByIDs(ctx, eventIDs)
	eventByID := map[string]models.Event{}
	for _, e := range events {
		eventByID[e.ID] = e
	}

	now := time.Now().UTC()
	for _, p := range items {
		rule, _ := m.Repo.GetExecutionRuleByStrategyName(ctx, strings.TrimSpace(p.StrategyName))
		if rule == nil {
			continue
		}
		reason := ""
		if p.CostBasis.GreaterThan(decimal.Zero) {
			ratio := p.UnrealizedPnL.Div(p.CostBasis)
			if ratio.LessThan(rule.StopLossPct.Neg()) {
				reason = "stop_loss"
			}
			if reason == "" && ratio.GreaterThan(rule.TakeProfitPct) {
				reason = "take_profit"
			}
		}
		if reason == "" && rule.MaxHoldHours > 0 {
			if now.Sub(p.OpenedAt) > time.Duration(rule.MaxHoldHours)*time.Hour {
				reason = "max_hold_hours"
			}
		}
		if reason == "" && strings.TrimSpace(p.EventID) != "" {
			if ev, ok := eventByID[p.EventID]; ok && ev.EndTime != nil && !ev.EndTime.IsZero() {
				if ev.EndTime.UTC().Sub(now) <= time.Hour {
					reason = "market_expiry"
				}
			}
		}
		if reason == "" {
			continue
		}
		realized := p.RealizedPnL.Add(p.UnrealizedPnL)
		if err := m.Repo.ClosePosition(ctx, p.ID, realized, now); err != nil {
			return err
		}
		order := &models.Order{
			PlanID:        0,
			TokenID:       p.TokenID,
			Side:          closeSideByDirection(p.Direction),
			OrderType:     "market",
			Price:         p.CurrentPrice,
			SizeUSD:       p.CurrentPrice.Mul(p.Quantity),
			FilledUSD:     p.CurrentPrice.Mul(p.Quantity),
			Status:        "filled",
			FailureReason: "auto_close:" + reason,
			FilledAt:      &now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_ = m.Repo.InsertOrder(ctx, order)
		if m.Logger != nil {
			m.Logger.Info("position auto closed",
				zap.Uint64("position_id", p.ID),
				zap.String("token_id", p.TokenID),
				zap.String("reason", reason),
			)
		}
	}
	return nil
}

func closeSideByDirection(direction string) string {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "NO":
		return "SELL_NO"
	default:
		return "SELL_YES"
	}
}
