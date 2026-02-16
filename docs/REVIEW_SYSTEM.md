# Review System Spec

This document specifies the trade review and retrospective system for the polymarket service. It is organized into three phases. Each phase is self-contained.

Read `docs/CODEX_GUIDE.md` first for codebase conventions.

---

## Phase 1: Trade Journal & Decision Tracing

### Goal

Make every trade fully traceable: from the signal that triggered it, through the strategy decision, to the final outcome. Currently the system has PnL records but no way to reconstruct "why was this trade made".

### 1.1 New Model: `TradeJournal`

File: `services/polymarket/backend/internal/models/trade_journal.go`

```go
type TradeJournal struct {
    ID                uint64          `gorm:"primaryKey;autoIncrement"`
    ExecutionPlanID   uint64          `gorm:"not null;uniqueIndex"`
    OpportunityID     uint64          `gorm:"not null;index"`
    StrategyName      string          `gorm:"type:varchar(50);not null;index"`

    // Snapshots captured at entry time
    EntryReasoning    string          `gorm:"type:text"`                  // from opportunity.reasoning
    SignalSnapshot    datatypes.JSON  `gorm:"type:jsonb"`                 // signals at time of opportunity creation
    MarketSnapshot    datatypes.JSON  `gorm:"type:jsonb"`                 // orderbook + price data at entry
    EntryParams       datatypes.JSON  `gorm:"type:jsonb"`                 // strategy params used

    // Filled after settlement
    ExitReasoning     string          `gorm:"type:text"`                  // why position was closed
    OutcomeSnapshot   datatypes.JSON  `gorm:"type:jsonb"`                 // market data at settlement
    Outcome           string          `gorm:"type:varchar(20);index"`     // "win", "loss", "partial", "pending"
    PnLUSD            decimal.Decimal `gorm:"type:numeric(30,10)"`
    ROI               decimal.Decimal `gorm:"type:numeric(20,10)"`

    // Human review
    Notes             string          `gorm:"type:text"`
    Tags              datatypes.JSON  `gorm:"type:jsonb"`                 // ["good_entry", "bad_timing", ...]
    ReviewedAt        *time.Time      `gorm:"type:timestamptz"`

    CreatedAt         time.Time       `gorm:"type:timestamptz;autoCreateTime;index"`
    UpdatedAt         time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

### 1.2 Auto-Capture Logic

File: `services/polymarket/backend/internal/service/journal_service.go`

```go
type JournalService struct {
    Repo   repository.Repository
    Logger *zap.Logger
}
```

Methods:

1. `CaptureEntry(ctx, planID uint64) error`:
   - Load execution plan and its parent opportunity
   - Load signals referenced by opportunity.signal_ids
   - Load orderbook data for tokens in opportunity legs
   - Create TradeJournal with:
     - `entry_reasoning` = opportunity.reasoning
     - `signal_snapshot` = JSON array of signal records
     - `market_snapshot` = JSON object with token orderbooks and prices
     - `entry_params` = plan.params
   - This method should be called when an execution plan transitions to "executing" status

2. `CaptureExit(ctx, planID uint64) error`:
   - Load journal by execution_plan_id
   - Load PnL record for the plan
   - Load current market data for the tokens
   - Update journal with:
     - `outcome` = pnl_record.outcome
     - `pnl_usd` = pnl_record.realized_pnl
     - `roi` = pnl_record.realized_roi
     - `outcome_snapshot` = current market data
     - `exit_reasoning` = auto-generated summary (e.g., "settled: market resolved YES, position was NO")
   - This method should be called when `settle` is called on an execution plan

### 1.3 Integration Points

Modify these existing handlers to trigger journal capture:

- `v2_executions.go` → `markExecuting()`: after updating status, call `JournalService.CaptureEntry(ctx, planID)`
- `v2_executions.go` → `settle()`: after settling PnL, call `JournalService.CaptureExit(ctx, planID)`

### 1.4 API Endpoints

File: `services/polymarket/backend/internal/handler/v2_journal.go`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/journal` | List journal entries. Filters: `strategy_name`, `outcome` (win/loss/partial/pending), `since` (RFC3339), `until` (RFC3339), `tags` (comma-separated). Pagination: `limit`, `offset`. Default order: `created_at desc`. |
| `GET` | `/api/v2/journal/:execution_plan_id` | Get single journal entry with full decision chain. |
| `PUT` | `/api/v2/journal/:execution_plan_id/notes` | Update human review notes and tags. Request body: `{"notes": "...", "tags": ["good_entry", "bad_timing"], "reviewed_at": "2026-01-01T00:00:00Z"}` |

### 1.5 Repository Methods to Add

```go
// In repository.Repository interface:

// Trade journal
InsertTradeJournal(ctx context.Context, item *models.TradeJournal) error
GetTradeJournalByPlanID(ctx context.Context, planID uint64) (*models.TradeJournal, error)
UpdateTradeJournalExit(ctx context.Context, planID uint64, updates map[string]any) error
UpdateTradeJournalNotes(ctx context.Context, planID uint64, notes string, tags []byte, reviewedAt *time.Time) error
ListTradeJournals(ctx context.Context, params ListTradeJournalParams) ([]models.TradeJournal, error)
CountTradeJournals(ctx context.Context, params ListTradeJournalParams) (int64, error)
```

New params type:
```go
type ListTradeJournalParams struct {
    Limit        int
    Offset       int
    StrategyName *string
    Outcome      *string
    Since        *time.Time
    Until        *time.Time
    Tags         []string    // any match
    OrderBy      string
    Asc          *bool
}
```

### 1.6 Frontend Page

File: `services/polymarket/frontend/app/v2/journal/page.tsx`

Layout:
- Filter bar: strategy dropdown, outcome dropdown (win/loss/all), date range
- Journal table: Date, Strategy, Market, Direction, Entry Price, Exit Price, P&L, ROI, Outcome, Tags
- Click a row to expand full decision chain:
  - **Signals**: list of signals that triggered the opportunity (from signal_snapshot)
  - **Strategy Reasoning**: the entry_reasoning text
  - **Market State at Entry**: bid/ask/spread from market_snapshot
  - **Execution**: planned size, actual fills, slippage
  - **Outcome**: market resolution, realized P&L
  - **Notes**: editable text area for human review notes
  - **Tags**: clickable tag chips (add/remove)

Add "Journal" link to the navigation bar in `app/layout.tsx`.

---

## Phase 2: Strategy Deep Analytics

### Goal

Upgrade analytics from "how much did we make" to "why did we make/lose money". Add time-series performance tracking, attribution analysis, and drawdown metrics.

### 2.1 New Model: `StrategyDailyStats`

File: `services/polymarket/backend/internal/models/strategy_daily_stats.go`

```go
type StrategyDailyStats struct {
    ID              uint64          `gorm:"primaryKey;autoIncrement"`
    StrategyName    string          `gorm:"type:varchar(50);not null;index"`
    Date            time.Time       `gorm:"type:date;not null;index"`
    TradesCount     int             `gorm:"not null;default:0"`
    WinCount        int             `gorm:"not null;default:0"`
    LossCount       int             `gorm:"not null;default:0"`
    PnLUSD          decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    AvgEdgePct      decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
    AvgSlippageBps  decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
    AvgHoldHours    decimal.Decimal `gorm:"type:numeric(20,4);not null;default:0"`
    MaxDrawdownUSD  decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    CumulativePnL   decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    CreatedAt       time.Time       `gorm:"type:timestamptz;autoCreateTime"`
    UpdatedAt       time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

Unique constraint: `(strategy_name, date)` composite unique index.

### 2.2 Daily Stats Aggregation Job

File: `services/polymarket/backend/internal/service/daily_stats_service.go`

A cron job (runs daily at 00:05 UTC, or `@every 6h` for catch-up) that:

1. For each strategy, queries PnL records settled in the previous day
2. Aggregates: count, win/loss, total PnL, avg expected edge, avg slippage
3. Calculates hold time from execution plan created_at to settled_at
4. Computes cumulative PnL (running sum of all prior daily PnL for the strategy)
5. Computes max drawdown: the largest peak-to-trough decline in cumulative PnL
6. Upserts into `strategy_daily_stats`

Config addition:
```yaml
daily_stats:
  enabled: true
  schedule: "5 0 * * *"    # 00:05 UTC daily
```

### 2.3 Attribution Analysis

File: `services/polymarket/backend/internal/service/attribution_service.go`

For a given strategy, compute PnL attribution breakdown:

- **Edge contribution**: sum of `expected_edge` across all settled plans
- **Slippage cost**: sum of `slippage_loss` across all settled plans
- **Fee cost**: sum of fees from fills
- **Timing value**: `realized_pnl - expected_edge + slippage_loss + fees` (residual)

This is a query-time calculation, not stored.

### 2.4 API Endpoints

Add to existing `V2AnalyticsHandler` or create new handler.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/analytics/daily` | List daily stats across all strategies. Filters: `strategy_name`, `since`, `until`. Returns array of StrategyDailyStats. |
| `GET` | `/api/v2/analytics/strategy/:name/daily` | Daily stats for a specific strategy. Filters: `since`, `until`. |
| `GET` | `/api/v2/analytics/strategy/:name/attribution` | PnL attribution breakdown for a strategy. Filters: `since`, `until`. Returns: `{ edge_contribution, slippage_cost, fee_cost, timing_value, net_pnl }` |
| `GET` | `/api/v2/analytics/drawdown` | Portfolio-level drawdown metrics. Returns: `{ max_drawdown_usd, max_drawdown_pct, drawdown_duration_days, current_drawdown_usd, peak_pnl, trough_pnl }` |
| `GET` | `/api/v2/analytics/correlation` | Strategy correlation matrix. Returns NxN matrix of daily PnL correlations between strategies. |
| `GET` | `/api/v2/analytics/ratios` | Key performance ratios. Returns: `{ sharpe_ratio, sortino_ratio, win_rate, profit_factor, avg_win, avg_loss, expectancy }` |

### 2.5 Repository Methods to Add

```go
// In repository.Repository interface:

// Strategy daily stats
UpsertStrategyDailyStats(ctx context.Context, item *models.StrategyDailyStats) error
ListStrategyDailyStats(ctx context.Context, params ListDailyStatsParams) ([]models.StrategyDailyStats, error)

// Attribution (query-time, no model)
AttributionByStrategy(ctx context.Context, strategyName string, since, until *time.Time) (AttributionResult, error)

// Drawdown (query-time)
PortfolioDrawdown(ctx context.Context) (DrawdownResult, error)

// Correlation (query-time)
StrategyCorrelation(ctx context.Context, since, until *time.Time) ([]CorrelationRow, error)

// Ratios (query-time)
PerformanceRatios(ctx context.Context, since, until *time.Time) (RatiosResult, error)
```

New result types:
```go
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
    MaxDrawdownUSD      float64
    MaxDrawdownPct      float64
    DrawdownDurationDays int
    CurrentDrawdownUSD  float64
    PeakPnL             float64
    TroughPnL           float64
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
```

### 2.6 Frontend Enhancement

Enhance existing analytics page `app/v2/analytics/page.tsx`:

- Add tabs: "Overview" (existing) | "Performance" (new) | "Attribution" (new)
- **Performance tab**:
  - Cumulative P&L line chart (x: date, y: cumulative PnL USD), one line per strategy + total
  - Drawdown chart (underwater curve): x: date, y: drawdown from peak
  - Date range selector
- **Attribution tab**:
  - Strategy selector dropdown
  - Waterfall chart: Expected Edge → Slippage → Fees → Timing → Net P&L
  - KPI cards: Sharpe, Win Rate, Profit Factor, Expectancy
- Use simple HTML/CSS charts (div bars) or SVG. No external chart library needed for MVP.

---

## Phase 3: Market Review & Missed Alpha

### Goal

Systematically evaluate what opportunities were missed and which market categories perform best. Enable learning from both taken and not-taken trades.

### 3.1 New Model: `MarketReview`

File: `services/polymarket/backend/internal/models/market_review.go`

```go
type MarketReview struct {
    ID              uint64          `gorm:"primaryKey;autoIncrement"`
    MarketID        string          `gorm:"type:varchar(100);not null;index"`
    EventID         string          `gorm:"type:varchar(100);index"`
    OurAction       string          `gorm:"type:varchar(20);not null;index"` // "traded", "dismissed", "missed", "expired"
    OpportunityID   *uint64         `gorm:"index"`                           // link to opportunity if exists
    StrategyName    string          `gorm:"type:varchar(50);index"`
    EdgeAtEntry     decimal.Decimal `gorm:"type:numeric(20,10)"`             // edge when opportunity was created
    FinalOutcome    string          `gorm:"type:varchar(10)"`                // "YES" or "NO"
    FinalPrice      decimal.Decimal `gorm:"type:numeric(20,10)"`
    HypotheticalPnL decimal.Decimal `gorm:"type:numeric(30,10)"`             // what we would have made
    ActualPnL       decimal.Decimal `gorm:"type:numeric(30,10)"`             // what we actually made (0 if not traded)
    LessonTags      datatypes.JSON  `gorm:"type:jsonb"`
    Notes           string          `gorm:"type:text"`
    SettledAt       time.Time       `gorm:"type:timestamptz;index"`
    CreatedAt       time.Time       `gorm:"type:timestamptz;autoCreateTime"`
    UpdatedAt       time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

### 3.2 Review Generation Job

File: `services/polymarket/backend/internal/service/review_service.go`

A cron job (runs every 6 hours) that:

1. Scans `market_settlement_history` for recently settled markets (last 24h)
2. For each settled market, checks if we had any opportunities:
   - **Traded**: opportunity → execution plan exists with fills → `our_action = "traded"`, compute actual vs hypothetical PnL
   - **Dismissed**: opportunity existed but was dismissed → `our_action = "dismissed"`, compute hypothetical PnL
   - **Expired**: opportunity existed but expired → `our_action = "expired"`, compute hypothetical PnL
   - **Missed**: no opportunity was ever created for this market → `our_action = "missed"`, estimate hypothetical PnL using settlement price vs mid-price at some lookback
3. Computes `hypothetical_pnl`: for dismissed/expired/missed, calculate what a standard-size position (from risk config `default_kelly_fraction * max_per_market_usd`) would have earned
4. Upserts into `market_review`

Config addition:
```yaml
market_review:
  enabled: true
  schedule: "@every 6h"
  lookback_hours: 24
  hypothetical_size_usd: 100    # default position size for "what if" calculations
```

### 3.3 API Endpoints

File: `services/polymarket/backend/internal/handler/v2_review.go`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/review` | List market reviews. Filters: `our_action` (traded/dismissed/missed/expired), `strategy_name`, `since`, `until`. Pagination. Order by: `hypothetical_pnl desc` (default, shows biggest missed alpha first). |
| `GET` | `/api/v2/review/missed` | Shortcut: list reviews where `our_action` is "dismissed", "expired", or "missed" AND `hypothetical_pnl > 0`. Shows profitable opportunities we did not take. |
| `GET` | `/api/v2/review/regret-index` | Aggregated regret metrics: `{ total_dismissed, profitable_dismissed, regret_rate, missed_alpha_usd, avg_missed_edge }`. Regret rate = profitable_dismissed / total_dismissed. |
| `GET` | `/api/v2/review/label-performance` | P&L breakdown by market label. Returns: `[{ label, traded_count, traded_pnl, missed_count, missed_alpha, win_rate }]`. Join market_review with market_labels. |
| `PUT` | `/api/v2/review/:id/notes` | Add human review notes and lesson tags. Request body: `{"notes": "...", "lesson_tags": ["should_have_traded", "edge_was_real"]}` |

### 3.4 Repository Methods to Add

```go
// In repository.Repository interface:

// Market review
UpsertMarketReview(ctx context.Context, item *models.MarketReview) error
GetMarketReviewByMarketID(ctx context.Context, marketID string) (*models.MarketReview, error)
ListMarketReviews(ctx context.Context, params ListMarketReviewParams) ([]models.MarketReview, error)
CountMarketReviews(ctx context.Context, params ListMarketReviewParams) (int64, error)
MissedAlphaSummary(ctx context.Context) (MissedAlphaSummary, error)
LabelPerformance(ctx context.Context) ([]LabelPerformanceRow, error)
UpdateMarketReviewNotes(ctx context.Context, id uint64, notes string, lessonTags []byte) error
```

New types:
```go
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
    RegretRate          float64   // profitable / total
    MissedAlphaUSD      float64   // sum of hypothetical_pnl for missed profitable trades
    AvgMissedEdge       float64
}

type LabelPerformanceRow struct {
    Label        string
    TradedCount  int64
    TradedPnL    float64
    MissedCount  int64
    MissedAlpha  float64
    WinRate      float64
}
```

### 3.5 Frontend Page

File: `services/polymarket/frontend/app/v2/review/page.tsx`

Layout with tabs: "Missed Alpha" | "Label Performance" | "All Reviews"

- **Missed Alpha tab**:
  - Top KPI cards: Regret Rate, Total Missed Alpha USD, Avg Missed Edge
  - Table: Market, Strategy, Our Action, Edge at Entry, Final Outcome, Hypothetical P&L, Lesson Tags
  - Sorted by hypothetical P&L descending (biggest misses first)

- **Label Performance tab**:
  - Table: Label, Traded Count, Traded P&L, Win Rate, Missed Count, Missed Alpha
  - Highlight labels with high win rate + high missed alpha (actionable: should trade more of these)

- **All Reviews tab**:
  - Full list of all market reviews with all filters
  - Click to expand: full details, editable notes and lesson tags

Add "Review" link to the navigation bar in `app/layout.tsx`.

---

## Implementation Order

```
Phase 1 (Trade Journal)              ← START HERE
  ├── model: TradeJournal
  ├── journal service (auto-capture)
  ├── integrate into execution handlers
  ├── repository methods
  ├── handler + API endpoints
  ├── frontend journal page
  └── wire into main.go

Phase 2 (Strategy Analytics)         ← independent of Phase 1
  ├── model: StrategyDailyStats
  ├── daily stats aggregation cron
  ├── attribution query service
  ├── repository methods
  ├── handler + API endpoints
  ├── frontend analytics enhancement
  └── wire into main.go

Phase 3 (Market Review)              ← requires settled markets data
  ├── model: MarketReview
  ├── review generation cron
  ├── repository methods
  ├── handler + API endpoints
  ├── frontend review page
  └── wire into main.go
```

---

## Cross-References

- Trading System: see `docs/TRADING_SYSTEM.md`
- Codebase Conventions: see `docs/CODEX_GUIDE.md`
- Architecture: see `docs/ARCHITECTURE.md`
