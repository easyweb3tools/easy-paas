# Pending Development Plan (Auto-Generated From Specs)

This document tracks features in `docs/TRADING_SYSTEM.md` and `docs/REVIEW_SYSTEM.md` that are not fully completed yet.

Execution rule for implementation:
- Build one feature at a time.
- After one feature is complete and validated, immediately continue to the next feature.
- Do not wait for manual "continue" unless blocked.

Status legend:
- `TODO`: not started
- `IN_PROGRESS`: currently developing
- `DONE`: implemented and validated

## A. Trading System Pending

### A1. Position Management & Portfolio (Phase 1)
- `DONE` Position model + migration
- `DONE` PortfolioSnapshot model + migration
- `DONE` Position repository methods (CRUD/list/summary)
- `DONE` Portfolio snapshot repository methods
- `DONE` PositionSyncService (sync from fills + price refresh)
- `DONE` Cron jobs: price refresh + hourly portfolio snapshot
- `DONE` APIs: `/api/v2/positions*`, `/api/v2/portfolio/history`
- `DONE` Frontend page: `/v2/portfolio`

### A2. CLOB Order Engine (Phase 2)
- `DONE` Order model + migration
- `DONE` Order repository methods
- `DONE` CLOB executor MVP (submit/poll/cancel, dry-run-first)
- `DONE` APIs: `/api/v2/orders*`, `/api/v2/executions/:id/submit`
- `DONE` Frontend page: `/v2/orders`
- `TODO` Live exchange integration depth (real CLOB signed order placement + status sync)

### A3. Automated Trading Loop (Phase 3)
- `DONE` ExecutionRule model + CRUD API + frontend automation page
- `DONE` AutoExecutor MVP (rule-based auto execution in dry-run loop)
- `DONE` PositionManager auto close loop (stop-loss / take-profit / hold time / expiry) [MVP]
- `TODO` Full live order submission integration in automation path

## B. Review System Pending

### B1. Trade Journal (Phase 1)
- `DONE` TradeJournal model + migration
- `DONE` JournalService entry/exit capture integration
- `DONE` Journal APIs + frontend journal page
- `TODO` Deep decision-chain completeness fields (entry/exit market execution details parity with spec)

### B2. Strategy Deep Analytics (Phase 2)
- `DONE` StrategyDailyStats model + migration
- `DONE` Daily aggregation service + schedule (MVP)
- `DONE` Attribution service (query-time)
- `DONE` Drawdown / Correlation / Ratios queries (query-time)
- `DONE` Analytics API extensions (`/api/v2/analytics/daily|strategy/:name/daily|strategy/:name/attribution|drawdown|correlation|ratios`)
- `TODO` Frontend analytics enhancement tabs

### B3. Market Review & Missed Alpha (Phase 3)
- `DONE` MarketReview model + migration
- `DONE` Review generation job (`ReviewService`, 6h loop, DB switch controlled)
- `DONE` Review repository + aggregate queries
- `DONE` APIs: `/api/v2/review*`
- `DONE` Frontend page: `/v2/review`
- `DONE` easyweb3-cli review commands (`review`, `review-missed`, `review-regret-index`, `review-label-performance`, `review-notes`)

## C. Remaining Backlog (After B3)

### C1. Trading Live CLOB Depth
- `DONE` Runtime executor mode control via DB setting (`trading.executor_mode`: `"dry-run"`/`"live"`)
- `DONE` Live mode now explicitly reports non-integrated path (no silent noop)
- `TODO` Real live CLOB signed order placement and robust order-state sync
- `TODO` Automation path to use live submit flow end-to-end

### C2. Journal Deep Decision Chain
- `DONE` Enrich journal capture with execution/fill summary snapshots in entry/exit
- `DONE` Journal frontend supports expanded decision-chain panels (signals/entry market/params/outcome snapshots)

### C3. Analytics Frontend Enhancement
- `DONE` Add tabs: Overview / Performance / Attribution
- `DONE` Cumulative PnL + drawdown visualizations (MVP div charts)
- `DONE` Attribution waterfall + ratios KPI panel

## Implementation Order

1. A1 Position Management & Portfolio
2. A2 CLOB Order Engine
3. A3 PositionManager loop + live automation completion
4. B1 Journal deepening
5. B2 Strategy Deep Analytics
6. B3 Market Review & Missed Alpha
7. C2 Journal decision-chain deepening
8. C3 Analytics frontend enhancement
9. C1 Live CLOB depth
