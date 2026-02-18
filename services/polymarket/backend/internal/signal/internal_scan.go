package signal

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	polymarketclob "polymarket/internal/client/polymarket/clob"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type InternalScanCollector struct {
	Repo   repository.Repository
	Logger *zap.Logger

	Interval     time.Duration
	MinSpreadBps float64
	Limit        int

	// S1 arb-sum signal tuning (P0).
	ArbCandidateEvents int
	ArbMinMarkets      int
	ArbMinLiquidityUSD float64
	ArbMinDeviationPct float64

	// S2 systematic no-bias signal tuning (P0).
	NoBiasLabels     []string
	NoBiasPriceMin   float64
	NoBiasPriceMax   float64
	NoBiasMinEVPct   float64
	NoBiasMaxPerTick int

	lastNoBias map[string]time.Time

	mu      sync.Mutex
	lastRun *time.Time
	lastErr *string

	// S3 pre-market fdv signal tuning (P0).
	FDVEntryMinDays int
	FDVEntryMaxDays int
	FDVNoPriceMin   float64
	FDVNoPriceMax   float64
	FDVMaxPerTick   int

	// S4 price anomaly signal tuning.
	AnomalyExtremeHigh float64 // default 0.95
	AnomalyExtremeLow  float64 // default 0.05
	AnomalyMaxPerTick  int     // default 100
	lastAnomaly        map[string]time.Time
}

func (c *InternalScanCollector) Name() string { return "internal_scan" }

func (c *InternalScanCollector) Start(ctx context.Context, out chan<- models.Signal) error {
	interval := c.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	limit := c.Limit
	if limit <= 0 {
		limit = 100
	}
	minSpread := c.MinSpreadBps
	if minSpread <= 0 {
		minSpread = 200
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if c.Repo == nil {
				continue
			}
			now := time.Now().UTC()
			c.setRun(now, nil)
			c.emitLiquidityGap(ctx, out, now, limit, minSpread)
			c.emitArbSumDeviation(ctx, out, now)
			c.emitNoBias(ctx, out, now)
			c.emitFDVOverpriced(ctx, out, now)
			c.emitPriceAnomaly(ctx, out, now)
		}
	}
}

func (c *InternalScanCollector) emitLiquidityGap(ctx context.Context, out chan<- models.Signal, now time.Time, limit int, minSpread float64) {
	rows, err := c.Repo.ListMarketDataHealthCandidates(ctx, limit, minSpread)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Warn("internal scan liquidity gap failed", zap.Error(err))
		}
		return
	}
	tokenIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.TokenID) != "" {
			tokenIDs = append(tokenIDs, strings.TrimSpace(row.TokenID))
		}
	}
	tokenByID := map[string]models.Token{}
	if len(tokenIDs) > 0 {
		toks, _ := c.Repo.ListTokensByIDs(ctx, tokenIDs)
		for _, t := range toks {
			tokenByID[t.ID] = t
		}
	}
	for _, row := range rows {
		spreadBps := 0.0
		if row.SpreadBps != nil {
			spreadBps = *row.SpreadBps
		}
		payload := datatypes.JSON([]byte(`{}`))
		tok := tokenByID[row.TokenID]
		marketID := ""
		if tok.MarketID != "" {
			marketID = tok.MarketID
		}
		// Keep liquidity_gap signals on YES tokens only for strategy consistency.
		if !strings.EqualFold(strings.TrimSpace(tok.Outcome), "yes") {
			continue
		}
		out <- models.Signal{
			SignalType: "liquidity_gap",
			Source:     "internal_scan",
			MarketID:   strPtr(marketID),
			TokenID:    strPtr(row.TokenID),
			Strength:   clamp01(spreadBps / 1000.0), // 1000bps => 1.0
			Direction:  "NEUTRAL",
			Payload:    payload,
			CreatedAt:  now,
		}
	}
}

func (c *InternalScanCollector) emitArbSumDeviation(ctx context.Context, out chan<- models.Signal, now time.Time) {
	candidateEvents := c.ArbCandidateEvents
	if candidateEvents <= 0 {
		candidateEvents = 200
	}
	minMarkets := c.ArbMinMarkets
	if minMarkets <= 0 {
		minMarkets = 2
	}
	minLiq := c.ArbMinLiquidityUSD
	if minLiq <= 0 {
		minLiq = 1000
	}
	minDevPct := c.ArbMinDeviationPct
	if minDevPct <= 0 {
		minDevPct = 2.0
	}

	aggs, err := c.Repo.ListMarketAggregates(ctx, candidateEvents)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Warn("internal scan aggregates failed", zap.Error(err))
		}
		return
	}
	for _, agg := range aggs {
		if agg.EventID == "" || agg.MarketCount < minMarkets {
			continue
		}
		if agg.SumLiquidity.LessThan(decimal.NewFromFloat(minLiq)) {
			continue
		}
		eventID := strings.TrimSpace(agg.EventID)
		markets, err := c.Repo.ListMarketsByEventID(ctx, eventID)
		if err != nil {
			continue
		}
		if len(markets) < minMarkets {
			continue
		}
		marketIDs := make([]string, 0, len(markets))
		for _, m := range markets {
			if m.ID != "" {
				marketIDs = append(marketIDs, m.ID)
			}
		}
		tokens, err := c.Repo.ListTokensByMarketIDs(ctx, marketIDs)
		if err != nil {
			continue
		}
		yesTokenByMarket := map[string]string{}
		for _, tok := range tokens {
			if tok.MarketID == "" || tok.ID == "" {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(tok.Outcome), "yes") {
				yesTokenByMarket[tok.MarketID] = tok.ID
			}
		}
		yesTokenIDs := make([]string, 0, len(yesTokenByMarket))
		for _, m := range markets {
			if id := yesTokenByMarket[m.ID]; id != "" {
				yesTokenIDs = append(yesTokenIDs, id)
			}
		}
		if len(yesTokenIDs) < minMarkets {
			continue
		}
		books, _ := c.Repo.ListOrderbookLatestByTokenIDs(ctx, yesTokenIDs)
		trades, _ := c.Repo.ListLastTradePricesByTokenIDs(ctx, yesTokenIDs)
		bookByToken := map[string]models.OrderbookLatest{}
		for _, b := range books {
			bookByToken[b.TokenID] = b
		}
		tradeByToken := map[string]models.LastTradePrice{}
		for _, tr := range trades {
			tradeByToken[tr.TokenID] = tr
		}

		sum := 0.0
		prices := map[string]float64{} // token -> price used
		for _, tokenID := range yesTokenIDs {
			price, ok := currentPrice(bookByToken[tokenID], tradeByToken[tokenID])
			if !ok {
				sum = 0
				break
			}
			sum += price
			prices[tokenID] = price
		}
		if sum <= 0 {
			continue
		}
		devPct := math.Abs(sum-1.0) * 100.0
		if devPct < minDevPct {
			continue
		}
		direction := "BOTH"
		if sum < 1.0 {
			direction = "YES"
		} else if sum > 1.0 {
			direction = "NO"
		}
		payload, _ := json.Marshal(map[string]any{
			"sum":           sum,
			"deviation_pct": devPct,
			"yes_token_ids": yesTokenIDs,
			"prices":        prices,
		})
		out <- models.Signal{
			SignalType: "arb_sum_deviation",
			Source:     "internal_scan",
			EventID:    strPtr(eventID),
			Strength:   clamp01(devPct / 10.0), // 10% => 1.0
			Direction:  direction,
			Payload:    datatypes.JSON(payload),
			CreatedAt:  now,
		}
	}
}

func (c *InternalScanCollector) emitNoBias(ctx context.Context, out chan<- models.Signal, now time.Time) {
	labels := c.NoBiasLabels
	if len(labels) == 0 {
		labels = []string{"pre_market_fdv", "geopolitical", "safe_no", "tge_deadline"}
	}
	priceMin := c.NoBiasPriceMin
	if priceMin <= 0 {
		priceMin = 0.20
	}
	priceMax := c.NoBiasPriceMax
	if priceMax <= 0 {
		priceMax = 0.55
	}
	minEV := c.NoBiasMinEVPct
	if minEV <= 0 {
		minEV = 10.0
	}
	maxPerTick := c.NoBiasMaxPerTick
	if maxPerTick <= 0 {
		maxPerTick = 200
	}
	if c.lastNoBias == nil {
		c.lastNoBias = map[string]time.Time{}
	}
	labelSet := map[string]struct{}{}
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l != "" {
			labelSet[l] = struct{}{}
		}
	}

	// Prefer learned historical rates when available (from SettlementHistoryCollector).
	labelRates := map[string]float64{}
	minSamples := int64(10)
	if strat, err := c.Repo.GetStrategyByName(ctx, "systematic_no"); err == nil && strat != nil && len(strat.Stats) > 0 {
		var stats map[string]any
		if err := json.Unmarshal(strat.Stats, &stats); err == nil {
			if v, ok := stats["learned_no_rate_min_samples"].(float64); ok && v >= 1 {
				minSamples = int64(v)
			}
			// Primary: category_no_rates map.
			if m, ok := stats["category_no_rates"].(map[string]any); ok {
				for k, raw := range m {
					label := strings.TrimSpace(k)
					if label == "" {
						continue
					}
					if f, ok := raw.(float64); ok && f > 0 && f < 1 {
						labelRates[label] = f
					}
				}
			}
		}
	}
	// Fallback: compute from DB on demand.
	if len(labelRates) == 0 {
		stats, _ := c.Repo.ListLabelNoRateStats(ctx, labels)
		for _, row := range stats {
			if strings.TrimSpace(row.Label) == "" {
				continue
			}
			if row.Total >= minSamples {
				labelRates[row.Label] = row.NoRate
			}
		}
	}
	rows, err := c.Repo.ListMarketLabels(ctx, repository.ListMarketLabelsParams{
		Limit:   2000,
		Offset:  0,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
	})
	if err != nil {
		return
	}
	marketToLabel := map[string]string{}
	marketIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, ok := labelSet[r.Label]; !ok {
			continue
		}
		if r.MarketID == "" {
			continue
		}
		if _, exists := marketToLabel[r.MarketID]; exists {
			continue
		}
		marketToLabel[r.MarketID] = r.Label
		marketIDs = append(marketIDs, r.MarketID)
		if len(marketIDs) >= 2000 {
			break
		}
	}
	if len(marketIDs) == 0 {
		return
	}
	tokens, err := c.Repo.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil {
		return
	}
	noTokenByMarket := map[string]string{}
	for _, tok := range tokens {
		if tok.MarketID == "" || tok.ID == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(tok.Outcome), "no") {
			noTokenByMarket[tok.MarketID] = tok.ID
		}
	}
	noTokenIDs := make([]string, 0, len(noTokenByMarket))
	for _, marketID := range marketIDs {
		if tid := noTokenByMarket[marketID]; tid != "" {
			noTokenIDs = append(noTokenIDs, tid)
		}
	}
	if len(noTokenIDs) == 0 {
		return
	}
	books, _ := c.Repo.ListOrderbookLatestByTokenIDs(ctx, noTokenIDs)
	trades, _ := c.Repo.ListLastTradePricesByTokenIDs(ctx, noTokenIDs)
	bookByToken := map[string]models.OrderbookLatest{}
	for _, b := range books {
		bookByToken[b.TokenID] = b
	}
	tradeByToken := map[string]models.LastTradePrice{}
	for _, tr := range trades {
		tradeByToken[tr.TokenID] = tr
	}

	emitted := 0
	for marketID, tokenID := range noTokenByMarket {
		if emitted >= maxPerTick {
			break
		}
		// Cooldown per token to avoid spamming signals.
		if last, ok := c.lastNoBias[tokenID]; ok && now.Sub(last) < 10*time.Minute {
			continue
		}
		price, ok := currentPrice(bookByToken[tokenID], tradeByToken[tokenID])
		if !ok {
			continue
		}
		if price < priceMin || price > priceMax {
			continue
		}
		label := marketToLabel[marketID]
		noRate := categoryNoRate(label)
		if v, ok := labelRates[label]; ok && v > 0 && v < 1 {
			noRate = v
		}
		// Expected profit relative to cost.
		evPct := ((noRate - price) / price) * 100.0
		if evPct < minEV {
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"label":      label,
			"no_rate":    noRate,
			"no_price":   price,
			"ev_pct":     evPct,
			"price_min":  priceMin,
			"price_max":  priceMax,
			"min_ev_pct": minEV,
		})
		out <- models.Signal{
			SignalType: "no_bias",
			Source:     "internal_scan",
			MarketID:   strPtr(marketID),
			TokenID:    strPtr(tokenID),
			Strength:   clamp01(evPct / 100.0),
			Direction:  "NO",
			Payload:    datatypes.JSON(payload),
			CreatedAt:  now,
		}
		c.lastNoBias[tokenID] = now
		emitted++
	}
}

func categoryNoRate(label string) float64 {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "pre_market_fdv":
		return 0.85
	case "geopolitical":
		return 0.90
	case "safe_no":
		return 0.95
	case "tge_deadline":
		return 0.80
	default:
		return 0.806
	}
}

func currentPrice(book models.OrderbookLatest, trade models.LastTradePrice) (float64, bool) {
	if book.Mid != nil && *book.Mid > 0 {
		return *book.Mid, true
	}
	if book.BestBid != nil && book.BestAsk != nil && *book.BestBid > 0 && *book.BestAsk > 0 {
		return (*book.BestBid + *book.BestAsk) / 2.0, true
	}
	if trade.Price > 0 {
		return trade.Price, true
	}
	return 0, false
}

func (c *InternalScanCollector) Stop() error { return nil }

func (c *InternalScanCollector) Health() HealthStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return HealthStatus{
		Status:     "healthy",
		LastPollAt: c.lastRun,
		LastError:  c.lastErr,
		Details:    map[string]any{"collector": "internal_scan"},
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ensure clob types stay referenced since orderbooks are serialized similarly.
var _ = polymarketclob.OrderBook{}

func boolPtr(v bool) *bool { return &v }

func (c *InternalScanCollector) SourceInfo() SourceInfo {
	return SourceInfo{
		SourceType:   "internal_scan",
		Endpoint:     "db",
		PollInterval: c.Interval,
	}
}

func (c *InternalScanCollector) setRun(now time.Time, err error) {
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

func (c *InternalScanCollector) emitFDVOverpriced(ctx context.Context, out chan<- models.Signal, now time.Time) {
	entryMin := c.FDVEntryMinDays
	entryMax := c.FDVEntryMaxDays
	if entryMin <= 0 {
		entryMin = 14
	}
	if entryMax <= 0 {
		entryMax = 28
	}
	noMin := c.FDVNoPriceMin
	noMax := c.FDVNoPriceMax
	if noMin <= 0 {
		noMin = 0.35
	}
	if noMax <= 0 {
		noMax = 0.55
	}
	maxPerTick := c.FDVMaxPerTick
	if maxPerTick <= 0 {
		maxPerTick = 50
	}

	// Get pre_market_fdv labels.
	label := "pre_market_fdv"
	rows, err := c.Repo.ListMarketLabels(ctx, repository.ListMarketLabelsParams{
		Limit:   2000,
		Offset:  0,
		Label:   &label,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
	})
	if err != nil || len(rows) == 0 {
		return
	}
	marketIDs := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, r := range rows {
		if r.MarketID == "" {
			continue
		}
		if _, ok := seen[r.MarketID]; ok {
			continue
		}
		seen[r.MarketID] = struct{}{}
		marketIDs = append(marketIDs, r.MarketID)
		if len(marketIDs) >= 2000 {
			break
		}
	}
	markets, err := c.Repo.ListMarketsByIDs(ctx, marketIDs)
	if err != nil || len(markets) == 0 {
		return
	}
	eventIDs := make([]string, 0, len(markets))
	evSeen := map[string]struct{}{}
	for _, m := range markets {
		if m.EventID == "" {
			continue
		}
		if _, ok := evSeen[m.EventID]; ok {
			continue
		}
		evSeen[m.EventID] = struct{}{}
		eventIDs = append(eventIDs, m.EventID)
	}
	events, _ := c.Repo.ListEventsByIDs(ctx, eventIDs)
	eventByID := map[string]models.Event{}
	for _, e := range events {
		eventByID[e.ID] = e
	}

	tokens, err := c.Repo.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil || len(tokens) == 0 {
		return
	}
	noTokenByMarket := map[string]string{}
	for _, tok := range tokens {
		if tok.MarketID == "" || tok.ID == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(tok.Outcome), "no") {
			noTokenByMarket[tok.MarketID] = tok.ID
		}
	}
	noTokenIDs := make([]string, 0, len(noTokenByMarket))
	for _, m := range markets {
		if tid := noTokenByMarket[m.ID]; tid != "" {
			noTokenIDs = append(noTokenIDs, tid)
		}
	}
	if len(noTokenIDs) == 0 {
		return
	}
	books, _ := c.Repo.ListOrderbookLatestByTokenIDs(ctx, noTokenIDs)
	trades, _ := c.Repo.ListLastTradePricesByTokenIDs(ctx, noTokenIDs)
	bookByToken := map[string]models.OrderbookLatest{}
	for _, b := range books {
		bookByToken[b.TokenID] = b
	}
	tradeByToken := map[string]models.LastTradePrice{}
	for _, tr := range trades {
		tradeByToken[tr.TokenID] = tr
	}

	emitted := 0
	for _, m := range markets {
		if emitted >= maxPerTick {
			break
		}
		ev := eventByID[m.EventID]
		if ev.EndTime == nil || ev.EndTime.IsZero() {
			continue
		}
		daysToEnd := int(ev.EndTime.Sub(now).Hours() / 24)
		if daysToEnd < entryMin || daysToEnd > entryMax {
			continue
		}
		tokenID := noTokenByMarket[m.ID]
		if tokenID == "" {
			continue
		}
		price, ok := currentPrice(bookByToken[tokenID], tradeByToken[tokenID])
		if !ok {
			continue
		}
		if price < noMin || price > noMax {
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"label":            "pre_market_fdv",
			"days_to_end":      daysToEnd,
			"entry_window":     []int{entryMin, entryMax},
			"no_price":         price,
			"no_sweet_spot":    []float64{noMin, noMax},
			"expected_no_rate": 0.85,
		})
		expires := *ev.EndTime
		out <- models.Signal{
			SignalType: "fdv_overpriced",
			Source:     "internal_scan",
			EventID:    strPtr(m.EventID),
			MarketID:   strPtr(m.ID),
			TokenID:    strPtr(tokenID),
			Strength:   0.7,
			Direction:  "NO",
			Payload:    datatypes.JSON(payload),
			ExpiresAt:  &expires,
			CreatedAt:  now,
		}
		emitted++
	}
}

func (c *InternalScanCollector) emitPriceAnomaly(ctx context.Context, out chan<- models.Signal, now time.Time) {
	extremeHigh := c.AnomalyExtremeHigh
	if extremeHigh <= 0 {
		extremeHigh = 0.95
	}
	extremeLow := c.AnomalyExtremeLow
	if extremeLow <= 0 {
		extremeLow = 0.05
	}
	maxPerTick := c.AnomalyMaxPerTick
	if maxPerTick <= 0 {
		maxPerTick = 100
	}
	if c.lastAnomaly == nil {
		c.lastAnomaly = map[string]time.Time{}
	}

	// Reuse the same data source as liquidity_gap.
	rows, err := c.Repo.ListMarketDataHealthCandidates(ctx, 2000, 0)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Warn("internal scan price anomaly candidates failed", zap.Error(err))
		}
		return
	}
	tokenIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.TokenID) != "" {
			tokenIDs = append(tokenIDs, strings.TrimSpace(row.TokenID))
		}
	}
	if len(tokenIDs) == 0 {
		return
	}

	// Load YES tokens only.
	toks, _ := c.Repo.ListTokensByIDs(ctx, tokenIDs)
	yesTokenIDs := make([]string, 0, len(toks))
	tokenByID := map[string]models.Token{}
	for _, t := range toks {
		tokenByID[t.ID] = t
		if strings.EqualFold(strings.TrimSpace(t.Outcome), "yes") {
			yesTokenIDs = append(yesTokenIDs, t.ID)
		}
	}
	if len(yesTokenIDs) == 0 {
		return
	}

	books, _ := c.Repo.ListOrderbookLatestByTokenIDs(ctx, yesTokenIDs)
	trades, _ := c.Repo.ListLastTradePricesByTokenIDs(ctx, yesTokenIDs)
	bookByToken := map[string]models.OrderbookLatest{}
	for _, b := range books {
		bookByToken[b.TokenID] = b
	}
	tradeByToken := map[string]models.LastTradePrice{}
	for _, tr := range trades {
		tradeByToken[tr.TokenID] = tr
	}

	emitted := 0
	for _, tokenID := range yesTokenIDs {
		if emitted >= maxPerTick {
			break
		}
		// 10-minute cooldown per token.
		if last, ok := c.lastAnomaly[tokenID]; ok && now.Sub(last) < 10*time.Minute {
			continue
		}
		price, ok := currentPrice(bookByToken[tokenID], tradeByToken[tokenID])
		if !ok {
			continue
		}

		var anomalyType, direction string
		var strength float64
		if price <= extremeLow {
			anomalyType = "extreme_cheap"
			direction = "YES"
			strength = clamp01((extremeLow - price) / extremeLow) // closer to 0 => stronger
		} else if price >= extremeHigh {
			anomalyType = "extreme_expensive"
			direction = "NO"
			strength = clamp01((price - extremeHigh) / (1.0 - extremeHigh))
		} else {
			continue
		}

		tok := tokenByID[tokenID]
		marketID := tok.MarketID

		payload, _ := json.Marshal(map[string]any{
			"anomaly_type": anomalyType,
			"yes_price":    price,
		})
		out <- models.Signal{
			SignalType: "price_anomaly",
			Source:     "internal_scan",
			MarketID:   strPtr(marketID),
			TokenID:    strPtr(tokenID),
			Strength:   strength,
			Direction:  direction,
			Payload:    datatypes.JSON(payload),
			CreatedAt:  now,
		}
		c.lastAnomaly[tokenID] = now
		emitted++
	}
}
