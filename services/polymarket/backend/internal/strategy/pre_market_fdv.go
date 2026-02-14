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

type PreMarketFDVStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinNoRate  float64
	NoPriceMin float64
	NoPriceMax float64
}

func (s *PreMarketFDVStrategy) Name() string { return "pre_market_fdv" }

func (s *PreMarketFDVStrategy) RequiredSignals() []string { return []string{"fdv_overpriced"} }

func (s *PreMarketFDVStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"entry_window_days_before_tge":[14,28],"no_price_sweet_spot":[0.35,0.55],"min_liquidity_usd":500,"expected_no_rate":0.85,"exit_no_price_take_profit":0.15,"stop_loss_no_price":0.70,"avoid_first_week":true}`)
}

func (s *PreMarketFDVStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		ExpectedNoRate   *float64  `json:"expected_no_rate"`
		NoPriceSweetSpot []float64 `json:"no_price_sweet_spot"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ExpectedNoRate != nil {
		s.MinNoRate = *p.ExpectedNoRate
	}
	if len(p.NoPriceSweetSpot) == 2 {
		s.NoPriceMin = p.NoPriceSweetSpot[0]
		s.NoPriceMax = p.NoPriceSweetSpot[1]
	}
	return nil
}

func (s *PreMarketFDVStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
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

	books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, []string{tokenID})
	if len(books) == 0 {
		return nil, nil
	}
	askPrice, askSize, ok := bestAsk(books[0])
	if !ok || askSize.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	s.mu.RLock()
	noMin := s.NoPriceMin
	noMax := s.NoPriceMax
	s.mu.RUnlock()
	if noMin <= 0 {
		noMin = 0.35
	}
	if noMax <= 0 {
		noMax = 0.55
	}
	askF, _ := askPrice.Float64()
	if askF < noMin || askF > noMax {
		return nil, nil
	}

	// Use a conservative expected NO settlement probability for FDV markets unless overridden by collector payload.
	s.mu.RLock()
	expectedNo := s.MinNoRate
	s.mu.RUnlock()
	if expectedNo <= 0 || expectedNo >= 1 {
		expectedNo = 0.85
	}
	var payload struct {
		ExpectedNoRate float64 `json:"expected_no_rate"`
		DaysToEnd      int     `json:"days_to_end"`
	}
	_ = json.Unmarshal(sig.Payload, &payload)
	if payload.ExpectedNoRate > 0 && payload.ExpectedNoRate < 1 {
		expectedNo = payload.ExpectedNoRate
	}

	expProfitPerShare := decimal.NewFromFloat(expectedNo).Sub(askPrice)
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
		},
	}
	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal([]string{marketID})
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})

	reasoning := fmt.Sprintf("pre_market_fdv market=%s expected_no=%.3f days_to_end=%d entry=%s",
		marketID, expectedNo, payload.DaysToEnd, askPrice.StringFixed(4))
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
		RiskScore:       0.7,
		DecayType:       "time_bound",
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

var _ = polymarketclob.OrderBook{}
