# Trading System Spec

This document specifies the trading system features to be built in the polymarket service. It is organized into three phases. Each phase is self-contained and can be implemented independently.

Read `docs/CODEX_GUIDE.md` first for codebase conventions.

---

## Phase 1: Position Management & Portfolio View

### Goal

The system currently tracks execution plans and fills but has no concept of "what do we currently hold". This phase adds position tracking and a portfolio overview.

### 1.1 New Model: `Position`

File: `services/polymarket/backend/internal/models/position.go`

```go
type Position struct {
    ID             uint64          `gorm:"primaryKey;autoIncrement"`
    TokenID        string          `gorm:"type:varchar(100);not null;uniqueIndex"`
    MarketID       string          `gorm:"type:varchar(100);not null;index"`
    EventID        string          `gorm:"type:varchar(100);index"`
    Direction      string          `gorm:"type:varchar(10);not null"` // "YES" or "NO"
    Quantity       decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    AvgEntryPrice  decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
    CurrentPrice   decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
    CostBasis      decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    UnrealizedPnL  decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    RealizedPnL    decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    Status         string          `gorm:"type:varchar(20);not null;default:'open';index"` // "open", "closed"
    StrategyName   string          `gorm:"type:varchar(50);index"`
    OpenedAt       time.Time       `gorm:"type:timestamptz;not null"`
    ClosedAt       *time.Time      `gorm:"type:timestamptz"`
    CreatedAt      time.Time       `gorm:"type:timestamptz;autoCreateTime"`
    UpdatedAt      time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

### 1.2 New Model: `PortfolioSnapshot`

File: `services/polymarket/backend/internal/models/portfolio_snapshot.go`

```go
type PortfolioSnapshot struct {
    ID              uint64          `gorm:"primaryKey;autoIncrement"`
    SnapshotAt      time.Time       `gorm:"type:timestamptz;not null;uniqueIndex"`
    TotalPositions  int             `gorm:"not null"`
    TotalCostBasis  decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    TotalMarketVal  decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    UnrealizedPnL   decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    RealizedPnL     decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    NetLiquidation  decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    CreatedAt       time.Time       `gorm:"type:timestamptz;autoCreateTime"`
}
```

### 1.3 Position Sync Logic

File: `services/polymarket/backend/internal/service/position_sync.go`

Implement a `PositionSyncService` that:

1. Listens for new fills (called after each `InsertFill`)
2. For each fill, upsert the corresponding `Position`:
   - If position exists for token_id: update quantity, recalculate avg_entry_price (weighted average), update cost_basis
   - If position does not exist: create new position with fill data
   - If quantity reaches 0: set status to "closed", set closed_at
3. Current price update: a periodic job (cron, every 30s) that:
   - Lists all open positions
   - For each, fetches the latest `OrderbookLatest` mid-price for the token
   - Updates `current_price` and recalculates `unrealized_pnl = (current_price - avg_entry_price) * quantity`

### 1.4 Portfolio Snapshot Job

Add a cron job (every 1 hour) that:
1. Queries all open positions
2. Sums up total cost_basis, total market_value, unrealized_pnl, realized_pnl
3. Inserts a `PortfolioSnapshot` row

Config addition to `config.yaml`:
```yaml
position_sync:
  price_refresh_interval: "30s"
  snapshot_interval: "@every 1h"
```

### 1.5 API Endpoints

File: `services/polymarket/backend/internal/handler/v2_positions.go`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/positions` | List positions. Filters: `status` (open/closed), `strategy_name`, `market_id`. Pagination: `limit`, `offset`. Order by: `unrealized_pnl`, `cost_basis`, `opened_at`. |
| `GET` | `/api/v2/positions/:id` | Get single position by ID. |
| `GET` | `/api/v2/positions/summary` | Portfolio summary: total open positions, total cost basis, total market value, total unrealized PnL, total realized PnL, net liquidation value. |
| `GET` | `/api/v2/portfolio/history` | List portfolio snapshots. Filters: `since` (RFC3339), `until` (RFC3339). Limit default 168 (7 days hourly). |

### 1.6 Repository Methods to Add

```go
// In repository.Repository interface:

// Positions
UpsertPosition(ctx context.Context, item *models.Position) error
GetPositionByID(ctx context.Context, id uint64) (*models.Position, error)
GetPositionByTokenID(ctx context.Context, tokenID string) (*models.Position, error)
ListPositions(ctx context.Context, params ListPositionsParams) ([]models.Position, error)
CountPositions(ctx context.Context, params ListPositionsParams) (int64, error)
ListOpenPositions(ctx context.Context) ([]models.Position, error)
ClosePosition(ctx context.Context, id uint64, realizedPnL decimal.Decimal, closedAt time.Time) error
PositionsSummary(ctx context.Context) (PositionsSummary, error)

// Portfolio snapshots
InsertPortfolioSnapshot(ctx context.Context, item *models.PortfolioSnapshot) error
ListPortfolioSnapshots(ctx context.Context, params ListPortfolioSnapshotsParams) ([]models.PortfolioSnapshot, error)
```

New param/result types:
```go
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
    TotalOpen       int64
    TotalCostBasis  float64
    TotalMarketVal  float64
    UnrealizedPnL   float64
    RealizedPnL     float64
    NetLiquidation  float64
}
```

### 1.7 Frontend Page

File: `services/polymarket/frontend/app/v2/portfolio/page.tsx`

Layout:
- Top: 6 KPI cards in a row (Open Positions, Cost Basis, Market Value, Unrealized P&L, Realized P&L, Net Liquidation)
- Middle: Portfolio net value chart over time (from `/api/v2/portfolio/history`)
- Bottom: Positions table with columns: Token, Market, Direction, Qty, Avg Entry, Current Price, Unrealized P&L, Status, Opened At. Filter by status (open/closed).

Add "Portfolio" link to the navigation bar in `app/layout.tsx`.

### 1.8 Integration Points

- In `v2_executions.go` handler `addFill` method: after inserting a fill, call `PositionSyncService.SyncFromFill(ctx, fill)` to update the position.
- In `cmd/monitor/main.go`: register the position price refresh cron job and portfolio snapshot cron job.
- Wire `V2PositionHandler` in main.go handler registration section.

---

## Phase 2: CLOB Order Engine

### Goal

Enable the system to submit orders to Polymarket's CLOB API, track order lifecycle, and auto-record fills.

### 2.1 New Model: `Order`

File: `services/polymarket/backend/internal/models/order.go`

```go
type Order struct {
    ID              uint64          `gorm:"primaryKey;autoIncrement"`
    PlanID          uint64          `gorm:"not null;index"`
    ClobOrderID     string          `gorm:"type:varchar(100);uniqueIndex"` // from CLOB API response
    TokenID         string          `gorm:"type:varchar(100);not null;index"`
    Side            string          `gorm:"type:varchar(10);not null"` // "BUY" or "SELL"
    OrderType       string          `gorm:"type:varchar(20);not null;default:'limit'"` // "limit", "market"
    Price           decimal.Decimal `gorm:"type:numeric(20,10);not null"`
    SizeUSD         decimal.Decimal `gorm:"type:numeric(30,10);not null"`
    FilledUSD       decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
    Status          string          `gorm:"type:varchar(20);not null;default:'pending';index"` // "pending", "submitted", "partial", "filled", "cancelled", "failed"
    FailureReason   string          `gorm:"type:text"`
    SubmittedAt     *time.Time      `gorm:"type:timestamptz"`
    FilledAt        *time.Time      `gorm:"type:timestamptz"`
    CancelledAt     *time.Time      `gorm:"type:timestamptz"`
    CreatedAt       time.Time       `gorm:"type:timestamptz;autoCreateTime;index"`
    UpdatedAt       time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

### 2.2 CLOB Executor Module

File: `services/polymarket/backend/internal/executor/clob_executor.go`

```go
type CLOBExecutor struct {
    Client  *clob.Client
    Repo    repository.Repository
    Risk    *risk.Manager
    Logger  *zap.Logger
    Config  ExecutorConfig
}

type ExecutorConfig struct {
    Mode                string          // "dry-run" or "live"
    MaxOrderSizeUSD     decimal.Decimal
    SlippageToleranceBps int
}
```

Methods:

1. `SubmitPlan(ctx, planID uint64) (*SubmitResult, error)`:
   - Load execution plan by ID
   - Verify plan status is "preflight_pass" (reject otherwise)
   - Run risk preflight recheck
   - For each leg in plan, create an `Order` record (status "pending")
   - If mode is "dry-run": set order status to "filled" immediately with simulated fill, return
   - If mode is "live": call CLOB API to place limit order, update order with clob_order_id, set status to "submitted"
   - Update plan status to "executing"
   - Return result with order IDs

2. `PollOrders(ctx) error`:
   - List all orders with status "submitted" or "partial"
   - For each, query CLOB API for order status
   - If filled: update order status, auto-create Fill record, trigger position sync
   - If partially filled: update filled_usd
   - If cancelled by exchange: update status to "cancelled"

3. `CancelOrder(ctx, orderID uint64) error`:
   - Load order, verify status is "submitted" or "partial"
   - Call CLOB API cancel
   - Update order status

### 2.3 Config Addition

```yaml
executor:
  mode: "dry-run"                    # "dry-run" or "live"
  max_order_size_usd: 100
  slippage_tolerance_bps: 200
  poll_interval: "5s"                # order status polling interval
  # CLOB API credentials (from env vars, not yaml)
  # POLYMARKET_CLOB_API_KEY
  # POLYMARKET_CLOB_API_SECRET
  # POLYMARKET_CLOB_PASSPHRASE
```

### 2.4 API Endpoints

File: `services/polymarket/backend/internal/handler/v2_orders.go`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/orders` | List orders. Filters: `status`, `plan_id`, `token_id`. Pagination. |
| `GET` | `/api/v2/orders/:id` | Get single order. |
| `POST` | `/api/v2/executions/:id/submit` | Submit execution plan to CLOB. Returns confirmation token in dry-run mode, or order IDs in live mode. |
| `POST` | `/api/v2/orders/:id/cancel` | Cancel a submitted order. |

Add to existing `V2ExecutionHandler` or create `V2OrderHandler`.

### 2.5 Execution Plan Status Extension

Current statuses: `draft → preflight_pass → executing → partial → executed`

Extended: `draft → preflight_pass → submitting → submitted → partial_fill → filled → settled`

The `submitting` status is set when `submit` is called. `submitted` when CLOB confirms. The existing `executing`/`executed` statuses map to `submitted`/`filled` in the new flow.

### 2.6 Frontend Updates

- Execution detail page (`app/v2/executions/[id]/page.tsx`): add "Submit to CLOB" button (visible when status is `preflight_pass`), show order status table below fills.
- New Orders page (`app/v2/orders/page.tsx`): list all orders with status badges, cancel button for active orders.

---

## Phase 3: Automated Trading Loop

### Goal

Close the loop: strategy engine automatically executes high-confidence opportunities end-to-end.

### 3.1 New Model: `ExecutionRule`

File: `services/polymarket/backend/internal/models/execution_rule.go`

```go
type ExecutionRule struct {
    ID              uint64          `gorm:"primaryKey;autoIncrement"`
    StrategyName    string          `gorm:"type:varchar(50);not null;uniqueIndex"`
    AutoExecute     bool            `gorm:"not null;default:false"`
    MinConfidence   float64         `gorm:"not null;default:0.8"`
    MinEdgePct      decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.05"`
    StopLossPct     decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.10"`
    TakeProfitPct   decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.20"`
    MaxHoldHours    int             `gorm:"not null;default:72"`
    MaxDailyTrades  int             `gorm:"not null;default:10"`
    CreatedAt       time.Time       `gorm:"type:timestamptz;autoCreateTime"`
    UpdatedAt       time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}
```

### 3.2 Auto-Execution Pipeline

File: `services/polymarket/backend/internal/executor/auto_executor.go`

A background goroutine that:

1. Polls for new opportunities with status "active" every `scan_interval` (default 10s)
2. For each opportunity, checks if the strategy has `auto_execute = true` in its `ExecutionRule`
3. Verifies opportunity meets minimum thresholds (confidence, edge)
4. Checks daily trade count has not exceeded max_daily_trades
5. Creates execution plan → runs preflight → submits to CLOB (all in one flow)
6. Logs each step to PaaS audit log
7. On any failure, sets opportunity to "failed" and logs the reason

### 3.3 Position Auto-Management

File: `services/polymarket/backend/internal/executor/position_manager.go`

A background goroutine that:

1. Runs every `check_interval` (default 30s)
2. For each open position:
   - **Stop-loss**: if `unrealized_pnl / cost_basis < -stop_loss_pct` from the strategy's execution rule, submit a sell order to close the position
   - **Take-profit**: if `unrealized_pnl / cost_basis > take_profit_pct`, submit a sell order
   - **Max hold time**: if `now - opened_at > max_hold_hours`, submit a sell order
   - **Market expiry**: if the market's end_date is within 1 hour, submit a sell order
3. Each auto-close creates an audit log entry with the reason

### 3.4 Config Addition

```yaml
auto_executor:
  enabled: false                     # master switch, default off
  scan_interval: "10s"
  check_interval: "30s"

# ExecutionRule defaults are per-strategy via API, not config file
```

### 3.5 API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v2/execution-rules` | List all execution rules. |
| `GET` | `/api/v2/execution-rules/:strategy` | Get rule for a strategy. |
| `PUT` | `/api/v2/execution-rules/:strategy` | Create or update execution rule. |
| `DELETE` | `/api/v2/execution-rules/:strategy` | Delete rule (disables auto-execute for strategy). |

### 3.6 Frontend Page

File: `services/polymarket/frontend/app/v2/automation/page.tsx`

Layout:
- Global toggle: "Auto-Execution Enabled" (calls a future config endpoint or is display-only based on config)
- Table of strategies with their execution rules: auto_execute toggle, min confidence, stop loss %, take profit %, max hold hours, daily trade limit
- Inline editing for each rule
- Activity log: recent auto-executed trades with timestamps and reasons

---

## Implementation Order

```
Phase 1 (Position Management)    ← START HERE
  ├── models: Position, PortfolioSnapshot
  ├── repository methods
  ├── position sync service
  ├── portfolio snapshot cron
  ├── handler + API endpoints
  ├── frontend portfolio page
  └── wire into main.go

Phase 2 (CLOB Orders)           ← requires Phase 1
  ├── model: Order
  ├── executor module
  ├── order polling cron
  ├── handler + API endpoints
  ├── frontend orders page
  └── wire into main.go

Phase 3 (Automation)            ← requires Phase 2
  ├── model: ExecutionRule
  ├── auto-executor goroutine
  ├── position manager goroutine
  ├── handler + API endpoints
  ├── frontend automation page
  └── wire into main.go
```
