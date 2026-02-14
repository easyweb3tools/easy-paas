package strategy

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// stubRepo is a test-only in-memory implementation of repository.Repository.
// It implements the full interface but only a small subset is used by strategy evaluator tests.
type stubRepo struct {
	marketsByEvent map[string][]models.Market
	marketsByID    map[string]models.Market
	tokensByMarket map[string][]models.Token
	tokensByID     map[string]models.Token
	booksByToken   map[string]models.OrderbookLatest
	tradesByToken  map[string]models.LastTradePrice
	labels         []models.MarketLabel
}

func (s *stubRepo) InTx(ctx context.Context, fn func(tx *gorm.DB) error) error { return fn(nil) }
func (s *stubRepo) UpsertEventsTx(ctx context.Context, tx *gorm.DB, items []models.Event) error {
	return nil
}
func (s *stubRepo) UpsertMarketsTx(ctx context.Context, tx *gorm.DB, items []models.Market) error {
	return nil
}
func (s *stubRepo) UpsertTokensTx(ctx context.Context, tx *gorm.DB, items []models.Token) error {
	return nil
}
func (s *stubRepo) UpsertSeriesTx(ctx context.Context, tx *gorm.DB, items []models.Series) error {
	return nil
}
func (s *stubRepo) UpsertTagsTx(ctx context.Context, tx *gorm.DB, items []models.Tag) error {
	return nil
}
func (s *stubRepo) UpsertEventTagsTx(ctx context.Context, tx *gorm.DB, items []models.EventTag) error {
	return nil
}
func (s *stubRepo) UpsertOrderbookLatest(ctx context.Context, item *models.OrderbookLatest) error {
	return nil
}
func (s *stubRepo) UpsertMarketDataHealth(ctx context.Context, item *models.MarketDataHealth) error {
	return nil
}
func (s *stubRepo) UpsertLastTradePrice(ctx context.Context, item *models.LastTradePrice) error {
	return nil
}
func (s *stubRepo) InsertRawWSEvent(ctx context.Context, item *models.RawWSEvent) error { return nil }
func (s *stubRepo) InsertRawRESTSnapshot(ctx context.Context, item *models.RawRESTSnapshot) error {
	return nil
}
func (s *stubRepo) FindMarketsByConditionIDs(ctx context.Context, conditionIDs []string) ([]models.Market, error) {
	return nil, nil
}
func (s *stubRepo) FindMarketsBySlugs(ctx context.Context, slugs []string) ([]models.Market, error) {
	return nil, nil
}
func (s *stubRepo) GetMarketBySlug(ctx context.Context, slug string) (*models.Market, error) {
	return nil, nil
}
func (s *stubRepo) GetEventBySlug(ctx context.Context, slug string) (*models.Event, error) {
	return nil, nil
}
func (s *stubRepo) ListMarketsByEventID(ctx context.Context, eventID string) ([]models.Market, error) {
	if s.marketsByEvent == nil {
		return nil, nil
	}
	return s.marketsByEvent[eventID], nil
}
func (s *stubRepo) ListMarketsByEventIDs(ctx context.Context, eventIDs []string) ([]models.Market, error) {
	var out []models.Market
	for _, id := range eventIDs {
		out = append(out, s.marketsByEvent[id]...)
	}
	return out, nil
}
func (s *stubRepo) ListMarketsByIDs(ctx context.Context, marketIDs []string) ([]models.Market, error) {
	out := make([]models.Market, 0, len(marketIDs))
	for _, id := range marketIDs {
		if m, ok := s.marketsByID[id]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *stubRepo) ListMarketIDsForStream(ctx context.Context, limit int) ([]string, error) {
	return nil, nil
}
func (s *stubRepo) ListTokensByMarketIDs(ctx context.Context, marketIDs []string) ([]models.Token, error) {
	out := make([]models.Token, 0, 2*len(marketIDs))
	for _, mid := range marketIDs {
		out = append(out, s.tokensByMarket[mid]...)
	}
	return out, nil
}
func (s *stubRepo) ListTokensByIDs(ctx context.Context, tokenIDs []string) ([]models.Token, error) {
	out := make([]models.Token, 0, len(tokenIDs))
	for _, id := range tokenIDs {
		if t, ok := s.tokensByID[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}
func (s *stubRepo) ListMarketDataHealthByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.MarketDataHealth, error) {
	return nil, nil
}
func (s *stubRepo) ListOrderbookLatestByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.OrderbookLatest, error) {
	out := make([]models.OrderbookLatest, 0, len(tokenIDs))
	for _, id := range tokenIDs {
		if b, ok := s.booksByToken[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}
func (s *stubRepo) ListLastTradePricesByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.LastTradePrice, error) {
	out := make([]models.LastTradePrice, 0, len(tokenIDs))
	for _, id := range tokenIDs {
		if tr, ok := s.tradesByToken[id]; ok {
			out = append(out, tr)
		}
	}
	return out, nil
}
func (s *stubRepo) ListMarketAggregates(ctx context.Context, limit int) ([]repository.EventAggregate, error) {
	return nil, nil
}
func (s *stubRepo) ListEventsByIDs(ctx context.Context, ids []string) ([]models.Event, error) {
	return nil, nil
}
func (s *stubRepo) ListEvents(ctx context.Context, params repository.ListEventsParams) ([]models.Event, error) {
	return nil, nil
}
func (s *stubRepo) CountEvents(ctx context.Context, params repository.ListEventsParams) (int64, error) {
	return 0, nil
}
func (s *stubRepo) ListMarkets(ctx context.Context, params repository.ListMarketsParams) ([]models.Market, error) {
	return nil, nil
}
func (s *stubRepo) CountMarkets(ctx context.Context, params repository.ListMarketsParams) (int64, error) {
	return 0, nil
}
func (s *stubRepo) ListTokens(ctx context.Context, params repository.ListTokensParams) ([]models.Token, error) {
	return nil, nil
}
func (s *stubRepo) CountTokens(ctx context.Context, params repository.ListTokensParams) (int64, error) {
	return 0, nil
}
func (s *stubRepo) GetSyncState(ctx context.Context, scope string) (*models.SyncState, error) {
	return nil, nil
}
func (s *stubRepo) SaveSyncStateTx(ctx context.Context, tx *gorm.DB, state *models.SyncState) error {
	return nil
}
func (s *stubRepo) ListSyncStates(ctx context.Context) ([]models.SyncState, error) { return nil, nil }
func (s *stubRepo) ListActiveEventsEndingSoon(ctx context.Context, hoursToExpiry int, limit int) ([]models.Event, error) {
	return nil, nil
}

func (s *stubRepo) InsertSignal(ctx context.Context, item *models.Signal) error { return nil }
func (s *stubRepo) ListSignals(ctx context.Context, params repository.ListSignalsParams) ([]models.Signal, error) {
	return nil, nil
}
func (s *stubRepo) DeleteExpiredSignals(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}
func (s *stubRepo) UpsertSignalSource(ctx context.Context, item *models.SignalSource) error {
	return nil
}
func (s *stubRepo) ListSignalSources(ctx context.Context) ([]models.SignalSource, error) {
	return nil, nil
}
func (s *stubRepo) ListMarketDataHealthCandidates(ctx context.Context, limit int, minSpreadBps float64) ([]models.MarketDataHealth, error) {
	return nil, nil
}
func (s *stubRepo) ListYesTokenJumpCandidates(ctx context.Context, limit int, minJumpBps float64, maxSpreadBps float64) ([]repository.TokenJumpCandidate, error) {
	return nil, nil
}
func (s *stubRepo) ListTagsByEventIDs(ctx context.Context, eventIDs []string) (map[string][]models.Tag, error) {
	return map[string][]models.Tag{}, nil
}

func (s *stubRepo) UpsertStrategy(ctx context.Context, item *models.Strategy) error { return nil }
func (s *stubRepo) GetStrategyByName(ctx context.Context, name string) (*models.Strategy, error) {
	return nil, nil
}
func (s *stubRepo) ListStrategies(ctx context.Context) ([]models.Strategy, error) { return nil, nil }
func (s *stubRepo) SetStrategyEnabled(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (s *stubRepo) UpdateStrategyParams(ctx context.Context, name string, params []byte) error {
	return nil
}
func (s *stubRepo) UpdateStrategyStats(ctx context.Context, name string, stats []byte) error {
	return nil
}

func (s *stubRepo) InsertOpportunity(ctx context.Context, item *models.Opportunity) error { return nil }
func (s *stubRepo) UpsertActiveOpportunity(ctx context.Context, item *models.Opportunity) error {
	return nil
}
func (s *stubRepo) GetOpportunityByID(ctx context.Context, id uint64) (*models.Opportunity, error) {
	return nil, nil
}
func (s *stubRepo) ListOpportunities(ctx context.Context, params repository.ListOpportunitiesParams) ([]models.Opportunity, error) {
	return nil, nil
}
func (s *stubRepo) CountOpportunities(ctx context.Context, params repository.ListOpportunitiesParams) (int64, error) {
	return 0, nil
}
func (s *stubRepo) UpdateOpportunityStatus(ctx context.Context, id uint64, status string) error {
	return nil
}
func (s *stubRepo) ExpireDueOpportunities(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}
func (s *stubRepo) CountActiveOpportunities(ctx context.Context) (int64, error) { return 0, nil }
func (s *stubRepo) ListOldestActiveOpportunityIDs(ctx context.Context, limit int) ([]uint64, error) {
	return nil, nil
}
func (s *stubRepo) BulkUpdateOpportunityStatus(ctx context.Context, ids []uint64, status string) (int64, error) {
	return 0, nil
}

func (s *stubRepo) UpsertMarketLabel(ctx context.Context, item *models.MarketLabel) error { return nil }
func (s *stubRepo) ListMarketLabels(ctx context.Context, params repository.ListMarketLabelsParams) ([]models.MarketLabel, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 200
	}
	out := make([]models.MarketLabel, 0, limit)
	for _, l := range s.labels {
		if params.MarketID != nil && *params.MarketID != "" && l.MarketID != *params.MarketID {
			continue
		}
		if params.Label != nil && *params.Label != "" && l.Label != *params.Label {
			continue
		}
		if params.SubLabel != nil && *params.SubLabel != "" {
			if l.SubLabel == nil || *l.SubLabel != *params.SubLabel {
				continue
			}
		}
		out = append(out, l)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
func (s *stubRepo) DeleteMarketLabel(ctx context.Context, marketID string, label string) error {
	return nil
}

func (s *stubRepo) InsertExecutionPlan(ctx context.Context, item *models.ExecutionPlan) error {
	return nil
}
func (s *stubRepo) GetExecutionPlanByID(ctx context.Context, id uint64) (*models.ExecutionPlan, error) {
	return nil, nil
}
func (s *stubRepo) ListExecutionPlans(ctx context.Context, params repository.ListExecutionPlansParams) ([]models.ExecutionPlan, error) {
	return nil, nil
}
func (s *stubRepo) CountExecutionPlans(ctx context.Context, params repository.ListExecutionPlansParams) (int64, error) {
	return 0, nil
}
func (s *stubRepo) ListExecutionPlansByStatuses(ctx context.Context, statuses []string, limit int) ([]models.ExecutionPlan, error) {
	return nil, nil
}
func (s *stubRepo) UpdateExecutionPlanStatus(ctx context.Context, id uint64, status string) error {
	return nil
}
func (s *stubRepo) UpdateExecutionPlanPreflight(ctx context.Context, id uint64, status string, preflightResult []byte) error {
	return nil
}
func (s *stubRepo) UpdateExecutionPlanExecutedAt(ctx context.Context, id uint64, status string, executedAt *time.Time) error {
	return nil
}
func (s *stubRepo) InsertFill(ctx context.Context, item *models.Fill) error { return nil }
func (s *stubRepo) ListFillsByPlanID(ctx context.Context, planID uint64) ([]models.Fill, error) {
	return nil, nil
}
func (s *stubRepo) UpsertPnLRecord(ctx context.Context, item *models.PnLRecord) error { return nil }
func (s *stubRepo) GetPnLRecordByPlanID(ctx context.Context, planID uint64) (*models.PnLRecord, error) {
	return nil, nil
}
func (s *stubRepo) SumRealizedPnLSince(ctx context.Context, since time.Time) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (s *stubRepo) UpsertMarketSettlementHistory(ctx context.Context, item *models.MarketSettlementHistory) error {
	return nil
}
func (s *stubRepo) ListMarketSettlementHistoryByMarketIDs(ctx context.Context, marketIDs []string) ([]models.MarketSettlementHistory, error) {
	return nil, nil
}
func (s *stubRepo) ListLabelNoRateStats(ctx context.Context, labels []string) ([]repository.LabelNoRateRow, error) {
	return nil, nil
}

func (s *stubRepo) AnalyticsOverview(ctx context.Context) (repository.AnalyticsOverview, error) {
	return repository.AnalyticsOverview{}, nil
}
func (s *stubRepo) AnalyticsByStrategy(ctx context.Context) ([]repository.StrategyAnalyticsRow, error) {
	return nil, nil
}
func (s *stubRepo) AnalyticsStrategyOutcomes(ctx context.Context) ([]repository.StrategyOutcomeRow, error) {
	return nil, nil
}
func (s *stubRepo) AnalyticsFailures(ctx context.Context) ([]repository.FailureAnalyticsRow, error) {
	return nil, nil
}
