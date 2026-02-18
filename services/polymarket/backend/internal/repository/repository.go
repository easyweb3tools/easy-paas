package repository

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"polymarket/internal/models"
)

type CatalogRepository interface {
	InTx(ctx context.Context, fn func(tx *gorm.DB) error) error
	UpsertEventsTx(ctx context.Context, tx *gorm.DB, items []models.Event) error
	UpsertMarketsTx(ctx context.Context, tx *gorm.DB, items []models.Market) error
	UpsertTokensTx(ctx context.Context, tx *gorm.DB, items []models.Token) error
	UpsertSeriesTx(ctx context.Context, tx *gorm.DB, items []models.Series) error
	UpsertTagsTx(ctx context.Context, tx *gorm.DB, items []models.Tag) error
	UpsertEventTagsTx(ctx context.Context, tx *gorm.DB, items []models.EventTag) error
	UpsertOrderbookLatest(ctx context.Context, item *models.OrderbookLatest) error
	UpsertMarketDataHealth(ctx context.Context, item *models.MarketDataHealth) error
	UpsertLastTradePrice(ctx context.Context, item *models.LastTradePrice) error
	InsertRawWSEvent(ctx context.Context, item *models.RawWSEvent) error
	InsertRawRESTSnapshot(ctx context.Context, item *models.RawRESTSnapshot) error
	FindMarketsByConditionIDs(ctx context.Context, conditionIDs []string) ([]models.Market, error)
	FindMarketsBySlugs(ctx context.Context, slugs []string) ([]models.Market, error)
	GetMarketBySlug(ctx context.Context, slug string) (*models.Market, error)
	GetEventBySlug(ctx context.Context, slug string) (*models.Event, error)
	ListMarketsByEventID(ctx context.Context, eventID string) ([]models.Market, error)
	ListMarketsByEventIDs(ctx context.Context, eventIDs []string) ([]models.Market, error)
	ListMarketsByIDs(ctx context.Context, marketIDs []string) ([]models.Market, error)
	ListMarketIDsForStream(ctx context.Context, limit int) ([]string, error)
	ListTokensByMarketIDs(ctx context.Context, marketIDs []string) ([]models.Token, error)
	ListTokensByIDs(ctx context.Context, tokenIDs []string) ([]models.Token, error)
	ListMarketDataHealthByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.MarketDataHealth, error)
	ListOrderbookLatestByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.OrderbookLatest, error)
	ListLastTradePricesByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.LastTradePrice, error)
	ListMarketAggregates(ctx context.Context, limit int) ([]EventAggregate, error)
	ListEventsByIDs(ctx context.Context, ids []string) ([]models.Event, error)
	ListEvents(ctx context.Context, params ListEventsParams) ([]models.Event, error)
	CountEvents(ctx context.Context, params ListEventsParams) (int64, error)
	ListMarkets(ctx context.Context, params ListMarketsParams) ([]models.Market, error)
	CountMarkets(ctx context.Context, params ListMarketsParams) (int64, error)
	ListTokens(ctx context.Context, params ListTokensParams) ([]models.Token, error)
	CountTokens(ctx context.Context, params ListTokensParams) (int64, error)
	GetSyncState(ctx context.Context, scope string) (*models.SyncState, error)
	SaveSyncStateTx(ctx context.Context, tx *gorm.DB, state *models.SyncState) error
	ListSyncStates(ctx context.Context) ([]models.SyncState, error)
	ListActiveEventsEndingSoon(ctx context.Context, hoursToExpiry int, limit int) ([]models.Event, error)
}

// Repository is the V2 unified repository expected by the strategy engine modules.
// It intentionally embeds CatalogRepository to preserve existing L1-L3 usage.
type Repository interface {
	CatalogRepository

	// L4: signals
	InsertSignal(ctx context.Context, item *models.Signal) error
	ListSignals(ctx context.Context, params ListSignalsParams) ([]models.Signal, error)
	DeleteExpiredSignals(ctx context.Context, before time.Time) (int64, error)

	// L4: signal sources
	UpsertSignalSource(ctx context.Context, item *models.SignalSource) error
	ListSignalSources(ctx context.Context) ([]models.SignalSource, error)

	// Existing hot data (helpers for collectors).
	ListMarketDataHealthCandidates(ctx context.Context, limit int, minSpreadBps float64) ([]models.MarketDataHealth, error)
	ListYesTokenJumpCandidates(ctx context.Context, limit int, minJumpBps float64, maxSpreadBps float64) ([]TokenJumpCandidate, error)

	// Catalog helpers for labeler.
	ListTagsByEventIDs(ctx context.Context, eventIDs []string) (map[string][]models.Tag, error)

	// L5: strategies
	UpsertStrategy(ctx context.Context, item *models.Strategy) error
	GetStrategyByName(ctx context.Context, name string) (*models.Strategy, error)
	ListStrategies(ctx context.Context) ([]models.Strategy, error)
	SetStrategyEnabled(ctx context.Context, name string, enabled bool) error
	UpdateStrategyParams(ctx context.Context, name string, params []byte) error
	UpdateStrategyStats(ctx context.Context, name string, stats []byte) error

	// L5: opportunities
	InsertOpportunity(ctx context.Context, item *models.Opportunity) error
	UpsertActiveOpportunity(ctx context.Context, item *models.Opportunity) error
	GetOpportunityByID(ctx context.Context, id uint64) (*models.Opportunity, error)
	ListOpportunities(ctx context.Context, params ListOpportunitiesParams) ([]models.Opportunity, error)
	CountOpportunities(ctx context.Context, params ListOpportunitiesParams) (int64, error)
	UpdateOpportunityStatus(ctx context.Context, id uint64, status string) error
	ExpireDueOpportunities(ctx context.Context, now time.Time) (int64, error)
	CountActiveOpportunities(ctx context.Context) (int64, error)
	ListOldestActiveOpportunityIDs(ctx context.Context, limit int) ([]uint64, error)
	BulkUpdateOpportunityStatus(ctx context.Context, ids []uint64, status string) (int64, error)

	// L5: labels
	UpsertMarketLabel(ctx context.Context, item *models.MarketLabel) error
	ListMarketLabels(ctx context.Context, params ListMarketLabelsParams) ([]models.MarketLabel, error)
	DeleteMarketLabel(ctx context.Context, marketID string, label string) error

	// L6: execution & analytics (MVP)
	InsertExecutionPlan(ctx context.Context, item *models.ExecutionPlan) error
	GetExecutionPlanByID(ctx context.Context, id uint64) (*models.ExecutionPlan, error)
	ListExecutionPlans(ctx context.Context, params ListExecutionPlansParams) ([]models.ExecutionPlan, error)
	CountExecutionPlans(ctx context.Context, params ListExecutionPlansParams) (int64, error)
	ListExecutionPlansByStatuses(ctx context.Context, statuses []string, limit int) ([]models.ExecutionPlan, error)
	UpdateExecutionPlanStatus(ctx context.Context, id uint64, status string) error
	UpdateExecutionPlanPreflight(ctx context.Context, id uint64, status string, preflightResult []byte) error
	UpdateExecutionPlanExecutedAt(ctx context.Context, id uint64, status string, executedAt *time.Time) error
	CountExecutionPlansByStrategySince(ctx context.Context, strategyName string, since time.Time) (int64, error)
	InsertFill(ctx context.Context, item *models.Fill) error
	ListFillsByPlanID(ctx context.Context, planID uint64) ([]models.Fill, error)
	UpsertPnLRecord(ctx context.Context, item *models.PnLRecord) error
	GetPnLRecordByPlanID(ctx context.Context, planID uint64) (*models.PnLRecord, error)
	SumRealizedPnLSince(ctx context.Context, since time.Time) (decimal.Decimal, error)

	// Automation rules (L7)
	UpsertExecutionRule(ctx context.Context, item *models.ExecutionRule) error
	GetExecutionRuleByStrategyName(ctx context.Context, strategyName string) (*models.ExecutionRule, error)
	ListExecutionRules(ctx context.Context) ([]models.ExecutionRule, error)
	DeleteExecutionRuleByStrategyName(ctx context.Context, strategyName string) error

	// Trade journal (L7)
	InsertTradeJournal(ctx context.Context, item *models.TradeJournal) error
	GetTradeJournalByPlanID(ctx context.Context, planID uint64) (*models.TradeJournal, error)
	UpdateTradeJournalExit(ctx context.Context, planID uint64, updates map[string]any) error
	UpdateTradeJournalNotes(ctx context.Context, planID uint64, notes string, tags []byte, reviewedAt *time.Time) error
	ListTradeJournals(ctx context.Context, params ListTradeJournalParams) ([]models.TradeJournal, error)
	CountTradeJournals(ctx context.Context, params ListTradeJournalParams) (int64, error)

	// System settings (L8)
	UpsertSystemSetting(ctx context.Context, item *models.SystemSetting) error
	GetSystemSettingByKey(ctx context.Context, key string) (*models.SystemSetting, error)
	ListSystemSettings(ctx context.Context, params ListSystemSettingsParams) ([]models.SystemSetting, error)
	CountSystemSettings(ctx context.Context, params ListSystemSettingsParams) (int64, error)

	// Positions & portfolio (L8)
	UpsertPosition(ctx context.Context, item *models.Position) error
	GetPositionByID(ctx context.Context, id uint64) (*models.Position, error)
	GetPositionByTokenID(ctx context.Context, tokenID string) (*models.Position, error)
	ListPositions(ctx context.Context, params ListPositionsParams) ([]models.Position, error)
	CountPositions(ctx context.Context, params ListPositionsParams) (int64, error)
	ListOpenPositions(ctx context.Context) ([]models.Position, error)
	ClosePosition(ctx context.Context, id uint64, realizedPnL decimal.Decimal, closedAt time.Time) error
	PositionsSummary(ctx context.Context) (PositionsSummary, error)

	InsertPortfolioSnapshot(ctx context.Context, item *models.PortfolioSnapshot) error
	ListPortfolioSnapshots(ctx context.Context, params ListPortfolioSnapshotsParams) ([]models.PortfolioSnapshot, error)

	// Orders (L8)
	InsertOrder(ctx context.Context, item *models.Order) error
	GetOrderByID(ctx context.Context, id uint64) (*models.Order, error)
	ListOrders(ctx context.Context, params ListOrdersParams) ([]models.Order, error)
	CountOrders(ctx context.Context, params ListOrdersParams) (int64, error)
	UpdateOrderStatus(ctx context.Context, id uint64, status string, updates map[string]any) error

	// Strategy deep analytics (L9)
	UpsertStrategyDailyStats(ctx context.Context, item *models.StrategyDailyStats) error
	ListStrategyDailyStats(ctx context.Context, params ListDailyStatsParams) ([]models.StrategyDailyStats, error)
	AttributionByStrategy(ctx context.Context, strategyName string, since, until *time.Time) (AttributionResult, error)
	PortfolioDrawdown(ctx context.Context) (DrawdownResult, error)
	StrategyCorrelation(ctx context.Context, since, until *time.Time) ([]CorrelationRow, error)
	PerformanceRatios(ctx context.Context, since, until *time.Time) (RatiosResult, error)
	RebuildStrategyDailyStats(ctx context.Context, since, until *time.Time) (int, error)

	// Settlement history (L6 support for systematic strategies)
	UpsertMarketSettlementHistory(ctx context.Context, item *models.MarketSettlementHistory) error
	ListMarketSettlementHistoryByMarketIDs(ctx context.Context, marketIDs []string) ([]models.MarketSettlementHistory, error)
	ListRecentMarketSettlementHistory(ctx context.Context, since time.Time, limit int) ([]models.MarketSettlementHistory, error)
	ListLabelNoRateStats(ctx context.Context, labels []string) ([]LabelNoRateRow, error)

	// Market review (L9)
	UpsertMarketReview(ctx context.Context, item *models.MarketReview) error
	GetMarketReviewByMarketID(ctx context.Context, marketID string) (*models.MarketReview, error)
	ListMarketReviews(ctx context.Context, params ListMarketReviewParams) ([]models.MarketReview, error)
	CountMarketReviews(ctx context.Context, params ListMarketReviewParams) (int64, error)
	MissedAlphaSummary(ctx context.Context) (MissedAlphaSummary, error)
	LabelPerformance(ctx context.Context) ([]LabelPerformanceRow, error)
	UpdateMarketReviewNotes(ctx context.Context, id uint64, notes string, lessonTags []byte) error

	// Analytics queries (L6)
	AnalyticsOverview(ctx context.Context) (AnalyticsOverview, error)
	AnalyticsByStrategy(ctx context.Context) ([]StrategyAnalyticsRow, error)
	AnalyticsStrategyOutcomes(ctx context.Context) ([]StrategyOutcomeRow, error)
	AnalyticsFailures(ctx context.Context) ([]FailureAnalyticsRow, error)

	// Pipeline observability (L10)
	CountOrderbookLatest(ctx context.Context, freshWindow time.Duration) (total int64, fresh int64, err error)
	CountMarketLabels(ctx context.Context) (int64, error)
	CountSignalsByType(ctx context.Context, since *time.Time) (map[string]int64, error)
}

type TokenJumpCandidate struct {
	TokenID      string
	MarketID     string
	PriceJumpBps float64
	SpreadBps    float64
	UpdatedAt    time.Time
}

type EventAggregate struct {
	EventID       string
	MarketCount   int
	SumLiquidity  decimal.Decimal
	SumVolume     decimal.Decimal
	LatestUpdated *time.Time
}

type ListEventsParams struct {
	Limit   int
	Offset  int
	Active  *bool
	Closed  *bool
	Slug    *string
	Title   *string
	OrderBy string
	Asc     *bool
}

type ListMarketsParams struct {
	Limit    int
	Offset   int
	Active   *bool
	Closed   *bool
	EventID  *string
	Slug     *string
	Question *string
	OrderBy  string
	Asc      *bool
}

type ListTokensParams struct {
	Limit    int
	Offset   int
	MarketID *string
	Outcome  *string
	Side     *string
	OrderBy  string
	Asc      *bool
}

type ListSignalsParams struct {
	Limit   int
	Offset  int
	Type    *string
	Source  *string
	Since   *time.Time
	OrderBy string
	Asc     *bool
}

type ListOpportunitiesParams struct {
	Limit         int
	Offset        int
	Status        *string
	StrategyName  *string
	Category      *string
	MinEdgePct    *decimal.Decimal
	MinConfidence *float64
	OrderBy       string
	Asc           *bool
}

type ListMarketLabelsParams struct {
	Limit    int
	Offset   int
	MarketID *string
	Label    *string
	SubLabel *string
	OrderBy  string
	Asc      *bool
}

type ListExecutionPlansParams struct {
	Limit   int
	Offset  int
	Status  *string
	OrderBy string
	Asc     *bool
}

type ListTradeJournalParams struct {
	Limit        int
	Offset       int
	StrategyName *string
	Outcome      *string
	Since        *time.Time
	Until        *time.Time
	Tags         []string
	OrderBy      string
	Asc          *bool
}

type ListSystemSettingsParams struct {
	Limit   int
	Offset  int
	Prefix  *string
	OrderBy string
	Asc     *bool
}

type ListPositionsParams struct {
	Limit        int
	Offset       int
	Status       *string
	StrategyName *string
	MarketID     *string
	OrderBy      string
	Asc          *bool
}

type ListPortfolioSnapshotsParams struct {
	Limit  int
	Offset int
	Since  *time.Time
	Until  *time.Time
}

type PositionsSummary struct {
	TotalOpen      int64
	TotalCostBasis float64
	TotalMarketVal float64
	UnrealizedPnL  float64
	RealizedPnL    float64
	NetLiquidation float64
}

type ListOrdersParams struct {
	Limit   int
	Offset  int
	Status  *string
	PlanID  *uint64
	TokenID *string
	OrderBy string
	Asc     *bool
}

type ListDailyStatsParams struct {
	Limit        int
	Offset       int
	StrategyName *string
	Since        *time.Time
	Until        *time.Time
}

type AttributionResult struct {
	EdgeContribution float64
	SlippageCost     float64
	FeeCost          float64
	TimingValue      float64
	NetPnL           float64
}

type DrawdownResult struct {
	MaxDrawdownUSD       float64
	MaxDrawdownPct       float64
	DrawdownDurationDays int
	CurrentDrawdownUSD   float64
	PeakPnL              float64
	TroughPnL            float64
}

type CorrelationRow struct {
	StrategyA   string
	StrategyB   string
	Correlation float64
}

type RatiosResult struct {
	SharpeRatio  float64
	SortinoRatio float64
	WinRate      float64
	ProfitFactor float64
	AvgWin       float64
	AvgLoss      float64
	Expectancy   float64
}

type ListMarketReviewParams struct {
	Limit        int
	Offset       int
	OurAction    *string
	StrategyName *string
	Since        *time.Time
	Until        *time.Time
	MinPnL       *decimal.Decimal
	OrderBy      string
	Asc          *bool
}

type MissedAlphaSummary struct {
	TotalDismissed      int64
	ProfitableDismissed int64
	RegretRate          float64
	MissedAlphaUSD      float64
	AvgMissedEdge       float64
}

type LabelPerformanceRow struct {
	Label       string
	TradedCount int64
	TradedPnL   float64
	MissedCount int64
	MissedAlpha float64
	WinRate     float64
}

type AnalyticsOverview struct {
	TotalPlans   int64
	TotalPnLUSD  float64
	AvgROI       float64
	WinCount     int64
	LossCount    int64
	PendingCount int64
}

type StrategyAnalyticsRow struct {
	StrategyName string
	Plans        int64
	TotalPnLUSD  float64
	AvgROI       float64
}

type StrategyOutcomeRow struct {
	StrategyName string
	WinCount     int64
	LossCount    int64
	PartialCount int64
	PendingCount int64
}

type FailureAnalyticsRow struct {
	FailureReason string
	Count         int64
}

type LabelNoRateRow struct {
	Label   string
	Total   int64
	NoCount int64
	NoRate  float64
}
