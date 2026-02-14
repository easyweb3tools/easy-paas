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

// BTCShortTermStrategy is P1: uses Binance depth imbalance to trade BTC 15-minute up/down binary markets.
// MVP: only uses markets labeled "btc_15min" and questions containing "up" or "down".
type BTCShortTermStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct float64
}

func (s *BTCShortTermStrategy) Name() string { return "btc_short_term" }

func (s *BTCShortTermStrategy) RequiredSignals() []string { return []string{"btc_depth_imbalance"} }

func (s *BTCShortTermStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.03}`)
}

func (s *BTCShortTermStrategy) SetParams(raw json.RawMessage) error {
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

var reUpDown = regexp.MustCompile(`(?i)\b(up|down)\b`)

func (s *BTCShortTermStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
	if s == nil || s.Repo == nil || len(signals) == 0 {
		return nil, nil
	}
	sig := signals[0]
	if sig.Direction != "YES" && sig.Direction != "NO" {
		return nil, nil
	}
	bullish := sig.Direction == "YES"

	label := "btc_15min"
	labels, err := s.Repo.ListMarketLabels(ctx, repository.ListMarketLabelsParams{
		Limit:   2000,
		Offset:  0,
		Label:   &label,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
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

	s.mu.RLock()
	minEdgeRaw := s.MinEdgePct
	s.mu.RUnlock()
	if minEdgeRaw <= 0 {
		minEdgeRaw = 0.03
	}
	minEdgePct := decimal.NewFromFloat(minEdgeRaw)
	now := time.Now().UTC()

	out := make([]models.Opportunity, 0, 8)
	for _, m := range markets {
		q := strings.TrimSpace(m.Question)
		if q == "" {
			continue
		}
		dir := marketUpDown(q)
		if dir == "" {
			continue
		}
		// Translate signal into a crude probability for this market.
		pYes := 0.5
		switch dir {
		case "up":
			if bullish {
				pYes = 0.60
			} else {
				pYes = 0.40
			}
		case "down":
			if bullish {
				pYes = 0.40
			} else {
				pYes = 0.60
			}
		}

		yesToken := tokenByMarketOutcome[m.ID]["yes"]
		noToken := tokenByMarketOutcome[m.ID]["no"]
		if yesToken == "" || noToken == "" {
			continue
		}

		opp, ok := s.bestSideOpportunity(ctx, sig, m.ID, yesToken, noToken, q, pYes, minEdgePct, now)
		if !ok {
			continue
		}
		out = append(out, opp)
	}
	return out, nil
}

func (s *BTCShortTermStrategy) bestSideOpportunity(
	ctx context.Context,
	sig models.Signal,
	marketID string,
	yesToken string,
	noToken string,
	question string,
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
				"p_yes":            pYes,
			},
		}
		legsJSON, _ := json.Marshal(legs)
		marketIDsJSON, _ := json.Marshal([]string{marketID})
		signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

		reasoning := fmt.Sprintf("btc_short_term market=%s p_yes=%.2f entry=%s question=%q",
			marketID, pYes, askPrice.StringFixed(4), question)

		opp := models.Opportunity{
			Status:          "active",
			EventID:         nil,
			PrimaryMarketID: strPtr(marketID),
			MarketIDs:       datatypes.JSON(marketIDsJSON),
			EdgePct:         edgePct,
			EdgeUSD:         edgeUSD,
			MaxSize:         cost,
			Confidence:      clamp01(sig.Strength),
			RiskScore:       0.75,
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

func marketUpDown(q string) string {
	m := reUpDown.FindStringSubmatch(q)
	if len(m) < 2 {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(m[1]))
	if v != "up" && v != "down" {
		return ""
	}
	return v
}
