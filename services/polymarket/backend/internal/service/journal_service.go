package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type JournalService struct {
	Repo repository.Repository
}

func (s *JournalService) CaptureEntry(ctx context.Context, planID uint64) error {
	if s == nil || s.Repo == nil || planID == 0 {
		return nil
	}
	existing, err := s.Repo.GetTradeJournalByPlanID(ctx, planID)
	if err != nil || existing != nil {
		return err
	}

	plan, err := s.Repo.GetExecutionPlanByID(ctx, planID)
	if err != nil || plan == nil {
		return err
	}
	opp, err := s.Repo.GetOpportunityByID(ctx, plan.OpportunityID)
	if err != nil || opp == nil {
		return err
	}

	ids := parseSignalIDs(opp.SignalIDs)
	signalSnapshot := map[string]any{"signal_ids": ids}
	if len(ids) > 0 {
		items, _ := s.Repo.ListSignals(ctx, repository.ListSignalsParams{
			Limit:   500,
			Offset:  0,
			OrderBy: "created_at",
			Asc:     boolPtrJournal(false),
		})
		if len(items) > 0 {
			set := map[uint64]struct{}{}
			for _, id := range ids {
				set[id] = struct{}{}
			}
			selected := make([]models.Signal, 0, len(ids))
			for _, it := range items {
				if _, ok := set[it.ID]; ok {
					selected = append(selected, it)
				}
			}
			if len(selected) > 0 {
				signalSnapshot["signals"] = selected
			}
		}
	}

	tokenIDs := planTokenIDs(plan.Legs)
	marketSnapshot := map[string]any{}
	if len(tokenIDs) > 0 {
		books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, tokenIDs)
		marketSnapshot["orderbooks"] = books
		trades, _ := s.Repo.ListLastTradePricesByTokenIDs(ctx, tokenIDs)
		marketSnapshot["last_trade_prices"] = trades
	}

	signalRaw, _ := json.Marshal(signalSnapshot)
	marketRaw, _ := json.Marshal(marketSnapshot)
	tagsRaw, _ := json.Marshal([]string{})
	item := &models.TradeJournal{
		ExecutionPlanID: plan.ID,
		OpportunityID:   plan.OpportunityID,
		StrategyName:    plan.StrategyName,
		EntryReasoning:  opp.Reasoning,
		SignalSnapshot:  datatypes.JSON(signalRaw),
		MarketSnapshot:  datatypes.JSON(marketRaw),
		EntryParams:     plan.Params,
		Outcome:         "pending",
		Tags:            datatypes.JSON(tagsRaw),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return s.Repo.InsertTradeJournal(ctx, item)
}

func (s *JournalService) CaptureExit(ctx context.Context, planID uint64) error {
	if s == nil || s.Repo == nil || planID == 0 {
		return nil
	}
	journal, err := s.Repo.GetTradeJournalByPlanID(ctx, planID)
	if err != nil || journal == nil {
		return err
	}
	rec, err := s.Repo.GetPnLRecordByPlanID(ctx, planID)
	if err != nil || rec == nil {
		return err
	}
	plan, err := s.Repo.GetExecutionPlanByID(ctx, planID)
	if err != nil || plan == nil {
		return err
	}

	outcomeSnapshot := map[string]any{}
	tokenIDs := planTokenIDs(plan.Legs)
	if len(tokenIDs) > 0 {
		books, _ := s.Repo.ListOrderbookLatestByTokenIDs(ctx, tokenIDs)
		outcomeSnapshot["orderbooks"] = books
		trades, _ := s.Repo.ListLastTradePricesByTokenIDs(ctx, tokenIDs)
		outcomeSnapshot["last_trade_prices"] = trades
	}
	outcomeRaw, _ := json.Marshal(outcomeSnapshot)

	exitReasoning := fmt.Sprintf("settled outcome=%s", strings.TrimSpace(rec.Outcome))
	if rec.RealizedPnL != nil {
		exitReasoning = fmt.Sprintf("%s pnl_usd=%s", exitReasoning, rec.RealizedPnL.String())
	}
	updates := map[string]any{
		"outcome":          strings.TrimSpace(rec.Outcome),
		"outcome_snapshot": datatypes.JSON(outcomeRaw),
		"exit_reasoning":   exitReasoning,
	}
	if rec.RealizedPnL != nil {
		updates["pnl_usd"] = rec.RealizedPnL
	}
	if rec.RealizedROI != nil {
		updates["roi"] = rec.RealizedROI
	}
	return s.Repo.UpdateTradeJournalExit(ctx, planID, updates)
}

func parseSignalIDs(raw []byte) []uint64 {
	if len(raw) == 0 {
		return nil
	}
	var out []uint64
	_ = json.Unmarshal(raw, &out)
	return out
}

type journalLeg struct {
	TokenID string `json:"token_id"`
}

func planTokenIDs(legsJSON []byte) []string {
	if len(legsJSON) == 0 {
		return nil
	}
	var legs []journalLeg
	if err := json.Unmarshal(legsJSON, &legs); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(legs))
	for _, leg := range legs {
		id := strings.TrimSpace(leg.TokenID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func boolPtrJournal(v bool) *bool { return &v }
