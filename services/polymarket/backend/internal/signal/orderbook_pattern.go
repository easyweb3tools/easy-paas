package signal

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/config"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// OrderbookPatternCollector derives higher-level pattern signals from market_data_health.
// MVP emits:
// - fear_spike: spread is wide and price jump is large
// - mm_inventory_skew: spread is wide but jump is small (a weak proxy for skew / adverse selection)
type OrderbookPatternCollector struct {
	Repo   repository.Repository
	Logger *zap.Logger

	Config config.OrderbookPatternConfig

	mu        sync.Mutex
	lastPoll  *time.Time
	lastError *string
	status    string
}

func (c *OrderbookPatternCollector) Name() string { return "orderbook_pattern" }

func (c *OrderbookPatternCollector) SourceInfo() SourceInfo {
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return SourceInfo{
		SourceType:   "rest_poll",
		Endpoint:     "db:market_data_health",
		PollInterval: interval,
	}
}

func (c *OrderbookPatternCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	limit := c.Config.Limit
	if limit <= 0 {
		limit = 100
	}
	minSpread := c.Config.MinSpreadBps
	if minSpread <= 0 {
		minSpread = 400
	}
	minJump := c.Config.MinJumpBps
	if minJump <= 0 {
		minJump = 600
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.pollOnce(ctx, out, limit, minSpread, minJump)
		}
	}
}

func (c *OrderbookPatternCollector) Stop() error { return nil }

func (c *OrderbookPatternCollector) Health() HealthStatus {
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

func (c *OrderbookPatternCollector) pollOnce(ctx context.Context, out chan<- models.Signal, limit int, minSpreadBps float64, minJumpBps float64) {
	now := time.Now().UTC()
	if c.Repo == nil {
		c.setHealth(now, "down", strPtr("repo unavailable"))
		return
	}
	rows, err := c.Repo.ListMarketDataHealthCandidates(ctx, limit, minSpreadBps)
	if err != nil {
		c.setHealth(now, "down", strPtr(err.Error()))
		return
	}
	if len(rows) == 0 {
		c.setHealth(now, "healthy", nil)
		return
	}

	tokenIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		if strings.TrimSpace(r.TokenID) != "" {
			tokenIDs = append(tokenIDs, strings.TrimSpace(r.TokenID))
		}
	}
	tokens, _ := c.Repo.ListTokensByIDs(ctx, tokenIDs)
	tokenByID := map[string]models.Token{}
	for _, t := range tokens {
		tokenByID[t.ID] = t
	}

	emitted := 0
	for _, r := range rows {
		tok := tokenByID[r.TokenID]
		if tok.ID == "" || tok.MarketID == "" {
			continue
		}
		// Only emit patterns for YES tokens for consistency with other strategies.
		if !strings.EqualFold(strings.TrimSpace(tok.Outcome), "yes") {
			continue
		}
		spreadBps := 0.0
		if r.SpreadBps != nil {
			spreadBps = *r.SpreadBps
		}
		jumpBps := 0.0
		if r.PriceJumpBps != nil {
			jumpBps = *r.PriceJumpBps
		}
		payload, _ := json.Marshal(map[string]any{
			"token_id":       tok.ID,
			"market_id":      tok.MarketID,
			"spread_bps":     spreadBps,
			"price_jump_bps": jumpBps,
			"updated_at":     r.UpdatedAt,
		})
		expires := now.Add(2 * time.Minute)

		switch {
		case jumpBps >= minJumpBps && spreadBps >= minSpreadBps:
			out <- models.Signal{
				SignalType: "fear_spike",
				Source:     "orderbook_pattern",
				MarketID:   strPtr(tok.MarketID),
				TokenID:    strPtr(tok.ID),
				Strength:   clamp01((jumpBps/1200.0 + spreadBps/1000.0) / 2.0),
				Direction:  "NEUTRAL",
				Payload:    datatypes.JSON(payload),
				ExpiresAt:  &expires,
				CreatedAt:  now,
			}
			emitted++
		case spreadBps >= minSpreadBps && jumpBps < minJumpBps/2.0:
			out <- models.Signal{
				SignalType: "mm_inventory_skew",
				Source:     "orderbook_pattern",
				MarketID:   strPtr(tok.MarketID),
				TokenID:    strPtr(tok.ID),
				Strength:   clamp01(spreadBps / 1000.0),
				Direction:  "NEUTRAL",
				Payload:    datatypes.JSON(payload),
				ExpiresAt:  &expires,
				CreatedAt:  now,
			}
			emitted++
		}
	}
	if emitted > 0 {
		c.setHealth(now, "healthy", nil)
		return
	}
	c.setHealth(now, "healthy", nil)
}

func (c *OrderbookPatternCollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}
