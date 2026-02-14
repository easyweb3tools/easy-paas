package signal

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// SettlementHistoryCollector aggregates `market_settlement_history` into learned NO rates by label.
// It writes the aggregates into `strategies.stats` for `systematic_no` and also emits an L4 signal
// for observability.
type SettlementHistoryCollector struct {
	Repo   repository.Repository
	Logger *zap.Logger

	Interval   time.Duration
	MinSamples int64

	mu      sync.Mutex
	lastRun *time.Time
	lastErr *string
	stopCh  chan struct{}
}

func (c *SettlementHistoryCollector) Name() string { return "settlement_history" }

func (c *SettlementHistoryCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil || c.Repo == nil {
		return nil
	}
	interval := c.Interval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	minSamples := c.MinSamples
	if minSamples <= 0 {
		minSamples = 10
	}
	c.MinSamples = minSamples

	// Run immediately on start.
	c.runOnce(ctx, out, time.Now().UTC())

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.runOnce(ctx, out, time.Now().UTC())
		}
	}
}

func (c *SettlementHistoryCollector) runOnce(ctx context.Context, out chan<- models.Signal, now time.Time) {
	rows, err := c.Repo.ListLabelNoRateStats(ctx, nil)
	if err != nil {
		c.setRun(now, err)
		c.logWarn("settlement history stats scan failed", err)
		return
	}
	filtered := make([]repository.LabelNoRateRow, 0, len(rows))
	rates := map[string]float64{}
	for _, r := range rows {
		label := strings.TrimSpace(r.Label)
		if label == "" {
			continue
		}
		if r.Total < c.MinSamples {
			continue
		}
		filtered = append(filtered, r)
		rates[label] = r.NoRate
	}

	// Persist into systematic_no strategy stats (merge to avoid clobbering other stats).
	strat, err := c.Repo.GetStrategyByName(ctx, "systematic_no")
	if err != nil {
		c.setRun(now, err)
		c.logWarn("get systematic_no strategy failed", err)
		return
	}
	if strat != nil {
		stats := map[string]any{}
		if len(strat.Stats) > 0 {
			_ = json.Unmarshal(strat.Stats, &stats)
		}
		stats["learned_no_rate_min_samples"] = c.MinSamples
		stats["learned_no_rates_by_label"] = filtered
		stats["learned_no_rates_updated_at"] = now.Format(time.RFC3339)
		// Also publish a simple map for consumers.
		stats["category_no_rates"] = rates

		raw, _ := json.Marshal(stats)
		if err := c.Repo.UpdateStrategyStats(ctx, "systematic_no", raw); err != nil {
			c.setRun(now, err)
			c.logWarn("update systematic_no stats failed", err)
			return
		}
	}

	// Emit an L4 signal for observability (even if no rows).
	payload, _ := json.Marshal(map[string]any{
		"min_samples": c.MinSamples,
		"rows":        filtered,
		"updated_at":  now.Format(time.RFC3339),
	})
	expires := now.Add(6 * time.Hour)
	out <- models.Signal{
		SignalType: "settlement_no_rates",
		Source:     "settlement_history",
		Strength:   1.0,
		Direction:  "NEUTRAL",
		Payload:    datatypes.JSON(payload),
		ExpiresAt:  &expires,
		CreatedAt:  now,
	}
	c.setRun(now, nil)
}

func (c *SettlementHistoryCollector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}
	return nil
}

func (c *SettlementHistoryCollector) Health() HealthStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return HealthStatus{
		Status:     "healthy",
		LastPollAt: c.lastRun,
		LastError:  c.lastErr,
		Details: map[string]any{
			"collector":   "settlement_history",
			"min_samples": c.MinSamples,
		},
	}
}

func (c *SettlementHistoryCollector) SourceInfo() SourceInfo {
	return SourceInfo{
		SourceType:   "db_aggregate",
		Endpoint:     "market_settlement_history",
		PollInterval: c.Interval,
	}
}

func (c *SettlementHistoryCollector) setRun(now time.Time, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRun = &now
	if err == nil {
		c.lastErr = nil
		return
	}
	msg := err.Error()
	c.lastErr = &msg
}

func (c *SettlementHistoryCollector) logWarn(msg string, err error, fields ...zap.Field) {
	if c == nil || c.Logger == nil {
		return
	}
	c.Logger.Warn(msg, append(fields, zap.Error(err))...)
}
