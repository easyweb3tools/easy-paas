package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type Engine struct {
	Repo   repository.Repository
	Hub    SignalSubscriber
	Logger *zap.Logger

	Evaluators []StrategyEvaluator
	Risk       interface {
		Filter([]models.Opportunity) []models.Opportunity
	}
	Opps interface {
		Upsert(context.Context, *models.Opportunity) error
	}

	// StrategyDefaults is the config-sourced default override map (config.strategy_defaults).
	// Shape: { "arb_sum": { "enabled": true, ... }, ... }
	StrategyDefaults map[string]any

	enabledMu     sync.RWMutex
	enabledByName map[string]bool

	paramsMu     sync.RWMutex
	paramsByName map[string]datatypes.JSON

	evByName map[string]StrategyEvaluator
}

func (e *Engine) Run(ctx context.Context) error {
	if e == nil || e.Repo == nil || e.Hub == nil {
		return nil
	}
	if len(e.Evaluators) == 0 {
		return nil
	}
	if e.enabledByName == nil {
		e.enabledByName = map[string]bool{}
	}
	if e.paramsByName == nil {
		e.paramsByName = map[string]datatypes.JSON{}
	}
	if e.evByName == nil {
		e.evByName = map[string]StrategyEvaluator{}
		for _, ev := range e.Evaluators {
			if ev != nil && ev.Name() != "" {
				e.evByName[ev.Name()] = ev
			}
		}
	}
	// Initial load before workers start consuming.
	e.reloadStrategies(ctx)
	go e.reloadEnabledLoop(ctx)
	for _, ev := range e.Evaluators {
		ev := ev
		if ev == nil {
			continue
		}
		if err := e.ensureStrategyRow(ctx, ev); err != nil && e.Logger != nil {
			e.Logger.Warn("ensure strategy row failed", zap.String("strategy", ev.Name()), zap.Error(err))
		}
		for _, sigType := range ev.RequiredSignals() {
			ch := e.Hub.Subscribe(sigType, 64)
			go e.runWorker(ctx, ev, sigType, ch)
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func (e *Engine) runWorker(ctx context.Context, ev StrategyEvaluator, sigType string, ch <-chan models.Signal) {
	if e.Logger != nil {
		e.Logger.Info("strategy worker started", zap.String("strategy", ev.Name()), zap.String("signal_type", sigType))
	}
	// Simple backoff on evaluator failure.
	backoff := 200 * time.Millisecond
	const (
		batchWindow = 300 * time.Millisecond
		batchMax    = 32
	)
	var (
		timer   *time.Timer
		timerCh <-chan time.Time
		batch   []models.Signal
	)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if !e.isEnabled(ev.Name()) {
			batch = batch[:0]
			return
		}
		strat, _ := e.Repo.GetStrategyByName(ctx, ev.Name())
		if strat == nil {
			batch = batch[:0]
			return
		}
		opps, err := ev.Evaluate(ctx, batch)
		batch = batch[:0]
		if err != nil {
			if e.Logger != nil && !errors.Is(err, context.Canceled) {
				e.Logger.Warn("strategy evaluate failed", zap.String("strategy", ev.Name()), zap.Error(err))
			}
			time.Sleep(backoff)
			if backoff < 5*time.Second {
				backoff *= 2
			}
			return
		}
		backoff = 200 * time.Millisecond
		if len(opps) == 0 {
			return
		}
		// Assign strategy before risk so risk can apply per-strategy gating.
		for i := range opps {
			opps[i].StrategyID = strat.ID
		}
		if e.Risk != nil {
			opps = e.Risk.Filter(opps)
		}
		if len(opps) == 0 {
			return
		}
		for i := range opps {
			if e.Opps != nil {
				_ = e.Opps.Upsert(ctx, &opps[i])
			} else {
				_ = e.Repo.UpsertActiveOpportunity(ctx, &opps[i])
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-ch:
			batch = append(batch, sig)
			if len(batch) == 1 {
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(batchWindow)
				timerCh = timer.C
			}
			if len(batch) >= batchMax {
				if timer != nil {
					timer.Stop()
				}
				timerCh = nil
				flush()
			}
		case <-timerCh:
			timerCh = nil
			flush()
		}
	}
}

func (e *Engine) reloadEnabledLoop(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.reloadStrategies(ctx)
		}
	}
}

func (e *Engine) reloadStrategies(ctx context.Context) {
	if e == nil || e.Repo == nil {
		return
	}
	items, err := e.Repo.ListStrategies(ctx)
	if err != nil {
		return
	}
	nextEnabled := map[string]bool{}
	nextParams := map[string]datatypes.JSON{}
	for _, it := range items {
		if strings.TrimSpace(it.Name) == "" {
			continue
		}
		nextEnabled[it.Name] = it.Enabled
		ev := e.evByName[it.Name]
		merged := mergeParams(ev, e.StrategyDefaults, it.Name, it.Params)
		nextParams[it.Name] = merged
		if p, ok := ev.(interface{ SetParams(json.RawMessage) error }); ok && len(merged) > 0 {
			_ = p.SetParams(json.RawMessage(merged))
		}
	}
	e.enabledMu.Lock()
	e.enabledByName = nextEnabled
	e.enabledMu.Unlock()
	e.paramsMu.Lock()
	e.paramsByName = nextParams
	e.paramsMu.Unlock()
}

func (e *Engine) isEnabled(name string) bool {
	if e == nil {
		return false
	}
	e.enabledMu.RLock()
	val, ok := e.enabledByName[name]
	e.enabledMu.RUnlock()
	if ok {
		return val
	}
	// If missing in cache, be conservative.
	return false
}

func mergeParams(ev StrategyEvaluator, defaults map[string]any, name string, db datatypes.JSON) datatypes.JSON {
	base := map[string]any{}
	// Start from evaluator defaults.
	if ev != nil {
		_ = json.Unmarshal(ev.DefaultParams(), &base)
	}
	// Apply config defaults for this strategy (excluding "enabled").
	if raw, ok := defaults[name]; ok {
		if m, ok := raw.(map[string]any); ok {
			for k, v := range m {
				if strings.EqualFold(k, "enabled") {
					continue
				}
				base[k] = v
			}
		}
	}
	// DB overrides.
	if len(db) > 0 {
		override := map[string]any{}
		if err := json.Unmarshal(db, &override); err == nil {
			for k, v := range override {
				base[k] = v
			}
		}
	}
	raw, err := json.Marshal(base)
	if err != nil {
		if len(db) > 0 {
			return db
		}
		if ev != nil {
			return datatypes.JSON(ev.DefaultParams())
		}
		return datatypes.JSON([]byte(`{}`))
	}
	return datatypes.JSON(raw)
}

func (e *Engine) ensureStrategyRow(ctx context.Context, ev StrategyEvaluator) error {
	if e == nil || e.Repo == nil || ev == nil {
		return nil
	}
	existing, _ := e.Repo.GetStrategyByName(ctx, ev.Name())
	req, _ := json.Marshal(ev.RequiredSignals())

	category := "arbitrage"
	priority := 0
	switch ev.Name() {
	case "systematic_no":
		category = "systematic"
		priority = 0
	case "pre_market_fdv":
		category = "systematic"
		priority = 0
	case "news_alpha":
		category = "speed"
		priority = 1
	case "weather":
		category = "data_driven"
		priority = 1
	case "btc_short_term":
		category = "data_driven"
		priority = 1
	case "volatility_arb":
		category = "data_driven"
		priority = 1
	case "contrarian_fear":
		category = "sentiment"
		priority = 2
	case "mm_behavior":
		category = "sentiment"
		priority = 2
	case "certainty_sweep":
		category = "speed"
		priority = 2
	case "liquidity_reward":
		category = "yield"
		priority = 2
	case "market_anomaly":
		category = "data_driven"
		priority = 2
	}

	enabled := false
	params := datatypes.JSON(ev.DefaultParams())
	stats := datatypes.JSON([]byte(`{}`))
	if existing != nil {
		enabled = existing.Enabled
		if len(existing.Params) > 0 {
			params = existing.Params
		}
		if len(existing.Stats) > 0 {
			stats = existing.Stats
		}
	}
	if existing == nil {
		if v, ok := defaultEnabled(e.StrategyDefaults, ev.Name()); ok {
			enabled = v
		}
	}
	item := &models.Strategy{
		Name:            ev.Name(),
		DisplayName:     ev.Name(),
		Description:     "",
		Category:        category,
		Enabled:         enabled,
		Priority:        priority,
		Params:          params,
		RequiredSignals: datatypes.JSON(req),
		Stats:           stats,
	}
	return e.Repo.UpsertStrategy(ctx, item)
}

func defaultEnabled(defaults map[string]any, name string) (bool, bool) {
	if len(defaults) == 0 || name == "" {
		return false, false
	}
	raw, ok := defaults[name]
	if !ok {
		return false, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return false, false
	}
	v, ok := m["enabled"]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
