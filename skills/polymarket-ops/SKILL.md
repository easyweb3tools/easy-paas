# Polymarket Ops: Startup Self-Check

## Description
Automated startup self-check for the Polymarket service. Verifies all critical feature switches are enabled, checks recent signal and opportunity pipeline health, and produces a status report.

## Trigger
Run this skill on startup or when asked to "check polymarket health" or "startup check".

## Steps

### 1. Check All Feature Switches
```bash
exec easyweb3 api polymarket switches
```
Review the output. The following core switches must be ON:
- `feature.strategy_engine`
- `feature.labeler`
- `feature.settlement_ingest`
- `feature.catalog_sync`
- `feature.clob_stream`

### 2. Enable Any Disabled Core Switches
For each core switch that is OFF, enable it:
```bash
exec easyweb3 api polymarket switches --set feature.strategy_engine=true
exec easyweb3 api polymarket switches --set feature.labeler=true
exec easyweb3 api polymarket switches --set feature.settlement_ingest=true
exec easyweb3 api polymarket switches --set feature.catalog_sync=true
exec easyweb3 api polymarket switches --set feature.clob_stream=true
```

### 3. Check Recent Signals
```bash
exec easyweb3 api polymarket signals --limit 10
```
Verify that signals are being generated. If no signals exist, note this as a warning.

### 4. Check Recent Opportunities
```bash
exec easyweb3 api polymarket opportunities --limit 10
```
Verify that opportunities are being generated. If none exist and signals are present, note this as a warning.

### 5. Log the Check
```bash
exec easyweb3 log create --agent polymarket-ops --action startup_self_check --level info --details '{"switches_checked": true, "signals_present": <true|false>, "opportunities_present": <true|false>, "switches_enabled": ["<list of switches that were enabled>"]}'
```

### 6. Output Health Report
Produce a summary like:

```
=== Polymarket Startup Self-Check ===
Switches:
  - strategy_engine: ON
  - labeler: ON
  - settlement_ingest: ON
  - catalog_sync: ON
  - clob_stream: ON
  Actions taken: enabled [list] (or "none")

Pipeline:
  - Recent signals: <count> (types: <signal_types>)
  - Recent opportunities: <count>

Status: HEALTHY | DEGRADED | UNHEALTHY
```

- HEALTHY: all switches on, signals and opportunities present
- DEGRADED: all switches on but no opportunities yet (may need time)
- UNHEALTHY: switches were off or no signals at all
