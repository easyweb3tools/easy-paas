package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// LiquidityRewardStrategy (P2) consumes "liquidity_gap" and surfaces opportunities where wide spreads
// imply unusually cheap entry against a neutral (0.5) expected payout.
//
// This does not place maker orders; it only evaluates taker entry at best ask.
type LiquidityRewardStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct float64
}

func (s *LiquidityRewardStrategy) Name() string { return "liquidity_reward" }

func (s *LiquidityRewardStrategy) RequiredSignals() []string { return []string{"liquidity_gap"} }

func (s *LiquidityRewardStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.02}`)
}

func (s *LiquidityRewardStrategy) SetParams(raw json.RawMessage) error {
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

func (s *LiquidityRewardStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
	if s == nil || s.Repo == nil || len(signals) == 0 {
		return nil, nil
	}
	sig := signals[0]
	if sig.MarketID == nil || sig.TokenID == nil {
		return nil, nil
	}
	marketID := strings.TrimSpace(*sig.MarketID)
	yesTokenID := strings.TrimSpace(*sig.TokenID)
	if marketID == "" || yesTokenID == "" {
		return nil, nil
	}

	// Find NO token.
	toks, err := s.Repo.ListTokensByMarketIDs(ctx, []string{marketID})
	if err != nil || len(toks) == 0 {
		return nil, err
	}
	noTokenID := ""
	for _, t := range toks {
		if t.MarketID != marketID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(t.Outcome), "no") {
			noTokenID = t.ID
			break
		}
	}
	if noTokenID == "" {
		return nil, nil
	}

	minEdgeRaw := 0.02
	s.mu.RLock()
	if s.MinEdgePct > 0 {
		minEdgeRaw = s.MinEdgePct
	}
	s.mu.RUnlock()
	minEdge := decimal.NewFromFloat(minEdgeRaw)

	// Expected payout is neutral 0.5.
	expected := 0.5
	now := time.Now().UTC()

	type cand struct {
		tokenID   string
		direction string
		payout    float64
	}
	cands := []cand{
		{tokenID: yesTokenID, direction: "BUY_YES", payout: expected},
		{tokenID: noTokenID, direction: "BUY_NO", payout: expected},
	}

	best := models.Opportunity{}
	bestSet := false
	for _, c := range cands {
		books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{c.tokenID})
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
		expProfitPerShare := decimal.NewFromFloat(c.payout).Sub(askPrice)
		if expProfitPerShare.LessThanOrEqual(decimal.Zero) {
			continue
		}
		edgePct := expProfitPerShare.Div(askPrice)
		if edgePct.LessThan(minEdge) {
			continue
		}
		cost := askPrice.Mul(askSize)
		edgeUSD := expProfitPerShare.Mul(askSize)

		legs := []map[string]any{
			{
				"token_id":         c.tokenID,
				"market_id":        marketID,
				"direction":        c.direction,
				"target_price":     askPrice.InexactFloat64(),
				"current_best_ask": askPrice.InexactFloat64(),
				"fillable_size":    askSize.InexactFloat64(),
				"expected_payout":  expected,
			},
		}
		legsJSON, _ := json.Marshal(legs)
		marketIDsJSON, _ := json.Marshal([]string{marketID})
		signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

		reasoning := fmt.Sprintf("liquidity_reward market=%s side=%s entry=%s expected=0.50",
			marketID, c.direction, askPrice.StringFixed(4))

		opp := models.Opportunity{
			Status:          "active",
			EventID:         sig.EventID,
			PrimaryMarketID: strPtr(marketID),
			MarketIDs:       datatypes.JSON(marketIDsJSON),
			EdgePct:         edgePct,
			EdgeUSD:         edgeUSD,
			MaxSize:         cost,
			Confidence:      clamp01(sig.Strength),
			RiskScore:       0.9,
			DecayType:       "none",
			ExpiresAt:       sig.ExpiresAt,
			Legs:            datatypes.JSON(legsJSON),
			SignalIDs:       datatypes.JSON(signalIDsJSON),
			Reasoning:       reasoning,
			DataAgeMs:       int(time.Since(books[0].UpdatedAt).Milliseconds()),
			Warnings:        datatypes.JSON([]byte(`["wide_spread"]`)),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if !bestSet || opp.EdgeUSD.GreaterThan(best.EdgeUSD) {
			best = opp
			bestSet = true
		}
	}
	if !bestSet {
		return nil, nil
	}
	return []models.Opportunity{best}, nil
}
