package signal

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// SignalHub runs collectors, persists signals, and fans out to subscribers by type.
type SignalHub struct {
	collectors map[string]SignalCollector
	subs       map[string][]chan models.Signal
	mu         sync.RWMutex

	repo   repository.Repository
	logger *zap.Logger

	dedupMu       sync.Mutex
	lastSeen      map[string]time.Time
	droppedDedup  uint64
	droppedFanout uint64
}

func NewHub(repo repository.Repository, logger *zap.Logger) *SignalHub {
	return &SignalHub{
		collectors: map[string]SignalCollector{},
		subs:       map[string][]chan models.Signal{},
		repo:       repo,
		logger:     logger,
		lastSeen:   map[string]time.Time{},
	}
}

func (h *SignalHub) Register(c SignalCollector) {
	if h == nil || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.collectors[c.Name()] = c
}

// Subscribe returns a channel that receives persisted signals for a given type.
func (h *SignalHub) Subscribe(signalType string, buf int) <-chan models.Signal {
	if buf <= 0 {
		buf = 16
	}
	ch := make(chan models.Signal, buf)
	h.mu.Lock()
	h.subs[signalType] = append(h.subs[signalType], ch)
	h.mu.Unlock()
	return ch
}

func (h *SignalHub) Run(ctx context.Context) error {
	if h == nil {
		return nil
	}
	out := make(chan models.Signal, 128)

	h.mu.RLock()
	collectors := make([]SignalCollector, 0, len(h.collectors))
	for _, c := range h.collectors {
		collectors = append(collectors, c)
	}
	h.mu.RUnlock()

	for _, c := range collectors {
		c := c
		h.upsertSource(ctx, c, HealthStatus{Status: "unknown"})
		go func() {
			if err := c.Start(ctx, out); err != nil && h.logger != nil {
				h.logger.Warn("signal collector stopped", zap.String("collector", c.Name()), zap.Error(err))
			}
		}()
	}

	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()
	statsTicker := time.NewTicker(60 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			for _, c := range collectors {
				_ = c.Stop()
			}
			return ctx.Err()
		case <-healthTicker.C:
			for _, c := range collectors {
				h.upsertSource(ctx, c, c.Health())
			}
		case <-statsTicker.C:
			if h.logger != nil {
				h.logger.Info("signal hub stats",
					zap.Uint64("dropped_dedup", atomic.LoadUint64(&h.droppedDedup)),
					zap.Uint64("dropped_fanout", atomic.LoadUint64(&h.droppedFanout)),
				)
			}
		case sig := <-out:
			sig = h.normalize(sig)
			if h.shouldDrop(sig) {
				atomic.AddUint64(&h.droppedDedup, 1)
				continue
			}
			if h.repo != nil {
				_ = h.repo.InsertSignal(ctx, &sig)
			}
			h.fanout(sig)
		}
	}
}

func (h *SignalHub) fanout(sig models.Signal) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs[sig.SignalType] {
		select {
		case ch <- sig:
		default:
			// Drop when subscriber is slow; hub must not block.
			atomic.AddUint64(&h.droppedFanout, 1)
		}
	}
}

func (h *SignalHub) normalize(sig models.Signal) models.Signal {
	now := time.Now().UTC()
	if sig.CreatedAt.IsZero() {
		sig.CreatedAt = now
	}
	if sig.ExpiresAt == nil {
		ttl := defaultSignalTTL(sig.SignalType)
		if ttl > 0 {
			t := sig.CreatedAt.Add(ttl)
			sig.ExpiresAt = &t
		}
	}
	return sig
}

func (h *SignalHub) shouldDrop(sig models.Signal) bool {
	window := defaultDedupWindow(sig.SignalType)
	if window <= 0 {
		return false
	}
	key := dedupKey(sig)
	if key == "" {
		return false
	}
	h.dedupMu.Lock()
	defer h.dedupMu.Unlock()
	if last, ok := h.lastSeen[key]; ok && sig.CreatedAt.Sub(last) < window {
		return true
	}
	h.lastSeen[key] = sig.CreatedAt
	return false
}

func dedupKey(sig models.Signal) string {
	// Keep this stable and coarse; it is meant to suppress spam, not provide perfect idempotency.
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		sig.Source,
		sig.SignalType,
		strVal(sig.EventID),
		strVal(sig.MarketID),
		strVal(sig.TokenID),
		sig.Direction,
	)
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func defaultDedupWindow(signalType string) time.Duration {
	switch signalType {
	case "arb_sum_deviation":
		return 30 * time.Second
	case "no_bias":
		return 2 * time.Minute
	case "liquidity_gap":
		return 2 * time.Minute
	default:
		return 30 * time.Second
	}
}

func defaultSignalTTL(signalType string) time.Duration {
	switch signalType {
	case "arb_sum_deviation":
		return 2 * time.Minute
	case "no_bias":
		return 2 * time.Hour
	case "liquidity_gap":
		return 10 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func (h *SignalHub) upsertSource(ctx context.Context, c SignalCollector, health HealthStatus) {
	if h == nil || h.repo == nil || c == nil {
		return
	}
	info := SourceInfo{SourceType: "internal", PollInterval: 0}
	if p, ok := c.(SignalSourceInfo); ok {
		info = p.SourceInfo()
	}
	hs := health.Status
	if hs == "" {
		hs = "unknown"
	}
	now := time.Now().UTC()
	lastPoll := health.LastPollAt
	if lastPoll == nil {
		lastPoll = &now
	}
	item := &models.SignalSource{
		Name:         c.Name(),
		SourceType:   info.SourceType,
		Endpoint:     info.Endpoint,
		PollInterval: durationString(info.PollInterval),
		Enabled:      true,
		LastPollAt:   lastPoll,
		LastError:    health.LastError,
		HealthStatus: hs,
	}
	_ = h.repo.UpsertSignalSource(ctx, item)
}

func durationString(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}
