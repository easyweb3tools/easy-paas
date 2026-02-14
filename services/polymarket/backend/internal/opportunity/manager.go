package opportunity

import (
	"context"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/paas"
	"polymarket/internal/repository"
)

type Manager struct {
	Repo   repository.Repository
	Logger *zap.Logger

	MaxActive int
}

func (m *Manager) Upsert(ctx context.Context, opp *models.Opportunity) error {
	if m == nil || m.Repo == nil || opp == nil {
		return nil
	}
	if err := m.Repo.UpsertActiveOpportunity(ctx, opp); err != nil {
		return err
	}
	paas.LogBestEffortCtx(ctx, "polymarket_opportunity_upserted", "info", map[string]any{
		"strategy_id": opp.StrategyID,
		"status":      opp.Status,
	})
	_, _ = m.Repo.ExpireDueOpportunities(ctx, time.Now().UTC())
	m.enforceMax(ctx)
	return nil
}

func (m *Manager) enforceMax(ctx context.Context) {
	if m == nil || m.Repo == nil || m.MaxActive <= 0 {
		return
	}
	total, err := m.Repo.CountActiveOpportunities(ctx)
	if err != nil {
		return
	}
	excess := int(total) - m.MaxActive
	if excess <= 0 {
		return
	}
	ids, err := m.Repo.ListOldestActiveOpportunityIDs(ctx, excess)
	if err != nil {
		return
	}
	if len(ids) == 0 {
		return
	}
	if _, err := m.Repo.BulkUpdateOpportunityStatus(ctx, ids, "expired"); err != nil {
		return
	}
	paas.LogBestEffortCtx(ctx, "polymarket_opportunities_expired", "info", map[string]any{
		"expired":    len(ids),
		"max_active": m.MaxActive,
	})
	if m.Logger != nil {
		m.Logger.Info("expired old opportunities to enforce max", zap.Int("expired", len(ids)), zap.Int("max_active", m.MaxActive))
	}
}
