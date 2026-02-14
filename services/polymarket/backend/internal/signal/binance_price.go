package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/models"
)

// BinancePriceCollector polls Binance REST price endpoint and emits "btc_price_change" signals.
//
// This collector is designed to be "no key" and minimal:
// - endpoint default: https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT
// - it computes percent change over a sliding window and emits a signal when abs(change) >= triggerPct.
type BinancePriceCollector struct {
	HTTP   *http.Client
	Logger *zap.Logger

	Endpoint      string
	PollInterval  time.Duration
	WindowSeconds int
	TriggerPct    float64

	mu        sync.Mutex
	lastPoll  *time.Time
	lastError *string
	status    string

	series []pricePoint
}

type pricePoint struct {
	ts    time.Time
	price float64
}

func (c *BinancePriceCollector) Name() string { return "binance_price" }

func (c *BinancePriceCollector) SourceInfo() SourceInfo {
	interval := c.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return SourceInfo{
		SourceType:   "rest_poll",
		Endpoint:     strings.TrimSpace(c.Endpoint),
		PollInterval: interval,
	}
}

func (c *BinancePriceCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	interval := c.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	// Run immediately once.
	c.pollOnce(ctx, out)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			c.pollOnce(ctx, out)
		}
	}
}

func (c *BinancePriceCollector) Stop() error { return nil }

func (c *BinancePriceCollector) Health() HealthStatus {
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

func (c *BinancePriceCollector) pollOnce(ctx context.Context, out chan<- models.Signal) {
	now := time.Now().UTC()
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint == "" {
		c.setHealth(now, "down", strPtr("missing endpoint"))
		return
	}
	window := c.WindowSeconds
	if window <= 0 {
		window = 300
	}
	trigger := c.TriggerPct
	if trigger <= 0 {
		trigger = 2.0
	}

	price, err := c.fetchPrice(ctx, endpoint)
	if err != nil {
		c.setHealth(now, "down", strPtr(err.Error()))
		return
	}
	c.setHealth(now, "healthy", nil)

	c.mu.Lock()
	c.series = append(c.series, pricePoint{ts: now, price: price})
	// Drop points outside the window (keep one extra for baseline search).
	cut := now.Add(-time.Duration(window) * time.Second)
	j := 0
	for ; j < len(c.series); j++ {
		if c.series[j].ts.After(cut) {
			break
		}
	}
	if j > 0 && j < len(c.series) {
		c.series = c.series[j:]
	} else if j >= len(c.series) {
		c.series = c.series[:0]
		c.series = append(c.series, pricePoint{ts: now, price: price})
	}
	// Baseline: earliest point in window.
	var base *pricePoint
	if len(c.series) > 0 {
		base = &c.series[0]
	}
	c.mu.Unlock()

	if base == nil || base.price <= 0 {
		return
	}
	pct := (price - base.price) / base.price * 100.0
	if abs(pct) < trigger {
		return
	}

	direction := "NEUTRAL"
	if pct >= 0 {
		direction = "YES" // BTC up
	} else {
		direction = "NO" // BTC down
	}
	strength := clamp01(abs(pct) / trigger)
	payload := map[string]any{
		"endpoint":         endpoint,
		"price":            price,
		"base_price":       base.price,
		"window_seconds":   window,
		"change_pct":       pct,
		"trigger_pct":      trigger,
		"base_timestamp":   base.ts.Format(time.RFC3339Nano),
		"sample_timestamp": now.Format(time.RFC3339Nano),
	}
	raw, _ := json.Marshal(payload)

	expires := now.Add(time.Duration(window) * time.Second)
	sig := models.Signal{
		SignalType: "btc_price_change",
		Source:     "binance_price",
		Strength:   strength,
		Direction:  direction,
		Payload:    raw,
		ExpiresAt:  &expires,
		CreatedAt:  now,
	}
	select {
	case out <- sig:
	default:
	}
}

func (c *BinancePriceCollector) fetchPrice(ctx context.Context, endpoint string) (float64, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("http %d", resp.StatusCode)
	}
	var parsed struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	p, ok := atofSafe(parsed.Price)
	if !ok || p <= 0 {
		return 0, fmt.Errorf("invalid price")
	}
	return p, nil
}

func (c *BinancePriceCollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}
