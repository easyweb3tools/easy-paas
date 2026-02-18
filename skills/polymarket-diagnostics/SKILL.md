# Polymarket Pipeline Diagnostics

This skill diagnoses the Polymarket OpenClaw pipeline health by inspecting each layer for data flow issues.

## When to Use

- After deployment to verify the pipeline is producing opportunities
- When opportunity count drops to zero
- When investigating why a specific strategy is not firing
- Periodic health checks

## Steps

1. **Call the pipeline health endpoint**:
   ```
   curl -s $PM_BASE_URL/api/v2/pipeline/health | jq .
   ```

2. **Analyze each layer**:
   - `markets_total` / `markets_active`: Should be > 0. If 0, catalog sync is broken.
   - `orderbook_latest_count`: Should be > 0. If 0, CLOB stream and REST book sync are both failing.
   - `orderbook_fresh_count`: Should be > 0. If 0 but total > 0, data is stale (stream disconnected?).
   - `market_labels_count`: Should be > 0. If 0, labeler has not run or no rules match.
   - `signals_last_hour`: Should contain entries like `no_bias`, `liquidity_gap`, `arb_sum_deviation`, `price_anomaly`. If empty, signal collectors are disabled or have no data.
   - `opportunities_active`: If 0 with signals present, strategies may be too conservative or risk manager is filtering everything.
   - `strategies_enabled` vs `strategies_total`: All strategies should be enabled unless intentionally disabled.

3. **Diagnose bottleneck** (first layer with count = 0 is the blocker):
   - Layer 1 (Markets = 0): Check `feature.catalog_sync` switch and Gamma API connectivity.
   - Layer 2 (Orderbooks = 0): Check `feature.clob_stream` switch. If fresh = 0, restart CLOB stream.
   - Layer 3 (Labels = 0): Check `feature.labeler` switch. Run `POST /api/v2/labels/scan` manually.
   - Layer 4 (Signals = 0): Check `feature.signal.price_change`, `feature.signal.orderbook_pattern`, `feature.signal.certainty_sweep` switches. Check `feature.strategy_engine` is ON.
   - Layer 5 (Opportunities = 0): Check risk manager logs for `risk: filtered` messages. Check strategy params (price bands, deviation thresholds).

4. **Fix suggestions**:
   - Enable disabled switches: `PUT /api/v2/settings/{key}` with `{"enabled": true}`
   - Widen strategy params: `PUT /api/v2/strategies/{name}/params`
   - Force label scan: `POST /api/v2/labels/scan`
   - Force catalog sync: `POST /api/v1/catalog/sync`

5. **Report status**:
   - **HEALTHY**: All layers have non-zero counts, opportunities are being generated.
   - **DEGRADED**: Some layers have data but opportunities = 0 (usually strategy params or risk filters).
   - **BLOCKED**: A critical layer has count = 0 (markets, orderbooks, or signals missing).

## Example Output

```
Pipeline Health Check
=====================
Layer 1 - Markets:     2500 total, 800 active    [OK]
Layer 2 - Orderbooks:  1200 total, 600 fresh     [OK]
Layer 3 - Labels:      350 markets labeled        [OK]
Layer 4 - Signals:     no_bias=42, price_anomaly=15, liquidity_gap=88  [OK]
Layer 5 - Opportunities: 3 active                 [OK]
Layer 6 - Strategies:  12/12 enabled              [OK]

Status: HEALTHY
```
