package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"nhooyr.io/websocket"

	"polymarket/internal/models"
)

// BinanceDepthCollector streams depth snapshots and emits "btc_depth_imbalance" signals.
// It is intentionally heuristic and rate-limited; it is meant to generate a directional signal, not a full orderbook model.
type BinanceDepthCollector struct {
	Logger *zap.Logger

	URL    string
	Symbol string

	mu        sync.Mutex
	cancel    context.CancelFunc
	conn      *websocket.Conn
	lastPoll  *time.Time
	lastError *string
	status    string
}

func (c *BinanceDepthCollector) Name() string { return "binance_ws" }

func (c *BinanceDepthCollector) SourceInfo() SourceInfo {
	return SourceInfo{
		SourceType:   "websocket",
		Endpoint:     c.URL,
		PollInterval: 0,
	}
}

func (c *BinanceDepthCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	if c == nil {
		return nil
	}
	url := strings.TrimSpace(c.URL)
	if url == "" {
		c.setHealth(time.Now().UTC(), "down", strPtr("missing url"))
		return fmt.Errorf("missing url")
	}
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		c.setHealth(time.Now().UTC(), "down", strPtr(err.Error()))
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "shutdown")
	}()

	lastEmit := time.Time{}
	for {
		_, msg, err := conn.Read(ctx)
		now := time.Now().UTC()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.setHealth(now, "down", strPtr(err.Error()))
			return err
		}
		c.setHealth(now, "healthy", nil)

		imb, ok := parseBinanceDepthImbalance(msg)
		if !ok {
			continue
		}

		// Rate-limit to avoid spamming SignalHub.
		if !lastEmit.IsZero() && now.Sub(lastEmit) < 2*time.Second {
			continue
		}
		lastEmit = now

		direction := "NEUTRAL"
		if imb.Ratio >= 1.25 {
			direction = "YES" // bullish
		} else if imb.Ratio <= 0.80 {
			direction = "NO" // bearish
		}

		payload := map[string]any{
			"symbol":         imb.Symbol,
			"bid_notional":   imb.BidNotional,
			"ask_notional":   imb.AskNotional,
			"ratio":          imb.Ratio,
			"levels":         imb.Levels,
			"raw_stream":     imb.Stream,
			"last_update_id": imb.LastUpdateID,
		}
		raw, _ := json.Marshal(payload)

		expires := now.Add(30 * time.Second)
		sig := models.Signal{
			SignalType: "btc_depth_imbalance",
			Source:     "binance_ws",
			Strength:   clamp01(abs(imb.Ratio-1.0) / 1.0),
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
}

func (c *BinanceDepthCollector) Stop() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	cancel := c.cancel
	conn := c.conn
	c.cancel = nil
	c.conn = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "stop")
	}
	return nil
}

func (c *BinanceDepthCollector) Health() HealthStatus {
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

func (c *BinanceDepthCollector) setHealth(ts time.Time, status string, errStr *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPoll = &ts
	c.status = status
	c.lastError = errStr
}

type binanceDepthSnapshot struct {
	LastUpdateId int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

type binanceStreamEnvelope struct {
	Stream string                `json:"stream"`
	Data   *binanceDepthSnapshot `json:"data"`
}

type imbalance struct {
	Symbol       string
	Stream       string
	LastUpdateID int64
	BidNotional  float64
	AskNotional  float64
	Ratio        float64
	Levels       int
}

func parseBinanceDepthImbalance(msg []byte) (imbalance, bool) {
	if len(msg) == 0 {
		return imbalance{}, false
	}
	// Try combined stream envelope first.
	var env binanceStreamEnvelope
	if err := json.Unmarshal(msg, &env); err == nil && env.Data != nil && (len(env.Data.Bids) > 0 || len(env.Data.Asks) > 0) {
		imb := computeImbalance(*env.Data)
		imb.Stream = env.Stream
		return imb, true
	}
	var snap binanceDepthSnapshot
	if err := json.Unmarshal(msg, &snap); err != nil {
		return imbalance{}, false
	}
	if len(snap.Bids) == 0 && len(snap.Asks) == 0 {
		return imbalance{}, false
	}
	return computeImbalance(snap), true
}

func computeImbalance(snap binanceDepthSnapshot) imbalance {
	bid := 0.0
	ask := 0.0
	levels := 0
	for _, row := range snap.Bids {
		if len(row) < 2 {
			continue
		}
		p, ok1 := atofSafe(row[0])
		q, ok2 := atofSafe(row[1])
		if !ok1 || !ok2 {
			continue
		}
		bid += p * q
		levels++
	}
	for _, row := range snap.Asks {
		if len(row) < 2 {
			continue
		}
		p, ok1 := atofSafe(row[0])
		q, ok2 := atofSafe(row[1])
		if !ok1 || !ok2 {
			continue
		}
		ask += p * q
		levels++
	}
	ratio := 1.0
	if ask > 0 {
		ratio = bid / ask
	}
	return imbalance{
		Symbol:       "",
		LastUpdateID: snap.LastUpdateId,
		BidNotional:  bid,
		AskNotional:  ask,
		Ratio:        ratio,
		Levels:       levels,
	}
}

func atofSafe(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	var sign float64 = 1
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	}
	n := 0.0
	frac := 0.0
	scale := 1.0
	seenDot := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '.' {
			if seenDot {
				return 0, false
			}
			seenDot = true
			continue
		}
		if ch < '0' || ch > '9' {
			return 0, false
		}
		d := float64(ch - '0')
		if !seenDot {
			n = n*10 + d
		} else {
			scale *= 10
			frac += d / scale
		}
	}
	return sign * (n + frac), true
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
