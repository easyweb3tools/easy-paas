package service

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type PositionSyncService struct {
	Repo   repository.Repository
	Logger *zap.Logger
	Flags  *SystemSettingsService
}

func (s *PositionSyncService) SyncFromFill(ctx context.Context, fill models.Fill) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeaturePositionSync, true) {
		return nil
	}
	tokenID := strings.TrimSpace(fill.TokenID)
	if tokenID == "" {
		return nil
	}
	plan, err := s.Repo.GetExecutionPlanByID(ctx, fill.PlanID)
	if err != nil || plan == nil {
		return err
	}
	tokens, err := s.Repo.ListTokensByIDs(ctx, []string{tokenID})
	if err != nil || len(tokens) == 0 {
		return err
	}
	tok := tokens[0]

	opp, _ := s.Repo.GetOpportunityByID(ctx, plan.OpportunityID)
	eventID := ""
	if opp != nil && opp.EventID != nil {
		eventID = strings.TrimSpace(*opp.EventID)
	}

	direction := normalizePositionDirection(fill.Direction)
	if direction == "" {
		direction = "YES"
	}
	sideSign := fillSideSign(fill.Direction)
	if sideSign == 0 {
		sideSign = 1
	}

	pos, err := s.Repo.GetPositionByTokenID(ctx, tokenID)
	if err != nil {
		return err
	}
	if pos == nil {
		pos = &models.Position{
			TokenID:       tokenID,
			MarketID:      strings.TrimSpace(tok.MarketID),
			EventID:       eventID,
			Direction:     direction,
			Quantity:      decimal.Zero,
			AvgEntryPrice: decimal.Zero,
			CurrentPrice:  fill.AvgPrice,
			CostBasis:     decimal.Zero,
			UnrealizedPnL: decimal.Zero,
			RealizedPnL:   decimal.Zero,
			Status:        "open",
			StrategyName:  plan.StrategyName,
			OpenedAt:      fill.FilledAt,
			CreatedAt:     time.Now().UTC(),
		}
	}

	oldQty := pos.Quantity
	oldAvg := pos.AvgEntryPrice
	qtyDelta := fill.FilledSize.Mul(decimal.NewFromInt(int64(sideSign)))
	newQty := oldQty.Add(qtyDelta)

	if sideSign > 0 {
		totalCost := pos.CostBasis.Add(fill.AvgPrice.Mul(fill.FilledSize)).Add(fill.Fee)
		if newQty.GreaterThan(decimal.Zero) {
			pos.AvgEntryPrice = totalCost.Div(newQty)
		}
		pos.CostBasis = totalCost
	} else {
		sellQty := fill.FilledSize
		if sellQty.GreaterThan(oldQty) {
			sellQty = oldQty
		}
		realizedDelta := fill.AvgPrice.Sub(oldAvg).Mul(sellQty).Sub(fill.Fee)
		pos.RealizedPnL = pos.RealizedPnL.Add(realizedDelta)
		if newQty.GreaterThan(decimal.Zero) {
			pos.CostBasis = oldAvg.Mul(newQty)
			pos.AvgEntryPrice = oldAvg
		} else {
			pos.CostBasis = decimal.Zero
			pos.AvgEntryPrice = decimal.Zero
		}
	}

	if newQty.LessThanOrEqual(decimal.Zero) {
		pos.Quantity = decimal.Zero
		pos.Status = "closed"
		now := time.Now().UTC()
		pos.ClosedAt = &now
		pos.UnrealizedPnL = decimal.Zero
	} else {
		pos.Quantity = newQty
		pos.Status = "open"
		pos.ClosedAt = nil
		pos.UnrealizedPnL = pos.CurrentPrice.Sub(pos.AvgEntryPrice).Mul(pos.Quantity)
	}
	pos.StrategyName = plan.StrategyName
	pos.UpdatedAt = time.Now().UTC()

	if err := s.Repo.UpsertPosition(ctx, pos); err != nil {
		return err
	}
	return nil
}

func (s *PositionSyncService) RefreshOpenPositionsPrices(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeaturePositionSync, true) {
		return nil
	}
	items, err := s.Repo.ListOpenPositions(ctx)
	if err != nil || len(items) == 0 {
		return err
	}
	tokenIDs := make([]string, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.TokenID) != "" {
			tokenIDs = append(tokenIDs, strings.TrimSpace(it.TokenID))
		}
	}
	books, err := s.Repo.ListOrderbookLatestByTokenIDs(ctx, tokenIDs)
	if err != nil {
		return err
	}
	bookByToken := map[string]models.OrderbookLatest{}
	for _, b := range books {
		bookByToken[b.TokenID] = b
	}

	for i := range items {
		pos := items[i]
		book := bookByToken[pos.TokenID]
		if book.Mid != nil && *book.Mid > 0 {
			pos.CurrentPrice = decimal.NewFromFloat(*book.Mid)
		} else if book.BestAsk != nil && *book.BestAsk > 0 {
			pos.CurrentPrice = decimal.NewFromFloat(*book.BestAsk)
		}
		pos.UnrealizedPnL = pos.CurrentPrice.Sub(pos.AvgEntryPrice).Mul(pos.Quantity)
		pos.UpdatedAt = time.Now().UTC()
		if err := s.Repo.UpsertPosition(ctx, &pos); err != nil {
			return err
		}
	}
	return nil
}

func (s *PositionSyncService) SnapshotPortfolio(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeaturePortfolioSnapshot, true) {
		return nil
	}
	sum, err := s.Repo.PositionsSummary(ctx)
	if err != nil {
		return err
	}
	item := &models.PortfolioSnapshot{
		SnapshotAt:     time.Now().UTC().Truncate(time.Hour),
		TotalPositions: int(sum.TotalOpen),
		TotalCostBasis: decimal.NewFromFloat(sum.TotalCostBasis),
		TotalMarketVal: decimal.NewFromFloat(sum.TotalMarketVal),
		UnrealizedPnL:  decimal.NewFromFloat(sum.UnrealizedPnL),
		RealizedPnL:    decimal.NewFromFloat(sum.RealizedPnL),
		NetLiquidation: decimal.NewFromFloat(sum.NetLiquidation),
		CreatedAt:      time.Now().UTC(),
	}
	return s.Repo.InsertPortfolioSnapshot(ctx, item)
}

func fillSideSign(fillDirection string) int {
	dir := strings.ToUpper(strings.TrimSpace(fillDirection))
	switch dir {
	case "BUY_YES", "BUY_NO", "BUY":
		return 1
	case "SELL_YES", "SELL_NO", "SELL":
		return -1
	default:
		return 0
	}
}

func normalizePositionDirection(fillDirection string) string {
	dir := strings.ToUpper(strings.TrimSpace(fillDirection))
	switch {
	case strings.HasSuffix(dir, "_YES"):
		return "YES"
	case strings.HasSuffix(dir, "_NO"):
		return "NO"
	}
	return ""
}
