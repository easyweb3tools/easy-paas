# OpenClaw Responsibilities (Role & Operating Model)

OpenClaw 是 easy-paas 的执行中枢。它通过 **easyweb3-cli** 控制平台与业务系统，把“人类目标”转化为可审计、可回放的动作序列。

OpenClaw upstream:
- https://github.com/openclaw/openclaw

## 1. 定位与边界

OpenClaw = Agent Runtime + Skills + easyweb3-cli

- Runtime：会话管理、工具调用、错误恢复。
- Skills：业务流程模板（本仓库见 `skills/polymarket-trader/SKILL.md`）。
- easyweb3-cli：唯一推荐写操作入口（避免直接拼 HTTP 导致协议漂移）。

硬边界：
- 查询可走读接口（GET）。
- 写操作必须鉴权，且应通过 `easyweb3` 发起。
- 每个关键写动作必须可追溯（日志 + 结果回读校验）。

## 2. easyweb3-cli 能力地图

easyweb3-cli（有时口语称 easyweb-cli）主要能力：

- 认证与令牌管理：`auth`
- 网关透传调用：`api raw`
- 业务高层命令（polymarket）：`api polymarket`
- 平台日志与通知：`log`、`notify`
- 服务发现与健康检查：`service`
- 外部集成查询：`integrations`
- 缓存操作：`cache`
- 文档读取：`docs`

优先级建议：
1. 首选 `easyweb3 api polymarket ...`（语义化命令，最稳定）
2. 其次 `easyweb3 api raw --service polymarket ...`（兜底/调试）
3. 避免在 Agent 内直接手写 curl 调内部地址做写入

## 3. 运行前准备

```bash
# 1) 配置 API Base（示例）
export EASYWEB3_API_BASE=http://127.0.0.1:18080

# 2) 登录（API Key -> JWT）
easyweb3 auth login --api-key <PAAS_API_KEY>

# 3) 检查当前凭据
easyweb3 auth status
```

建议：
- `easyweb3` 位于 `/usr/local/bin/easyweb3`
- `EASYWEB3_DIR` 放在 OpenClaw workspace 内，便于 exec sandbox 访问

## 4. polymarket 命令清单（Agent 常用）

机会与执行：

```bash
easyweb3 api polymarket opportunities --limit 50 --status active
easyweb3 api polymarket opportunity-get 123
easyweb3 api polymarket opportunity-execute 123

easyweb3 api polymarket executions --limit 50
easyweb3 api polymarket execution-get 456
easyweb3 api polymarket execution-preflight 456
easyweb3 api polymarket execution-mark-executing 456
easyweb3 api polymarket execution-fill --id 456 --token-id <token> --direction BUY_YES --filled-size 10 --avg-price 0.42 --fee 0
easyweb3 api polymarket execution-settle --id 456 --body '{"market_outcomes":{"<market_id>":"YES"}}'
```

策略、自动化规则、复盘（当前建议走 `api raw`）：

```bash
# strategies
easyweb3 api raw --service polymarket --method GET --path /api/v2/strategies
easyweb3 api raw --service polymarket --method POST --path /api/v2/strategies/<strategy_name>/enable --body '{}'
easyweb3 api raw --service polymarket --method POST --path /api/v2/strategies/<strategy_name>/disable --body '{}'

# execution-rules
easyweb3 api raw --service polymarket --method GET --path /api/v2/execution-rules
easyweb3 api raw --service polymarket --method GET --path /api/v2/execution-rules/<strategy_name>
easyweb3 api raw --service polymarket --method PUT --path /api/v2/execution-rules/<strategy_name> --body '{"auto_execute":true,"min_confidence":0.8,"min_edge_pct":"0.05"}'

# journal
easyweb3 api raw --service polymarket --method GET --path /api/v2/journal?limit=100
easyweb3 api raw --service polymarket --method GET --path /api/v2/journal/456
easyweb3 api raw --service polymarket --method PUT --path /api/v2/journal/456/notes --body '{"notes":"good timing","tags":["good_entry"]}'
```

说明：`api polymarket` 优先用于已封装高频动作；其余接口统一用 `api raw` 兜底。

## 5. 数据库开关控制（核心）

系统开关已迁移到数据库 `system_settings`，推荐使用以下命令让 Agent 动态控制系统行为：

```bash
# 列出全部 feature.* 开关
easyweb3 api polymarket switches

# 查询单个开关
easyweb3 api polymarket switch-get auto_executor

# 开启/关闭开关
easyweb3 api polymarket switch-enable auto_executor
easyweb3 api polymarket switch-disable strategy_engine

# 显式设置布尔值
easyweb3 api polymarket switch-set --name settlement_ingest --enabled true
```

通用设置（非布尔）：

```bash
easyweb3 api polymarket setting-get feature.catalog_sync
easyweb3 api polymarket setting-set --key feature.catalog_sync --value true --desc "runtime feature switch"
```

当前核心开关名（`feature.<name>`）：
- `catalog_sync`
- `clob_stream`
- `strategy_engine`
- `labeler`
- `settlement_ingest`
- `auto_executor`
- `signal.binance_ws`
- `signal.binance_price`
- `signal.weather_api`
- `signal.price_change`
- `signal.orderbook_pattern`
- `signal.certainty_sweep`

## 6. 推荐操作闭环（Read -> Decide -> Write -> Verify）

1. Read：先查状态与风险上下文
2. Decide：输出计划摘要（目标、理由、风险、回退）
3. Write：执行最小必要写操作
4. Verify：立即回读验证状态
5. Audit：补充决策日志

示例：

```bash
easyweb3 api polymarket opportunities --limit 20 --status active
easyweb3 api polymarket switch-get auto_executor
easyweb3 api polymarket switch-enable auto_executor
easyweb3 api polymarket executions --limit 20
easyweb3 log create --action polymarket_decision --level info --details '{"action":"enable_auto_executor","reason":"high-confidence opportunities"}'
```

## 7. 审计与安全要求

- 不回显 API Key/JWT 到日志或回复正文。
- 不在未读回校验前宣称“已执行成功”。
- 不绕过 easyweb3-cli 直接写数据库。
- 对不可逆动作（取消、结算、批量开关）必须先记录决策依据。
