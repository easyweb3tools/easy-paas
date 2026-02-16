package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type ReviewService struct {
	Repo   repository.Repository
	Logger *zap.Logger
	Flags  *SystemSettingsService
}

func (s *ReviewService) Run(ctx context.Context, interval time.Duration) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.RunOnce(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("review service run failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (s *ReviewService) RunOnce(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeatureMarketReview, true) {
		return nil
	}
	since := time.Now().UTC().Add(-24 * time.Hour)
	settlements, err := s.Repo.ListRecentMarketSettlementHistory(ctx, since, 1000)
	if err != nil || len(settlements) == 0 {
		return err
	}
	plans, _ := s.Repo.ListExecutionPlans(ctx, repository.ListExecutionPlansParams{
		Limit:   1000,
		Offset:  0,
		OrderBy: "created_at",
		Asc:     boolPtrReview(false),
	})
	planByMarket := map[string]models.ExecutionPlan{}
	for _, p := range plans {
		for _, m := range planMarketIDsFromLegs(p.Legs) {
			if _, ok := planByMarket[m]; !ok {
				planByMarket[m] = p
			}
		}
	}

	for _, st := range settlements {
		marketID := strings.TrimSpace(st.MarketID)
		if marketID == "" {
			continue
		}
		existing, err := s.Repo.GetMarketReviewByMarketID(ctx, marketID)
		if err != nil {
			return err
		}
		action := "missed"
		strategy := ""
		actualPnL := decimal.Zero
		var opportunityID *uint64
		if p, ok := planByMarket[marketID]; ok {
			action = "traded"
			strategy = p.StrategyName
			rec, _ := s.Repo.GetPnLRecordByPlanID(ctx, p.ID)
			if rec != nil && rec.RealizedPnL != nil {
				actualPnL = *rec.RealizedPnL
			}
			opID := p.OpportunityID
			opportunityID = &opID
		}
		finalPrice := st.FinalYesPrice
		if finalPrice == nil {
			if strings.EqualFold(st.Outcome, "YES") {
				v := decimal.NewFromInt(1)
				finalPrice = &v
			} else if strings.EqualFold(st.Outcome, "NO") {
				v := decimal.Zero
				finalPrice = &v
			}
		}
		tagsRaw, _ := json.Marshal([]string{})
		item := &models.MarketReview{
			MarketID:         marketID,
			EventID:          st.EventID,
			OurAction:        action,
			OpportunityID:    opportunityID,
			StrategyName:     strategy,
			FinalOutcome:     strings.ToUpper(strings.TrimSpace(st.Outcome)),
			FinalPrice:       finalPrice,
			HypotheticalPnL:  decimal.Zero,
			ActualPnL:        actualPnL,
			LessonTags:       datatypes.JSON(tagsRaw),
			SettledAt:        st.SettledAt,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		if existing != nil {
			item.ID = existing.ID
		}
		if err := s.Repo.UpsertMarketReview(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

type reviewPlanLeg struct {
	MarketID string `json:"market_id"`
}

func planMarketIDsFromLegs(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var legs []reviewPlanLeg
	if err := json.Unmarshal(raw, &legs); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(legs))
	for _, l := range legs {
		id := strings.TrimSpace(l.MarketID)
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

func boolPtrReview(v bool) *bool { return &v }
