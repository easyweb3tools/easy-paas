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

// CertaintySweepCollector scans events ending soon and emits "certainty_sweep" for markets that are already priced near certainty.
// This is an MVP proxy for "late-stage sweep" ideas; it does not integrate external score feeds.
type CertaintySweepCollector struct {
	Repo   repository.Repository
	Logger *zap.Logger

	Config config.CertaintySweepConfig

	mu        sync.Mutex
	lastPoll  *time.Time
	lastError *string
	status    string
}

func (c *CertaintySweepCollector) Name() string { return "certainty_sweep" }

func (c *CertaintySweepCollector) SourceInfo() SourceInfo {
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return SourceInfo{SourceType: "internal_scan", Endpoint: "db", PollInterval: interval}
}

func (c *CertaintySweepCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	interval := c.Config.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	hours := c.Config.HoursToExpiry
	if hours <= 0 {
		hours = 6
	}
	limit := c.Config.Limit
	if limit <= 0 {
		limit = 50
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.pollOnce(ctx, out, hours, limit)
		}
	}
}

func (c *CertaintySweepCollector) Stop() error { return nil }

func (c *CertaintySweepCollector) Health() HealthStatus {
	if c == nil {
		return HealthStatus{Status: "unknown"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	status := c.status
	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}
	return HealthStatus{Status: status, LastPollAt: c.lastPoll, LastError: c.lastError}
}

func (c *CertaintySweepCollector) pollOnce(ctx context.Context, out chan<- models.Signal, hoursToExpiry int, limit int) {
	now := time.Now().UTC()
	if c.Repo == nil {
		c.setHealth(now, "down", strPtr("repo unavailable"))
		return
	}
	events, err := c.Repo.ListActiveEventsEndingSoon(ctx, hoursToExpiry, limit)
	if err != nil {
		c.setHealth(now, "down", strPtr(err.Error()))
		return
	}
	emitted := 0
	for _, ev := range events {
		if strings.TrimSpace(ev.ID) == "" {
			continue
		}
		markets, err := c.Repo.ListMarketsByEventID(ctx, ev.ID)
		if err != nil || len(markets) == 0 {
			continue
		}
		marketIDs := make([]string, 0, len(markets))
		for _, m := range markets {
			if strings.TrimSpace(m.ID) != "" {
				marketIDs = append(marketIDs, strings.TrimSpace(m.ID))
			}
		}
		toks, err := c.Repo.ListTokensByMarketIDs(ctx, marketIDs)
		if err != nil || len(toks) == 0 {
			continue
		}
		yesByMarket := map[string]string{}
		for _, t := range toks {
			if strings.EqualFold(strings.TrimSpace(t.Outcome), "yes") {
				yesByMarket[t.MarketID] = t.ID
			}
		}
		yesIDs := make([]string, 0, len(yesByMarket))
		for _, mid := range marketIDs {
			if id := yesByMarket[mid]; id != "" {
				yesIDs = append(yesIDs, id)
			}
		}
		books, _ := c.Repo.ListOrderbookLatestByTokenIDs(ctx, yesIDs)
		bookByToken := map[string]models.OrderbookLatest{}
		for _, b := range books {
			bookByToken[b.TokenID] = b
		}
		for mid, tid := range yesByMarket {
			book := bookByToken[tid]
			if book.TokenID == "" || book.BestAsk == nil {
				continue
			}
			ask := *book.BestAsk
			if ask >= 0.97 || ask <= 0.03 {
				payload, _ := json.Marshal(map[string]any{
					"event_id":     ev.ID,
					"market_id":    mid,
					"token_id":     tid,
					"yes_best_ask": ask,
				})
				expires := now.Add(15 * time.Minute)
				out <- models.Signal{
					SignalType: "certainty_sweep",
					Source:     "certainty_sweep",
					EventID:    strPtr(ev.ID),
					MarketID:   strPtr(mid),
					TokenID:    strPtr(tid),
					Strength:   clamp01(abs(ask-0.5) / 0.5),
					Direction:  "NEUTRAL",
					Payload:    datatypes.JSON(payload),
					ExpiresAt:  &expires,
					CreatedAt:  now,
				}
				emitted++
			}
		}
	}
	c.setHealth(now, "healthy", nil)
	_ = emitted
}

func (c *CertaintySweepCollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}
