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

// CertaintySweepStrategy (P2) consumes "certainty_sweep" and proposes trades when the market is priced near certainty.
// MVP:
// - If YES ask >= 0.97: BUY_YES if edge vs expected 0.995 is positive.
// - If YES ask <= 0.03: BUY_NO if edge vs expected 0.995 is positive.
type CertaintySweepStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct float64
}

func (s *CertaintySweepStrategy) Name() string { return "certainty_sweep" }

func (s *CertaintySweepStrategy) RequiredSignals() []string { return []string{"certainty_sweep"} }

func (s *CertaintySweepStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.01}`)
}

func (s *CertaintySweepStrategy) SetParams(raw json.RawMessage) error {
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

func (s *CertaintySweepStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
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

	yesBooks, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{yesTokenID})
	if len(yesBooks) == 0 {
		return nil, nil
	}
	yesAsk, _, ok := bestAsk(yesBooks[0])
	if !ok || yesAsk.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}

	side := ""
	expPayout := 0.0
	tokenID := ""
	switch {
	case yesAsk.GreaterThanOrEqual(decimal.NewFromFloat(0.97)):
		side = "BUY_YES"
		tokenID = yesTokenID
		expPayout = 0.995
	case yesAsk.LessThanOrEqual(decimal.NewFromFloat(0.03)):
		// Need NO token ID.
		toks, err := s.Repo.ListTokensByMarketIDs(ctx, []string{marketID})
		if err != nil {
			return nil, err
		}
		noTokenID := ""
		for _, t := range toks {
			if t.MarketID == marketID && strings.EqualFold(strings.TrimSpace(t.Outcome), "no") {
				noTokenID = t.ID
				break
			}
		}
		if noTokenID == "" {
			return nil, nil
		}
		side = "BUY_NO"
		tokenID = noTokenID
		expPayout = 0.995
	default:
		return nil, nil
	}

	books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{tokenID})
	if len(books) == 0 {
		return nil, nil
	}
	askPrice, askSize, ok := bestAsk(books[0])
	if !ok || askPrice.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	if askSize.LessThanOrEqual(decimal.Zero) {
		askSize = decimal.NewFromInt(10)
	}

	expProfitPerShare := decimal.NewFromFloat(expPayout).Sub(askPrice)
	if expProfitPerShare.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	edgePct := expProfitPerShare.Div(askPrice)
	s.mu.RLock()
	minEdgeRaw := s.MinEdgePct
	s.mu.RUnlock()
	if minEdgeRaw <= 0 {
		minEdgeRaw = 0.01
	}
	if edgePct.LessThan(decimal.NewFromFloat(minEdgeRaw)) {
		return nil, nil
	}
	cost := askPrice.Mul(askSize)
	edgeUSD := expProfitPerShare.Mul(askSize)

	legs := []map[string]any{
		{
			"token_id":         tokenID,
			"market_id":        marketID,
			"direction":        side,
			"target_price":     askPrice.InexactFloat64(),
			"current_best_ask": askPrice.InexactFloat64(),
			"fillable_size":    askSize.InexactFloat64(),
			"expected_payout":  expPayout,
		},
	}
	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal([]string{marketID})
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

	reasoning := fmt.Sprintf("certainty_sweep market=%s side=%s entry=%s expected_payout=%.3f",
		marketID, side, askPrice.StringFixed(4), expPayout)
	now := time.Now().UTC()

	opp := models.Opportunity{
		Status:          "active",
		EventID:         sig.EventID,
		PrimaryMarketID: strPtr(marketID),
		MarketIDs:       datatypes.JSON(marketIDsJSON),
		EdgePct:         edgePct,
		EdgeUSD:         edgeUSD,
		MaxSize:         cost,
		Confidence:      clamp01(sig.Strength),
		RiskScore:       0.4,
		DecayType:       "step",
		ExpiresAt:       sig.ExpiresAt,
		Legs:            datatypes.JSON(legsJSON),
		SignalIDs:       datatypes.JSON(signalIDsJSON),
		Reasoning:       reasoning,
		DataAgeMs:       int(time.Since(books[0].UpdatedAt).Milliseconds()),
		Warnings:        datatypes.JSON([]byte(`[]`)),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return []models.Opportunity{opp}, nil
}
