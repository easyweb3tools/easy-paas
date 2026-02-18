package strategy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"

	"polymarket/internal/models"
)

func mkBook(t *testing.T, tokenID string, ask float64, askSize float64, now time.Time) models.OrderbookLatest {
	t.Helper()
	raw, err := json.Marshal([][]float64{{ask, askSize}})
	if err != nil {
		t.Fatalf("marshal asks: %v", err)
	}
	return models.OrderbookLatest{
		TokenID:   tokenID,
		AsksJSON:  datatypes.JSON(raw),
		BidsJSON:  datatypes.JSON([]byte(`[]`)),
		BestAsk:   &ask,
		BestBid:   nil,
		Mid:       nil,
		UpdatedAt: now,
	}
}

func TestArbSumStrategy_Evaluate_BuyNo(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		marketsByEvent: map[string][]models.Market{
			"e1": {
				{ID: "m1", EventID: "e1", Question: "Q1", LastSeenAt: now},
				{ID: "m2", EventID: "e1", Question: "Q2", LastSeenAt: now},
			},
		},
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
			"m2": {
				{ID: "y2", MarketID: "m2", Outcome: "Yes"},
				{ID: "n2", MarketID: "m2", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			// YES mids used for sum_yes (1.1 => overpriced YES => BUY_NO).
			"y1": func() models.OrderbookLatest { v := 0.55; b := mkBook(t, "y1", 0.55, 100, now); b.Mid = &v; return b }(),
			"y2": func() models.OrderbookLatest { v := 0.55; b := mkBook(t, "y2", 0.55, 100, now); b.Mid = &v; return b }(),
			// NO books used for execution cost.
			"n1": mkBook(t, "n1", 0.45, 100, now),
			"n2": mkBook(t, "n2", 0.45, 100, now),
		},
		tradesByToken: map[string]models.LastTradePrice{},
	}
	for _, toks := range repo.tokensByMarket {
		for _, tok := range toks {
			if repo.tokensByID == nil {
				repo.tokensByID = map[string]models.Token{}
			}
			repo.tokensByID[tok.ID] = tok
		}
	}

	s := &ArbitrageSumStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 1, SignalType: "arb_sum_deviation", Source: "internal_scan", EventID: strPtr("e1"), Strength: 0.9, CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
	if opps[0].EdgePct.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("edge_pct=%s want>0", opps[0].EdgePct.String())
	}
}

func TestSystematicNOStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		booksByToken: map[string]models.OrderbookLatest{
			"n1": mkBook(t, "n1", 0.40, 100, now),
		},
	}
	s := &SystematicNOStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	payload := datatypes.JSON([]byte(`{"label":"safe_no","no_rate":0.95,"no_price":0.40,"ev_pct":25.0}`))
	sig := models.Signal{ID: 2, SignalType: "no_bias", Source: "internal_scan", MarketID: strPtr("m1"), TokenID: strPtr("n1"), Strength: 0.8, Direction: "NO", Payload: payload, CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestPreMarketFDVStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		booksByToken: map[string]models.OrderbookLatest{
			"n1": mkBook(t, "n1", 0.45, 100, now),
		},
	}
	s := &PreMarketFDVStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	payload := datatypes.JSON([]byte(`{"expected_no_rate":0.85,"days_to_end":21}`))
	sig := models.Signal{ID: 3, SignalType: "fdv_overpriced", Source: "internal_scan", MarketID: strPtr("m1"), TokenID: strPtr("n1"), Strength: 0.7, Direction: "NO", Payload: payload, CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestNewsAlphaStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.80, 100, now),
			"n1": mkBook(t, "n1", 0.20, 100, now),
		},
	}
	s := &NewsAlphaStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 4, SignalType: "news_alpha", Source: "price_change", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.9, Direction: "NEUTRAL", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestWeatherStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	city := "new-york"
	repo := &stubRepo{
		labels: []models.MarketLabel{
			{MarketID: "mw1", Label: "weather", SubLabel: &city},
		},
		marketsByID: map[string]models.Market{
			"mw1": {ID: "mw1", EventID: "e1", Question: "Will temperature be above 50 in NYC?", LastSeenAt: now},
		},
		tokensByMarket: map[string][]models.Token{
			"mw1": {
				{ID: "wy", MarketID: "mw1", Outcome: "Yes"},
				{ID: "wn", MarketID: "mw1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"wy": mkBook(t, "wy", 0.50, 100, now),
			"wn": mkBook(t, "wn", 0.55, 100, now),
		},
	}
	s := &WeatherStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	payload := datatypes.JSON([]byte(`{"city":"new-york","forecast_temp_f":60}`))
	sig := models.Signal{ID: 5, SignalType: "weather_deviation", Source: "weather_api", Strength: 0.9, Direction: "NEUTRAL", Payload: payload, CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) == 0 {
		t.Fatalf("expected >=1 opportunity")
	}
}

func TestBTCShortTermStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		labels: []models.MarketLabel{
			{MarketID: "mb1", Label: "btc_15min"},
		},
		marketsByID: map[string]models.Market{
			"mb1": {ID: "mb1", EventID: "e1", Question: "Will Bitcoin be up in 15 minutes?", LastSeenAt: now},
		},
		tokensByMarket: map[string][]models.Token{
			"mb1": {
				{ID: "by", MarketID: "mb1", Outcome: "Yes"},
				{ID: "bn", MarketID: "mb1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"by": mkBook(t, "by", 0.45, 100, now),
			"bn": mkBook(t, "bn", 0.55, 100, now),
		},
	}
	s := &BTCShortTermStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 6, SignalType: "btc_depth_imbalance", Source: "binance_ws", Strength: 0.8, Direction: "YES", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) == 0 {
		t.Fatalf("expected >=1 opportunity")
	}
}

func TestVolatilityArbStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.80, 100, now),
			"n1": mkBook(t, "n1", 0.20, 100, now),
		},
	}
	s := &VolatilityArbStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 7, SignalType: "volatility_spread", Source: "price_change", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.8, Direction: "NEUTRAL", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestContrarianFearStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.80, 100, now),
			"n1": mkBook(t, "n1", 0.20, 100, now),
		},
	}
	s := &ContrarianFearStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 8, SignalType: "fear_spike", Source: "orderbook_pattern", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.9, Direction: "NEUTRAL", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestMMBehaviorStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.80, 100, now),
			"n1": mkBook(t, "n1", 0.20, 100, now),
		},
	}
	s := &MMBehaviorStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 9, SignalType: "mm_inventory_skew", Source: "orderbook_pattern", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.9, Direction: "NEUTRAL", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestCertaintySweepStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.98, 100, now),
		},
	}
	s := &CertaintySweepStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 10, SignalType: "certainty_sweep", Source: "certainty_sweep", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.7, Direction: "YES", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
}

func TestLiquidityRewardStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			"y1": mkBook(t, "y1", 0.40, 100, now),
		},
	}
	s := &LiquidityRewardStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	sig := models.Signal{ID: 11, SignalType: "liquidity_gap", Source: "internal_scan", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.9, Direction: "NEUTRAL", Payload: datatypes.JSON([]byte(`{}`)), CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) == 0 {
		t.Fatalf("expected >=1 opportunity")
	}
}

func TestMarketAnomalyStrategy_Evaluate(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRepo{
		tokensByMarket: map[string][]models.Token{
			"m1": {
				{ID: "y1", MarketID: "m1", Outcome: "Yes"},
				{ID: "n1", MarketID: "m1", Outcome: "No"},
			},
		},
		booksByToken: map[string]models.OrderbookLatest{
			// YES token with extreme cheap price (0.03).
			"y1": mkBook(t, "y1", 0.03, 100, now),
		},
	}
	s := &MarketAnomalyStrategy{Repo: repo}
	_ = s.SetParams(s.DefaultParams())

	payload := datatypes.JSON([]byte(`{"anomaly_type":"extreme_cheap","yes_price":0.03}`))
	sig := models.Signal{ID: 12, SignalType: "price_anomaly", Source: "internal_scan", MarketID: strPtr("m1"), TokenID: strPtr("y1"), Strength: 0.8, Direction: "YES", Payload: payload, CreatedAt: now}
	opps, err := s.Evaluate(context.Background(), []models.Signal{sig})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(opps) != 1 {
		t.Fatalf("opps=%d want=1", len(opps))
	}
	if opps[0].EdgePct.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("edge_pct=%s want>0", opps[0].EdgePct.String())
	}
}
