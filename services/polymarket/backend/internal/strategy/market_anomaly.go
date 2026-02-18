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

// MarketAnomalyStrategy consumes "price_anomaly" signals and applies mean-reversion
// logic to extreme price outliers (YES < 0.05 or YES > 0.95).
type MarketAnomalyStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct       float64
	MeanRevertTarget float64
	MeanRevertWeight float64
}

func (s *MarketAnomalyStrategy) Name() string { return "market_anomaly" }

func (s *MarketAnomalyStrategy) RequiredSignals() []string { return []string{"price_anomaly"} }

func (s *MarketAnomalyStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.05,"mean_revert_target":0.50,"mean_revert_weight":0.40}`)
}

func (s *MarketAnomalyStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		MinEdgePct       *float64 `json:"min_edge_pct"`
		MeanRevertTarget *float64 `json:"mean_revert_target"`
		MeanRevertWeight *float64 `json:"mean_revert_weight"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.MinEdgePct != nil {
		s.MinEdgePct = *p.MinEdgePct
	}
	if p.MeanRevertTarget != nil {
		s.MeanRevertTarget = *p.MeanRevertTarget
	}
	if p.MeanRevertWeight != nil {
		s.MeanRevertWeight = *p.MeanRevertWeight
	}
	return nil
}

func (s *MarketAnomalyStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
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

	// Parse signal payload.
	var payload struct {
		AnomalyType string  `json:"anomaly_type"`
		YesPrice    float64 `json:"yes_price"`
	}
	if len(sig.Payload) > 0 {
		_ = json.Unmarshal(sig.Payload, &payload)
	}
	if payload.AnomalyType == "" || payload.YesPrice <= 0 {
		return nil, nil
	}

	s.mu.RLock()
	minEdgeRaw := s.MinEdgePct
	meanTarget := s.MeanRevertTarget
	meanWeight := s.MeanRevertWeight
	s.mu.RUnlock()
	if minEdgeRaw <= 0 {
		minEdgeRaw = 0.05
	}
	if meanTarget <= 0 || meanTarget > 1 {
		meanTarget = 0.50
	}
	if meanWeight <= 0 || meanWeight > 1 {
		meanWeight = 0.40
	}

	// Determine side and expected price via mean reversion.
	var side, tokenID string
	pYesExp := (1.0-meanWeight)*payload.YesPrice + meanWeight*meanTarget
	pYesExp = clamp01(pYesExp)

	switch payload.AnomalyType {
	case "extreme_cheap":
		side = "BUY_YES"
		tokenID = yesTokenID
	case "extreme_expensive":
		side = "BUY_NO"
		// Find NO token.
		toks, err := s.Repo.ListTokensByMarketIDs(ctx, []string{marketID})
		if err != nil || len(toks) == 0 {
			return nil, err
		}
		for _, t := range toks {
			if t.MarketID == marketID && strings.EqualFold(strings.TrimSpace(t.Outcome), "no") {
				tokenID = t.ID
				break
			}
		}
		if tokenID == "" {
			return nil, nil
		}
	default:
		return nil, nil
	}

	// Get orderbook for the target token to compute edge.
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

	// Expected payout: for BUY_YES it's pYesExp, for BUY_NO it's 1-pYesExp.
	expPayout := pYesExp
	if side == "BUY_NO" {
		expPayout = 1.0 - pYesExp
	}

	expProfitPerShare := decimal.NewFromFloat(expPayout).Sub(askPrice)
	if expProfitPerShare.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	edgePct := expProfitPerShare.Div(askPrice)
	minEdge := decimal.NewFromFloat(minEdgeRaw)
	if edgePct.LessThan(minEdge) {
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
			"p_yes_now":        payload.YesPrice,
			"p_yes_expected":   pYesExp,
		},
	}
	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal([]string{marketID})
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

	reasoning := fmt.Sprintf("market_anomaly market=%s type=%s side=%s yes_price=%.4f p_yes_expected=%.2f entry=%s",
		marketID, payload.AnomalyType, side, payload.YesPrice, pYesExp, askPrice.StringFixed(4))
	now := time.Now().UTC()

	opp := models.Opportunity{
		Status:          "active",
		EventID:         nil,
		PrimaryMarketID: strPtr(marketID),
		MarketIDs:       datatypes.JSON(marketIDsJSON),
		EdgePct:         edgePct,
		EdgeUSD:         edgeUSD,
		MaxSize:         cost,
		Confidence:      clamp01(sig.Strength),
		RiskScore:       0.90,
		DecayType:       "exponential",
		ExpiresAt:       sig.ExpiresAt,
		Legs:            datatypes.JSON(legsJSON),
		SignalIDs:       datatypes.JSON(signalIDsJSON),
		Reasoning:       reasoning,
		DataAgeMs:       int(time.Since(books[0].UpdatedAt).Milliseconds()),
		Warnings:        datatypes.JSON([]byte(`["price_anomaly"]`)),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return []models.Opportunity{opp}, nil
}
