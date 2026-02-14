package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
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

type ArbitrageSumStrategy struct {
	Repo   repository.Repository
	Logger *zap.Logger

	mu sync.RWMutex

	MinDeviationPct float64
	MinProfitUSD    float64
	MinLiquidityUSD float64

	AlphaExtraction   float64
	UseOrderbookDepth bool
	MaxLegs           int
}

type askLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

func (s *ArbitrageSumStrategy) Name() string { return "arb_sum" }

func (s *ArbitrageSumStrategy) RequiredSignals() []string { return []string{"arb_sum_deviation"} }

func (s *ArbitrageSumStrategy) DefaultParams() json.RawMessage {
	return json.RawMessage(`{"min_deviation_pct":2.0,"min_profit_usd":5.0,"min_liquidity_usd":1000,"alpha_extraction":0.9,"use_orderbook_depth":true,"max_legs":10}`)
}

func (s *ArbitrageSumStrategy) SetParams(raw json.RawMessage) error {
	var p struct {
		MinDeviationPct   *float64 `json:"min_deviation_pct"`
		MinProfitUSD      *float64 `json:"min_profit_usd"`
		MinLiquidityUSD   *float64 `json:"min_liquidity_usd"`
		AlphaExtraction   *float64 `json:"alpha_extraction"`
		UseOrderbookDepth *bool    `json:"use_orderbook_depth"`
		MaxLegs           *int     `json:"max_legs"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.MinDeviationPct != nil {
		s.MinDeviationPct = *p.MinDeviationPct
	}
	if p.MinProfitUSD != nil {
		s.MinProfitUSD = *p.MinProfitUSD
	}
	if p.MinLiquidityUSD != nil {
		s.MinLiquidityUSD = *p.MinLiquidityUSD
	}
	if p.AlphaExtraction != nil {
		s.AlphaExtraction = *p.AlphaExtraction
	}
	if p.UseOrderbookDepth != nil {
		s.UseOrderbookDepth = *p.UseOrderbookDepth
	}
	if p.MaxLegs != nil {
		s.MaxLegs = *p.MaxLegs
	}
	return nil
}

func (s *ArbitrageSumStrategy) Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error) {
	if s == nil || s.Repo == nil || len(signals) == 0 {
		return nil, nil
	}
	sig := signals[0]
	if sig.EventID == nil || strings.TrimSpace(*sig.EventID) == "" {
		return nil, nil
	}
	eventID := strings.TrimSpace(*sig.EventID)

	s.mu.RLock()
	minDevPct := s.MinDeviationPct
	minProfit := s.MinProfitUSD
	minLiq := s.MinLiquidityUSD
	alpha := s.AlphaExtraction
	useDepth := s.UseOrderbookDepth
	maxLegs := s.MaxLegs
	s.mu.RUnlock()
	if minDevPct <= 0 {
		minDevPct = 2.0
	}
	if minProfit <= 0 {
		minProfit = 5.0
	}
	_ = minLiq
	if alpha <= 0 || alpha > 1 {
		alpha = 0.9
	}
	if maxLegs <= 0 {
		maxLegs = 10
	}

	markets, err := s.Repo.ListMarketsByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if len(markets) < 2 {
		return nil, nil
	}
	if maxLegs > 0 && len(markets) > maxLegs {
		sort.Slice(markets, func(i, j int) bool {
			li := decimal.Zero
			lj := decimal.Zero
			if markets[i].Liquidity != nil {
				li = *markets[i].Liquidity
			}
			if markets[j].Liquidity != nil {
				lj = *markets[j].Liquidity
			}
			return li.GreaterThan(lj)
		})
		markets = markets[:maxLegs]
	}
	marketIDs := make([]string, 0, len(markets))
	for _, m := range markets {
		if m.ID != "" {
			marketIDs = append(marketIDs, m.ID)
		}
	}

	tokens, err := s.Repo.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil {
		return nil, err
	}
	yesTokenByMarket := map[string]string{}
	noTokenByMarket := map[string]string{}
	for _, tok := range tokens {
		if tok.MarketID == "" || tok.ID == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(tok.Outcome)) {
		case "yes":
			yesTokenByMarket[tok.MarketID] = tok.ID
		case "no":
			noTokenByMarket[tok.MarketID] = tok.ID
		}
	}
	yesTokenIDs := make([]string, 0, len(markets))
	for _, m := range markets {
		if id := yesTokenByMarket[m.ID]; id != "" {
			yesTokenIDs = append(yesTokenIDs, id)
		}
	}
	if len(yesTokenIDs) < 2 {
		return nil, nil
	}
	yesBooks, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, yesTokenIDs)
	yesTrades, _ := s.Repo.ListLastTradePricesByTokenIDs(ctx, yesTokenIDs)
	yesBookByToken := map[string]models.OrderbookLatest{}
	for _, b := range yesBooks {
		yesBookByToken[b.TokenID] = b
	}
	yesTradeByToken := map[string]models.LastTradePrice{}
	for _, tr := range yesTrades {
		yesTradeByToken[tr.TokenID] = tr
	}
	sumYes := 0.0
	for _, tokenID := range yesTokenIDs {
		price, ok := currentPrice(yesBookByToken[tokenID], yesTradeByToken[tokenID])
		if !ok {
			return nil, nil
		}
		sumYes += price
	}
	devPct := math.Abs(sumYes-1.0) * 100.0
	if devPct < minDevPct {
		return nil, nil
	}

	// Determine trade direction from current sum.
	action := "BUY_YES"
	if sumYes > 1.0 {
		action = "BUY_NO"
	}

	// Fetch books for the tokens we intend to buy.
	buyTokenIDs := make([]string, 0, len(markets))
	legs := make([]map[string]any, 0, len(markets))
	for _, m := range markets {
		var tokenID string
		if action == "BUY_YES" {
			tokenID = yesTokenByMarket[m.ID]
		} else {
			tokenID = noTokenByMarket[m.ID]
		}
		if tokenID == "" {
			return nil, nil
		}
		buyTokenIDs = append(buyTokenIDs, tokenID)
		legs = append(legs, map[string]any{
			"token_id":  tokenID,
			"market_id": m.ID,
			"direction": action,
		})
	}

	books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, buyTokenIDs)
	bookByToken := map[string]models.OrderbookLatest{}
	for _, b := range books {
		bookByToken[b.TokenID] = b
	}

	// Compute top-of-book fill.
	costPerShare := decimal.Zero
	profitPerShare := decimal.Zero
	maxShares := decimal.NewFromInt(0) // common size across legs
	hasShares := false
	maxAge := time.Duration(0)
	now := time.Now().UTC()

	asksByToken := map[string][]askLevel{}

	for i := range legs {
		tokenID := legs[i]["token_id"].(string)
		book := bookByToken[tokenID]
		askPrice, askSize, ok := bestAsk(book)
		if !ok {
			return nil, nil
		}
		// Default: best-ask only.
		legs[i]["target_price"] = askPrice.InexactFloat64()
		legs[i]["current_best_ask"] = askPrice.InexactFloat64()
		legs[i]["fillable_size"] = askSize.InexactFloat64()

		if useDepth && len(book.AsksJSON) > 0 {
			var raw []polymarketclob.Order
			if err := json.Unmarshal(book.AsksJSON, &raw); err == nil && len(raw) > 0 {
				lvls := make([]askLevel, 0, len(raw))
				for _, o := range raw {
					if o.Price.LessThanOrEqual(decimal.Zero) || o.Size.LessThanOrEqual(decimal.Zero) {
						continue
					}
					lvls = append(lvls, askLevel{Price: o.Price, Size: o.Size})
				}
				if len(lvls) > 0 {
					asksByToken[tokenID] = lvls
				}
			}
		}

		available := askSize
		if useDepth {
			if lvls, ok := asksByToken[tokenID]; ok && len(lvls) > 0 {
				available = decimal.Zero
				for _, l := range lvls {
					available = available.Add(l.Size)
				}
			}
		}
		if available.GreaterThan(decimal.Zero) {
			if !hasShares {
				maxShares = available
				hasShares = true
			} else if available.LessThan(maxShares) {
				maxShares = available
			}
		}

		if !book.UpdatedAt.IsZero() {
			age := now.Sub(book.UpdatedAt)
			if age > maxAge {
				maxAge = age
			}
		}
	}
	if !hasShares || maxShares.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}

	// If using depth, compute per-leg avg fill prices at the common size.
	if useDepth && len(asksByToken) > 0 {
		for i := range legs {
			tokenID := legs[i]["token_id"].(string)
			lvls, ok := asksByToken[tokenID]
			if !ok || len(lvls) == 0 {
				continue
			}
			avg, worst, ok := avgAskForSize(lvls, maxShares)
			if !ok {
				return nil, nil
			}
			legs[i]["avg_fill_price"] = avg.InexactFloat64()
			legs[i]["worst_fill_price"] = worst.InexactFloat64()
			legs[i]["fillable_size"] = maxShares.InexactFloat64()
		}
		// Recompute costPerShare as sum of avg fill prices.
		costPerShare = decimal.Zero
		for i := range legs {
			if v, ok := legs[i]["avg_fill_price"].(float64); ok && v > 0 {
				costPerShare = costPerShare.Add(decimal.NewFromFloat(v))
			} else if v, ok := legs[i]["target_price"].(float64); ok && v > 0 {
				costPerShare = costPerShare.Add(decimal.NewFromFloat(v))
			}
		}
	} else {
		// Best-ask only cost_per_share.
		costPerShare = decimal.Zero
		for i := range legs {
			if v, ok := legs[i]["target_price"].(float64); ok && v > 0 {
				costPerShare = costPerShare.Add(decimal.NewFromFloat(v))
			}
		}
	}

	n := decimal.NewFromInt(int64(len(legs)))
	if action == "BUY_YES" {
		// Payout 1.0 if exactly one YES resolves.
		profitPerShare = decimal.NewFromInt(1).Sub(costPerShare)
	} else {
		// Payout n-1 if exactly one market resolves YES (so its NO loses).
		profitPerShare = n.Sub(decimal.NewFromInt(1)).Sub(costPerShare)
	}
	if profitPerShare.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	// Capture only a fraction of theoretical alpha to be more conservative.
	profitPerShare = profitPerShare.Mul(decimal.NewFromFloat(alpha))

	maxCostUSD := costPerShare.Mul(maxShares)
	edgeUSD := profitPerShare.Mul(maxShares)
	if edgeUSD.LessThan(decimal.NewFromFloat(minProfit)) {
		return nil, nil
	}
	edgePct := decimal.Zero
	if costPerShare.GreaterThan(decimal.Zero) {
		edgePct = profitPerShare.Div(costPerShare)
	}

	legsJSON, _ := json.Marshal(legs)
	marketIDsJSON, _ := json.Marshal(marketIDs)
	signalIDsJSON, _ := json.Marshal([]uint64{sig.ID})
	reasoning := fmt.Sprintf("arb_sum event=%s sum_yes=%.4f deviation=%.2f%% action=%s cost_per_share=%s profit_per_share=%s",
		eventID, sumYes, devPct, action, costPerShare.StringFixed(4), profitPerShare.StringFixed(4))

	opp := models.Opportunity{
		Status:     "active",
		EventID:    strPtr(eventID),
		MarketIDs:  datatypes.JSON(marketIDsJSON),
		EdgePct:    edgePct,
		EdgeUSD:    edgeUSD,
		MaxSize:    maxCostUSD,
		Confidence: 0.6,
		RiskScore:  0.3,
		DecayType:  "none",
		ExpiresAt:  nil,
		Legs:       datatypes.JSON(legsJSON),
		SignalIDs:  datatypes.JSON(signalIDsJSON),
		Reasoning:  reasoning,
		DataAgeMs:  int(maxAge.Milliseconds()),
		Warnings:   datatypes.JSON([]byte(`[]`)),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return []models.Opportunity{opp}, nil
}

func avgAskForSize(levels []askLevel, size decimal.Decimal) (avg decimal.Decimal, worst decimal.Decimal, ok bool) {
	if size.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, false
	}
	remain := size
	notional := decimal.Zero
	worst = decimal.Zero
	for _, l := range levels {
		if remain.LessThanOrEqual(decimal.Zero) {
			break
		}
		take := l.Size
		if take.GreaterThan(remain) {
			take = remain
		}
		notional = notional.Add(l.Price.Mul(take))
		worst = l.Price
		remain = remain.Sub(take)
	}
	if remain.GreaterThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, false
	}
	avg = notional.Div(size)
	return avg, worst, true
}

func bestAsk(book models.OrderbookLatest) (decimal.Decimal, decimal.Decimal, bool) {
	if book.BestAsk != nil && *book.BestAsk > 0 && len(book.AsksJSON) == 0 {
		return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
	}
	if len(book.AsksJSON) == 0 {
		if book.BestAsk != nil && *book.BestAsk > 0 {
			return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
		}
		return decimal.Zero, decimal.Zero, false
	}
	var asks []polymarketclob.Order
	if err := json.Unmarshal(book.AsksJSON, &asks); err != nil || len(asks) == 0 {
		if book.BestAsk != nil && *book.BestAsk > 0 {
			return decimal.NewFromFloat(*book.BestAsk), decimal.Zero, true
		}
		return decimal.Zero, decimal.Zero, false
	}
	price := asks[0].Price
	size := asks[0].Size
	if price.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, false
	}
	return price, size, true
}

func currentPrice(book models.OrderbookLatest, trade models.LastTradePrice) (float64, bool) {
	if book.Mid != nil && *book.Mid > 0 {
		return *book.Mid, true
	}
	if book.BestBid != nil && book.BestAsk != nil && *book.BestBid > 0 && *book.BestAsk > 0 {
		return (*book.BestBid + *book.BestAsk) / 2.0, true
	}
	if trade.Price > 0 {
		return trade.Price, true
	}
	return 0, false
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}
