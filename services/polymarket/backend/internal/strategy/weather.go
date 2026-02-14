package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// WeatherStrategy is P1: uses weather forecast signal + labeled markets to find mispriced binary "above/below" markets.
// MVP: only supports markets with questions containing "above N" or "below N".
type WeatherStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct float64
}

func (s *WeatherStrategy) Name() string { return "weather" }

func (s *WeatherStrategy) RequiredSignals() []string { return []string{"weather_deviation"} }

func (s *WeatherStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.05,"min_confidence":0.6}`)
}

func (s *WeatherStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		MinEdgePct *float64 `json:"min_edge_pct"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.MinEdgePct != nil {
		s.MinEdgePct = *p.MinEdgePct
	}
	return nil
}

var (
	reAbove = regexp.MustCompile(`(?i)\babove\s+(-?\d{1,3})\b`)
	reBelow = regexp.MustCompile(`(?i)\bbelow\s+(-?\d{1,3})\b`)
)

func (s *WeatherStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
	if s == nil || s.Repo == nil || len(signals) == 0 {
		return nil, nil
	}
	sig := signals[0]
	var payload struct {
		City          string  `json:"city"`
		ForecastTempF float64 `json:"forecast_temp_f"`
	}
	_ = json.Unmarshal(sig.Payload, &payload)
	city := strings.ToLower(strings.TrimSpace(payload.City))
	if city == "" {
		return nil, nil
	}

	label := "weather"
	labels, err := s.Repo.ListMarketLabels(ctx, repository.ListMarketLabelsParams{
		Limit:    1000,
		Offset:   0,
		Label:    &label,
		SubLabel: &city,
		OrderBy:  "created_at",
		Asc:      boolPtr(false),
	})
	if err != nil || len(labels) == 0 {
		return nil, err
	}
	marketIDs := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, it := range labels {
		id := strings.TrimSpace(it.MarketID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		marketIDs = append(marketIDs, id)
	}
	markets, err := s.Repo.ListMarketsByIDs(ctx, marketIDs)
	if err != nil || len(markets) == 0 {
		return nil, err
	}
	tokens, err := s.Repo.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil || len(tokens) == 0 {
		return nil, err
	}
	tokenByMarketOutcome := map[string]map[string]string{}
	for _, t := range tokens {
		if tokenByMarketOutcome[t.MarketID] == nil {
			tokenByMarketOutcome[t.MarketID] = map[string]string{}
		}
		tokenByMarketOutcome[t.MarketID][strings.ToLower(strings.TrimSpace(t.Outcome))] = t.ID
	}

	// Evaluate each market independently.
	out := make([]models.Opportunity, 0, 8)
	now := time.Now().UTC()
	s.mu.RLock()
	minEdgePctRaw := s.MinEdgePct
	s.mu.RUnlock()
	if minEdgePctRaw <= 0 {
		minEdgePctRaw = 0.05
	}
	minEdgePct := decimal.NewFromFloat(minEdgePctRaw)

	for _, m := range markets {
		q := strings.TrimSpace(m.Question)
		if q == "" {
			continue
		}
		threshold, mode, ok := parseAboveBelowThreshold(q)
		if !ok {
			continue
		}
		pYes := impliedYesProb(payload.ForecastTempF, threshold, mode)

		yesToken := tokenByMarketOutcome[m.ID]["yes"]
		noToken := tokenByMarketOutcome[m.ID]["no"]
		if yesToken == "" || noToken == "" {
			continue
		}

		// Prefer the side with higher edge.
		opp, ok := s.bestSideOpportunity(ctx, sig, m.ID, yesToken, noToken, q, city, payload.ForecastTempF, threshold, mode, pYes, minEdgePct, now)
		if !ok {
			continue
		}
		out = append(out, opp)
	}
	return out, nil
}

func (s *WeatherStrategy) bestSideOpportunity(
	ctx context.Context,
	sig models.Signal,
	marketID string,
	yesToken string,
	noToken string,
	question string,
	city string,
	forecast float64,
	threshold int,
	mode string,
	pYes float64,
	minEdgePct decimal.Decimal,
	now time.Time,
) (models.Opportunity, bool) {
	type cand struct {
		direction string
		tokenID   string
		payoutP   float64
	}
	choices := []cand{
		{direction: "BUY_YES", tokenID: yesToken, payoutP: pYes},
		{direction: "BUY_NO", tokenID: noToken, payoutP: 1.0 - pYes},
	}
	best := models.Opportunity{}
	bestSet := false
	for _, ch := range choices {
		books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{ch.tokenID})
		if len(books) == 0 {
			continue
		}
		askPrice, askSize, ok := bestAsk(books[0])
		if !ok || askPrice.LessThanOrEqual(decimal.Zero) {
			continue
		}
		if askSize.LessThanOrEqual(decimal.Zero) {
			// Still allow; bestAsk can return size=0 for BestAsk-only rows.
			askSize = decimal.NewFromInt(10)
		}
		expProfitPerShare := decimal.NewFromFloat(ch.payoutP).Sub(askPrice)
		if expProfitPerShare.LessThanOrEqual(decimal.Zero) {
			continue
		}
		edgePct := expProfitPerShare.Div(askPrice)
		if edgePct.LessThan(minEdgePct) {
			continue
		}
		cost := askPrice.Mul(askSize)
		edgeUSD := expProfitPerShare.Mul(askSize)

		legs := []map[string]any{
			{
				"token_id":         ch.tokenID,
				"market_id":        marketID,
				"direction":        ch.direction,
				"target_price":     askPrice.InexactFloat64(),
				"current_best_ask": askPrice.InexactFloat64(),
				"fillable_size":    askSize.InexactFloat64(),
				"city":             city,
				"forecast_temp_f":  forecast,
				"threshold":        threshold,
				"mode":             mode,
				"p_yes":            pYes,
			},
		}
		legsJSON, _ := json.Marshal(legs)
		marketIDsJSON, _ := json.Marshal([]string{marketID})
		signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

		reasoning := fmt.Sprintf("weather market=%s city=%s %s %dF forecast=%.1fF p_yes=%.2f entry=%s",
			marketID, city, mode, threshold, forecast, pYes, askPrice.StringFixed(4))

		opp := models.Opportunity{
			Status:          "active",
			EventID:         nil,
			PrimaryMarketID: strPtr(marketID),
			MarketIDs:       datatypes.JSON(marketIDsJSON),
			EdgePct:         edgePct,
			EdgeUSD:         edgeUSD,
			MaxSize:         cost,
			Confidence:      clamp01(sig.Strength),
			RiskScore:       0.7,
			DecayType:       "exponential",
			ExpiresAt:       sig.ExpiresAt,
			Legs:            datatypes.JSON(legsJSON),
			SignalIDs:       datatypes.JSON(signalIDsJSON),
			Reasoning:       reasoning,
			DataAgeMs:       int(time.Since(books[0].UpdatedAt).Milliseconds()),
			Warnings:        datatypes.JSON([]byte(`[]`)),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if !bestSet || opp.EdgeUSD.GreaterThan(best.EdgeUSD) {
			best = opp
			bestSet = true
		}
	}
	return best, bestSet
}

func parseAboveBelowThreshold(q string) (int, string, bool) {
	if m := reAbove.FindStringSubmatch(q); len(m) >= 2 {
		n, ok := atoiSafe(m[1])
		if ok {
			return n, "above", true
		}
	}
	if m := reBelow.FindStringSubmatch(q); len(m) >= 2 {
		n, ok := atoiSafe(m[1])
		if ok {
			return n, "below", true
		}
	}
	return 0, "", false
}

func impliedYesProb(forecast float64, threshold int, mode string) float64 {
	t := float64(threshold)
	// Simple heuristic: linear probability from diff in [-10, +10] around 0.5.
	diff := forecast - t
	if mode == "below" {
		diff = t - forecast
	}
	p := 0.5 + diff/20.0
	return clamp01(p)
}

func atoiSafe(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	sign := 1
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return sign * n, true
}

func boolPtr(v bool) *bool { return &v }
