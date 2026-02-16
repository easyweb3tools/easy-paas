# OpenClaw Responsibilities (Role & Operating Model)

OpenClaw 是 easy-paas 的执行中枢。它通过 `easyweb3-cli` 控制平台与业务系统，把“人类目标”转化为可审计、可回放的动作序列。

OpenClaw upstream:
- https://github.com/openclaw/openclaw

## 1. 定位与边界

OpenClaw = Agent Runtime + Skills + easyweb3-cli

- Runtime：会话管理、工具调用、错误恢复
- Skills：业务流程模板（本仓库示例见 `skills/polymarket-trader/SKILL.md`）
- easyweb3-cli：推荐的统一控制入口

硬边界：
- 查询可直接 GET
- 写操作必须鉴权
- 关键写动作必须有回读校验
- 不绕过 easyweb3-cli 直接写库

## 2. easyweb3-cli 能力地图

核心命令组：
- 认证：`easyweb3 auth ...`
- 透传 API：`easyweb3 api raw ...`
- Polymarket 语义化命令：`easyweb3 api polymarket ...`
- 日志与通知：`easyweb3 log ...` / `easyweb3 notify ...`
- 服务与文档：`easyweb3 service ...` / `easyweb3 docs ...`

推荐优先级：
1. `easyweb3 api polymarket ...`
2. `easyweb3 api raw --service polymarket ...`（兜底）

## 3. 运行前准备

```bash
export EASYWEB3_API_BASE=http://127.0.0.1:18080

easyweb3 auth login --api-key <PAAS_API_KEY>
easyweb3 auth status
```

## 4. polymarket 可控能力（面向 Agent）

### 4.1 机会与执行

```bash
easyweb3 api polymarket opportunities --limit 50 --status active
easyweb3 api polymarket opportunity-get 123
easyweb3 api polymarket opportunity-execute 123

easyweb3 api polymarket executions --limit 50
easyweb3 api polymarket execution-get 456
easyweb3 api polymarket execution-preflight 456
easyweb3 api polymarket execution-submit 456

# 手动补录成交/结算（调试与回补场景）
easyweb3 api polymarket execution-fill --id 456 --token-id <token_id> --direction BUY_YES --filled-size 10 --avg-price 0.42 --fee 0
easyweb3 api polymarket execution-settle --id 456 --body '{"market_outcomes":{"<market_id>":"YES"}}'
```

### 4.2 订单与持仓组合

```bash
# orders
easyweb3 api polymarket orders --limit 100
easyweb3 api polymarket order-get 1001
easyweb3 api polymarket order-cancel 1001

# positions & portfolio
easyweb3 api polymarket positions --limit 200 --status open
easyweb3 api polymarket position-get 88
easyweb3 api polymarket portfolio-summary
easyweb3 api polymarket portfolio-history --limit 168
```

### 4.3 复盘与分析

```bash
# journal
easyweb3 api raw --service polymarket --method GET --path /api/v2/journal?limit=100
easyweb3 api raw --service polymarket --method GET --path /api/v2/journal/456
easyweb3 api raw --service polymarket --method PUT --path /api/v2/journal/456/notes --body '{"notes":"good timing","tags":["good_entry"]}'

# market review
easyweb3 api polymarket review --limit 100
easyweb3 api polymarket review-missed --limit 50
easyweb3 api polymarket review-regret-index
easyweb3 api polymarket review-label-performance
easyweb3 api polymarket review-notes --id 12 --notes "should_have_traded" --lesson-tags should_have_traded,edge_was_real

# analytics
easyweb3 api polymarket analytics-daily --limit 365
easyweb3 api polymarket analytics-attribution --strategy systematic_no
easyweb3 api polymarket analytics-drawdown
easyweb3 api polymarket analytics-correlation
easyweb3 api polymarket analytics-ratios
```

### 4.4 策略与自动化规则（当前仍建议 raw）

```bash
# strategies
easyweb3 api raw --service polymarket --method GET --path /api/v2/strategies
easyweb3 api raw --service polymarket --method POST --path /api/v2/strategies/<strategy_name>/enable --body '{}'

# execution-rules
easyweb3 api raw --service polymarket --method GET --path /api/v2/execution-rules
easyweb3 api raw --service polymarket --method PUT --path /api/v2/execution-rules/<strategy_name> --body '{"auto_execute":true,"min_confidence":0.8,"min_edge_pct":"0.05"}'
```

## 5. 数据库开关与运行时参数

系统开关已经迁移到数据库 `system_settings`，通过 API 动态控制。

### 5.1 feature 开关

```bash
# 列出全部 feature.*
easyweb3 api polymarket switches

# 查询/设置单个开关（name 不带 feature. 前缀）
easyweb3 api polymarket switch-get auto_executor
easyweb3 api polymarket switch-enable auto_executor
easyweb3 api polymarket switch-disable strategy_engine
easyweb3 api polymarket switch-set --name market_review --enabled true
```

当前核心开关名：
- `catalog_sync`
- `clob_stream`
- `strategy_engine`
- `labeler`
- `settlement_ingest`
- `auto_executor`
- `position_sync`
- `portfolio_snapshot`
- `position_manager`
- `daily_stats`
- `market_review`
- `signal.binance_ws`
- `signal.binance_price`
- `signal.weather_api`
- `signal.price_change`
- `signal.orderbook_pattern`
- `signal.certainty_sweep`

### 5.2 通用设置（非布尔）

```bash
# 读取/设置任意 key
easyweb3 api polymarket setting-get trading.executor_mode
easyweb3 api polymarket setting-set --key trading.executor_mode --value '"dry-run"' --desc "executor runtime mode"

# 可切换到 live（当前 live 下单链路仍在完善）
easyweb3 api polymarket setting-set --key trading.executor_mode --value '"live"'
```

## 6. 推荐执行闭环

1. Read：读取机会、风控、开关状态
2. Decide：形成动作计划（目标/约束/回退）
3. Write：执行最小必要写操作
4. Verify：立即回读关键状态
5. Audit：写入决策日志

示例：

```bash
easyweb3 api polymarket opportunities --limit 20 --status active
easyweb3 api polymarket switch-get auto_executor
easyweb3 api polymarket switch-enable auto_executor
easyweb3 api polymarket executions --limit 20
easyweb3 log create --action polymarket_decision --level info --details '{"action":"enable_auto_executor","reason":"high-confidence opportunities"}'
```

## 7. 安全与审计约束

- 不回显 API Key/JWT
- 不在回读校验前宣称“已成功”
- 不直接写数据库
- 对不可逆动作（取消、结算、批量开关）先记录依据
