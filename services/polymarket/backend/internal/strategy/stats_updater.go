package strategy

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/repository"
)

// StatsUpdater periodically writes derived stats into strategies.stats so the UI can display them
// without expensive fanout queries.
type StatsUpdater struct {
	Repo     repository.Repository
	Logger   *zap.Logger
	Interval time.Duration
}

func (u *StatsUpdater) Run(ctx context.Context) error {
	if u == nil || u.Repo == nil {
		return nil
	}
	interval := u.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	// First run immediately so stats are available shortly after boot.
	_ = u.UpdateOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_ = u.UpdateOnce(ctx)
		}
	}
}

func (u *StatsUpdater) UpdateOnce(ctx context.Context) error {
	if u == nil || u.Repo == nil {
		return nil
	}
	now := time.Now().UTC()

	strategies, err := u.Repo.ListStrategies(ctx)
	if err != nil {
		u.logWarn("list strategies failed", err)
		return err
	}
	analytics, err := u.Repo.AnalyticsByStrategy(ctx)
	if err != nil {
		u.logWarn("analytics by strategy failed", err)
	}
	outcomes, err := u.Repo.AnalyticsStrategyOutcomes(ctx)
	if err != nil {
		u.logWarn("analytics outcomes by strategy failed", err)
	}

	anaByName := map[string]repository.StrategyAnalyticsRow{}
	for _, row := range analytics {
		name := strings.TrimSpace(row.StrategyName)
		if name != "" {
			anaByName[name] = row
		}
	}
	outByName := map[string]repository.StrategyOutcomeRow{}
	for _, row := range outcomes {
		name := strings.TrimSpace(row.StrategyName)
		if name != "" {
			outByName[name] = row
		}
	}

	active := "active"
	for _, s := range strategies {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		activeOpps, err := u.Repo.CountOpportunities(ctx, repository.ListOpportunitiesParams{
			Status:       &active,
			StrategyName: &name,
		})
		if err != nil {
			u.logWarn("count opportunities failed", err, zap.String("strategy", name))
		}
		ana := anaByName[name]
		out := outByName[name]

		winRate := 0.0
		decisions := out.WinCount + out.LossCount
		if decisions > 0 {
			winRate = float64(out.WinCount) / float64(decisions)
		}

		stats := map[string]any{
			"updated_at":           now.Format(time.RFC3339),
			"enabled":              s.Enabled,
			"priority":             s.Priority,
			"category":             s.Category,
			"active_opportunities": activeOpps,
			"plans":                ana.Plans,
			"total_pnl_usd":        ana.TotalPnLUSD,
			"avg_roi":              ana.AvgROI,
			"wins":                 out.WinCount,
			"losses":               out.LossCount,
			"partials":             out.PartialCount,
			"pending":              out.PendingCount,
			"win_rate":             winRate,
		}

		raw, _ := json.Marshal(stats)
		if err := u.Repo.UpdateStrategyStats(ctx, name, raw); err != nil {
			u.logWarn("update strategy stats failed", err, zap.String("strategy", name))
		}
	}
	return nil
}

func (u *StatsUpdater) logWarn(msg string, err error, fields ...zap.Field) {
	if u != nil && u.Logger != nil {
		u.Logger.Warn(msg, append(fields, zap.Error(err))...)
	}
}
