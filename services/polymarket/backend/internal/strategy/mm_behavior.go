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

// MMBehaviorStrategy (P2) consumes "mm_inventory_skew" signals.
// MVP: uses the same contrarian extreme pricing heuristic to surface opportunities around wide spreads.
type MMBehaviorStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEdgePct       float64
	YesExtremeMin    float64
	YesExtremeMax    float64
	MeanRevertWeight float64
}

func (s *MMBehaviorStrategy) Name() string { return "mm_behavior" }

func (s *MMBehaviorStrategy) RequiredSignals() []string { return []string{"mm_inventory_skew"} }

func (s *MMBehaviorStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_edge_pct":0.05,"yes_extreme_min":0.75,"yes_extreme_max":0.25,"mean_revert_weight":0.5}`)
}

func (s *MMBehaviorStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		MinEdgePct       *float64 `json:"min_edge_pct"`
		YesExtremeMin    *float64 `json:"yes_extreme_min"`
		YesExtremeMax    *float64 `json:"yes_extreme_max"`
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
	if p.YesExtremeMin != nil {
		s.YesExtremeMin = *p.YesExtremeMin
	}
	if p.YesExtremeMax != nil {
		s.YesExtremeMax = *p.YesExtremeMax
	}
	if p.MeanRevertWeight != nil {
		s.MeanRevertWeight = *p.MeanRevertWeight
	}
	return nil
}

func (s *MMBehaviorStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
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

	yesBooks, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{yesTokenID})
	if len(yesBooks) == 0 {
		return nil, nil
	}
	yesAsk, _, ok := bestAsk(yesBooks[0])
	if !ok || yesAsk.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}

	s.mu.RLock()
	minEdgeRaw := s.MinEdgePct
	yesExtremeMin := s.YesExtremeMin
	yesExtremeMax := s.YesExtremeMax
	meanRevertWeight := s.MeanRevertWeight
	s.mu.RUnlock()
	if minEdgeRaw <= 0 {
		minEdgeRaw = 0.05
	}
	if yesExtremeMin <= 0 {
		yesExtremeMin = 0.75
	}
	if yesExtremeMax <= 0 {
		yesExtremeMax = 0.25
	}
	if meanRevertWeight <= 0 || meanRevertWeight > 1 {
		meanRevertWeight = 0.5
	}

	side := ""
	if yesAsk.GreaterThanOrEqual(decimal.NewFromFloat(yesExtremeMin)) {
		side = "BUY_NO"
	} else if yesAsk.LessThanOrEqual(decimal.NewFromFloat(yesExtremeMax)) {
		side = "BUY_YES"
	} else {
		return nil, nil
	}

	pYesNow, _ := yesAsk.Float64()
	pYesExp := (1.0-meanRevertWeight)*pYesNow + meanRevertWeight*0.5
	pYesExp = clamp01(pYesExp)

	tokenID := yesTokenID
	expPayout := pYesExp
	if side == "BUY_NO" {
		tokenID = noTokenID
		expPayout = 1.0 - pYesExp
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
			"p_yes_now":        pYesNow,
			"p_yes_expected":   pYesExp,
		},
	}
	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal([]string{marketID})
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

	reasoning := fmt.Sprintf("mm_behavior market=%s side=%s yes_ask=%s p_yes_expected=%.2f entry=%s",
		marketID, side, yesAsk.StringFixed(4), pYesExp, askPrice.StringFixed(4))
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
		RiskScore:       0.88,
		DecayType:       "step",
		ExpiresAt:       sig.ExpiresAt,
		Legs:            datatypes.JSON(legsJSON),
		SignalIDs:       datatypes.JSON(signalIDsJSON),
		Reasoning:       reasoning,
		DataAgeMs:       int(time.Since(books[0].UpdatedAt).Milliseconds()),
		Warnings:        datatypes.JSON([]byte(`["wide_spread"]`)),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return []models.Opportunity{opp}, nil
}
