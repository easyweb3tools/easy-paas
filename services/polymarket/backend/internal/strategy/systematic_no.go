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

	polymarketclob "polymarket/internal/client/polymarket/clob"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type SystematicNOStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinEVPct        float64
	NoPriceMin      float64
	NoPriceMax      float64
	StopLossNoPrice float64
}

func (s *SystematicNOStrategy) Name() string { return "systematic_no" }

func (s *SystematicNOStrategy) RequiredSignals() []string { return []string{"no_bias"} }

func (s *SystematicNOStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"no_price_range":[0.10,0.70],"min_ev_pct":10.0,"historical_no_rate":0.806,"category_no_rates":{},"stop_loss_no_price":0.80}`)
}

func (s *SystematicNOStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		MinEVPct        *float64  `json:"min_ev_pct"`
		NoPriceRange    []float64 `json:"no_price_range"`
		StopLossNoPrice *float64  `json:"stop_loss_no_price"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.MinEVPct != nil {
		s.MinEVPct = *p.MinEVPct
	}
	if len(p.NoPriceRange) == 2 {
		s.NoPriceMin = p.NoPriceRange[0]
		s.NoPriceMax = p.NoPriceRange[1]
	}
	if p.StopLossNoPrice != nil {
		s.StopLossNoPrice = *p.StopLossNoPrice
	}
	return nil
}

func (s *SystematicNOStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
	if s == nil || s.Repo == nil || len(signals) == 0 {
		return nil, nil
	}
	sig := signals[0]
	if sig.MarketID == nil || sig.TokenID == nil {
		return nil, nil
	}
	marketID := strings.TrimSpace(*sig.MarketID)
	tokenID := strings.TrimSpace(*sig.TokenID)
	if marketID == "" || tokenID == "" {
		return nil, nil
	}

	s.mu.RLock()
	minEV := s.MinEVPct
	priceMin := s.NoPriceMin
	priceMax := s.NoPriceMax
	stopLoss := s.StopLossNoPrice
	s.mu.RUnlock()
	if minEV <= 0 {
		minEV = 10.0
	}
	if priceMin <= 0 {
		priceMin = 0.10
	}
	if priceMax <= 0 {
		priceMax = 0.70
	}
	if stopLoss <= 0 {
		stopLoss = 0.80
	}

	var payload struct {
		Label   string  `json:"label"`
		NoRate  float64 `json:"no_rate"`
		NoPrice float64 `json:"no_price"`
		EVPct   float64 `json:"ev_pct"`
	}
	_ = json.Unmarshal(sig.Payload, &payload)
	if payload.EVPct < minEV {
		return nil, nil
	}

	books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{tokenID})
	if len(books) == 0 {
		return nil, nil
	}
	askPrice, askSize, ok := bestAsk(books[0])
	if !ok {
		return nil, nil
	}
	if askSize.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	askF, _ := askPrice.Float64()
	if askF < priceMin || askF > priceMax {
		return nil, nil
	}
	if stopLoss > 0 && askF > stopLoss {
		return nil, nil
	}

	// Expected profit per share using no_rate - entry_price.
	expProfitPerShare := decimal.NewFromFloat(payload.NoRate).Sub(askPrice)
	if expProfitPerShare.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	cost := askPrice.Mul(askSize)
	edgeUSD := expProfitPerShare.Mul(askSize)
	edgePct := expProfitPerShare.Div(askPrice)

	legs := []map[string]any{
		{
			"token_id":         tokenID,
			"market_id":        marketID,
			"direction":        "BUY_NO",
			"target_price":     askPrice.InexactFloat64(),
			"current_best_ask": askPrice.InexactFloat64(),
			"fillable_size":    askSize.InexactFloat64(),
			"label":            payload.Label,
			"expected_no_rate": payload.NoRate,
		},
	}
	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal([]string{marketID})
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

	reasoning := fmt.Sprintf("systematic_no market=%s label=%s ev_pct=%.2f%% entry=%s expected_no_rate=%.3f",
		marketID, payload.Label, payload.EVPct, askPrice.StringFixed(4), payload.NoRate)
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
		RiskScore:       0.6,
		DecayType:       "none",
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

// Use clob OrderBook parser compatibility.
var _ = polymarketclob.OrderBook{}
