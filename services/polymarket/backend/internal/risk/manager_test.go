package risk

import (
	"testing"

	"github.com/shopspring/decimal"

	"polymarket/internal/config"
)

func TestLimitPlannedSize_TotalExposureCap(t *testing.T) {
	cfg := config.RiskConfig{
		MaxTotalExposureUSD: 100,
	}
	exp := exposureSnapshot{
		Total:      decimal.NewFromInt(90),
		ByStrategy: map[string]decimal.Decimal{},
		ByMarket:   map[string]decimal.Decimal{},
	}
	planned, warnings := limitPlannedSize(cfg, exp, "", nil, decimal.NewFromInt(50))
	if planned.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("planned=%s want=10", planned.String())
	}
	if len(warnings) == 0 || warnings[0] != "total_exposure_cap" {
		t.Fatalf("warnings=%v want contains total_exposure_cap", warnings)
	}
}

func TestLimitPlannedSize_StrategyCap(t *testing.T) {
	cfg := config.RiskConfig{
		MaxPerStrategyUSD: 200,
	}
	exp := exposureSnapshot{
		Total:      decimal.Zero,
		ByStrategy: map[string]decimal.Decimal{"arb_sum": decimal.NewFromInt(180)},
		ByMarket:   map[string]decimal.Decimal{},
	}
	planned, warnings := limitPlannedSize(cfg, exp, "arb_sum", nil, decimal.NewFromInt(100))
	if planned.Cmp(decimal.NewFromInt(20)) != 0 {
		t.Fatalf("planned=%s want=20", planned.String())
	}
	found := false
	for _, w := range warnings {
		if w == "strategy_exposure_cap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings=%v want contains strategy_exposure_cap", warnings)
	}
}

func TestLimitPlannedSize_MarketCap_MultiMarket(t *testing.T) {
	cfg := config.RiskConfig{
		MaxPerMarketUSD: 100,
	}
	exp := exposureSnapshot{
		Total:      decimal.Zero,
		ByStrategy: map[string]decimal.Decimal{},
		ByMarket: map[string]decimal.Decimal{
			"m1": decimal.NewFromInt(80),
			"m2": decimal.NewFromInt(10),
		},
	}
	// With 2 markets, we assume equal split:
	// Remaining for m1 = 20 => total planned <= 40.
	planned, warnings := limitPlannedSize(cfg, exp, "", []string{"m1", "m2"}, decimal.NewFromInt(60))
	if planned.Cmp(decimal.NewFromInt(40)) != 0 {
		t.Fatalf("planned=%s want=40", planned.String())
	}
	found := false
	for _, w := range warnings {
		if w == "market_exposure_cap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings=%v want contains market_exposure_cap", warnings)
	}
}
