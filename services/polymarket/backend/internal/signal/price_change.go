package signal

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/config"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// PriceChangeCollector scans market_data_health for large price jumps and emits "news_alpha" signals.
// architecture-v2: it conceptually comes from CLOB WS; here we use the derived DB health rows as a stable source.
type PriceChangeCollector struct {
	Repo   repository.Repository
	Logger *zap.Logger

	Config config.PriceChangeConfig

	mu        sync.Mutex
	lastPoll  *time.Time
	lastError *string
	status    string
}

func (c *PriceChangeCollector) Name() string { return "price_change" }

func (c *PriceChangeCollector) SourceInfo() SourceInfo {
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return SourceInfo{
		SourceType:   "rest_poll",
		Endpoint:     "db:market_data_health",
		PollInterval: interval,
	}
}

func (c *PriceChangeCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	limit := c.Config.Limit
	if limit <= 0 {
		limit = 50
	}
	minJump := c.Config.MinJumpBps
	if minJump <= 0 {
		minJump = 500
	}
	maxSpread := c.Config.MaxSpreadBps
	if maxSpread <= 0 {
		maxSpread = 400
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.pollOnce(ctx, out, limit, minJump, maxSpread)
		}
	}
}

func (c *PriceChangeCollector) Stop() error { return nil }

func (c *PriceChangeCollector) Health() HealthStatus {
	if c == nil {
		return HealthStatus{Status: "unknown"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	status := c.status
	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}
	return HealthStatus{
		Status:     status,
		LastPollAt: c.lastPoll,
		LastError:  c.lastError,
	}
}

func (c *PriceChangeCollector) pollOnce(ctx context.Context, out chan<- models.Signal, limit int, minJumpBps float64, maxSpreadBps float64) {
	now := time.Now().UTC()
	if c.Repo == nil {
		c.setHealth(now, "down", strPtr("repo unavailable"))
		return
	}
	rows, err := c.Repo.ListYesTokenJumpCandidates(ctx, limit, minJumpBps, maxSpreadBps)
	if err != nil {
		c.setHealth(now, "down", strPtr(err.Error()))
		return
	}
	if len(rows) == 0 {
		c.setHealth(now, "healthy", nil)
		return
	}
	for _, row := range rows {
		if strings.TrimSpace(row.TokenID) == "" || strings.TrimSpace(row.MarketID) == "" {
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"token_id":       row.TokenID,
			"market_id":      row.MarketID,
			"price_jump_bps": row.PriceJumpBps,
			"spread_bps":     row.SpreadBps,
			"updated_at":     row.UpdatedAt,
		})
		expires := now.Add(2 * time.Minute)
		// Both strategies (news_alpha / volatility_arb) can consume the same underlying "jump" signal.
		base := models.Signal{
			Source:    "price_change",
			MarketID:  strPtr(row.MarketID),
			TokenID:   strPtr(row.TokenID),
			Strength:  clamp01(row.PriceJumpBps / 1200.0),
			Direction: "NEUTRAL",
			Payload:   payload,
			ExpiresAt: &expires,
			CreatedAt: now,
		}
		s1 := base
		s1.SignalType = "news_alpha"
		out <- s1
		s2 := base
		s2.SignalType = "volatility_spread"
		out <- s2
	}
	c.setHealth(now, "healthy", nil)
}

func (c *PriceChangeCollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}
