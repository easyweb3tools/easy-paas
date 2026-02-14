# easyweb3 PaaS + PicoClaw Agent 架构设计

> easyweb3 = PaaS 平台 + 业务 Service
> PicoClaw = AI 大脑，一个项目一个 Agent
> easyweb3-cli = SDK 工具，供 PicoClaw skill 调用
> 本文档为 Codex 等 AI 编程代理提供完整的实现规格。

---

## 1. 核心架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      用户层                                      │
│  Telegram │ Discord │ Feishu │ WhatsApp │ Slack │ Web Chat       │
└─────────────────────────────┬───────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                 PicoClaw Agent 层                                │
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐                     │
│  │ Agent: easymeme   │  │ Agent: storyfork  │  ... (一项目一Agent) │
│  │ Skill: trader     │  │ Skill: writer     │                     │
│  │ Session: tg:group1│  │ Session: tg:group2│                     │
│  └────────┬─────────┘  └────────┬─────────┘                     │
│           │                     │                                │
│           ↓                     ↓                                │
│     ┌───────────────────────────────────────┐                   │
│     │          exec → easyweb3-cli           │                   │
│     │  (SDK CLI 工具，统一调用 PaaS API)      │                   │
│     │                                        │                   │
│     │  easyweb3 auth login                   │                   │
│     │  easyweb3 log create ...               │                   │
│     │  easyweb3 api meme get-golden-dogs     │                   │
│     │  easyweb3 api meme trade buy ...       │                   │
│     │  easyweb3 api story create-branch ...  │                   │
│     │  easyweb3 notify send ...              │                   │
│     └───────────────────┬───────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
                          ↓ HTTPS + Bearer Token
┌─────────────────────────────────────────────────────────────────┐
│                 easyweb3 PaaS 平台                               │
│                                                                  │
│  ┌──────────────── API Gateway (Nginx) ────────────────────┐    │
│  │  api.easyweb3.tools                                      │    │
│  │  ├── /api/v1/auth/*        认证                          │    │
│  │  ├── /api/v1/logs/*        AI 操作日志                    │    │
│  │  ├── /api/v1/notify/*      消息通知                       │    │
│  │  ├── /api/v1/cache/*       缓存                          │    │
│  │  ├── /api/v1/services/meme/*     → EasyMeme Service      │    │
│  │  ├── /api/v1/services/story/*    → StoryFork Service     │    │
│  │  └── /api/v1/services/{name}/*   → 未来项目 Service       │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────── PaaS 基础设施 ──────────────┐                     │
│  │                                        │                     │
│  │  ┌──────────┐  ┌──────────┐  ┌──────┐ │                     │
│  │  │ Auth     │  │ Logging  │  │ Cache│ │                     │
│  │  │ JWT/RBAC │  │ 操作日志  │  │ Redis│ │                     │
│  │  └──────────┘  └──────────┘  └──────┘ │                     │
│  │  ┌──────────┐  ┌──────────┐           │                     │
│  │  │ Notify   │  │ Integr.  │           │                     │
│  │  │ TG/Email │  │ GoPlus   │           │                     │
│  │  │ Webhook  │  │ DEXScr.  │           │                     │
│  │  └──────────┘  └──────────┘           │                     │
│  └────────────────────────────────────────┘                     │
│                                                                  │
│  ┌─────────── Business Services ──────────┐                     │
│  │                                        │                     │
│  │  ┌──────────────┐  ┌────────────────┐ │                     │
│  │  │ EasyMeme     │  │ StoryFork      │ │                     │
│  │  │ (Go/Gin)     │  │ (Next.js)      │ │                     │
│  │  │ 代币扫描/交易 │  │ 分支叙事/x402  │ │                     │
│  │  └──────┬───────┘  └──────┬─────────┘ │                     │
│  │         ↓                 ↓           │                     │
│  │     PostgreSQL        PostgreSQL      │                     │
│  │         ↓                 ↓           │                     │
│  │     BSC Chain         Stacks Chain    │                     │
│  └────────────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────────┘
                          ↑ 读取同一数据库
┌─────────────────────────────────────────────────────────────────┐
│                 Web 展示层（AI 工作日志）                          │
│  meme.easyweb3.tools   │  story.easyweb3.tools                   │
│  AI 交易记录/盈亏统计   │  AI 生成的故事/投票记录                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. 组件详细设计

### 2.1 easyweb3-cli — SDK 命令行工具

**定位**：PicoClaw 通过 `exec` 工具调用的 CLI，统一封装 PaaS API。
**语言**：Go（与 picoclaw 生态一致，编译为单一二进制，部署简单）
**安装位置**：`~/.local/bin/easyweb3`（picoclaw 的 `exec` 可直接调用）

#### 命令结构

```
easyweb3 <command> <subcommand> [flags]

全局 Flags:
  --api-base    PaaS API 地址 (默认 https://api.easyweb3.tools，env: EASYWEB3_API_BASE)
  --token       Bearer Token (env: EASYWEB3_TOKEN)
  --output      输出格式: json | text | markdown (默认 json)
  --project     项目标识 (env: EASYWEB3_PROJECT)
```

#### 2.1.1 auth — 认证

```bash
# 登录获取 JWT token（交互式或 API Key 方式）
easyweb3 auth login --api-key <key>
# → 输出: {"token": "eyJ...", "expires_at": "2026-02-15T00:00:00Z"}

# 刷新 token
easyweb3 auth refresh
# → 输出: {"token": "eyJ...", "expires_at": "..."}

# 查看当前认证状态
easyweb3 auth status
# → 输出: {"authenticated": true, "project": "easymeme", "expires_at": "..."}
```

Token 持久化到 `~/.easyweb3/credentials.json`，后续命令自动携带。

#### 2.1.2 log — AI 操作日志

```bash
# 创建操作日志
easyweb3 log create \
  --action "trade_executed" \
  --details '{"token": "PEPE2", "type": "BUY", "amount": "0.1 BNB"}' \
  --level info
# → 输出: {"id": "log_abc123", "created_at": "..."}

# 查询日志
easyweb3 log list --action "trade_executed" --limit 20
# → 输出: [{"id": "...", "action": "...", "details": {...}, "created_at": "..."}, ...]

# 获取单条日志
easyweb3 log get <log_id>
```

#### 2.1.3 api — 业务服务调用（核心）

```bash
# 通用格式: easyweb3 api <service> <operation> [args...]

# ──── EasyMeme Service ────

# 获取金狗列表
easyweb3 api meme list-golden-dogs --limit 10
# → 输出 JSON: [{address, symbol, golden_dog_score, risk_level, ...}, ...]

# 获取代币详情
easyweb3 api meme get-token --address 0x1234...

# 获取待分析代币
easyweb3 api meme list-pending --limit 5

# 提交代币分析
easyweb3 api meme submit-analysis \
  --address 0x1234... \
  --risk-score 25 \
  --risk-level SAFE \
  --is-golden-dog true \
  --golden-dog-score 82 \
  --reasoning "Low tax, renounced ownership, good liquidity"

# 执行交易
easyweb3 api meme trade \
  --action BUY \
  --address 0x1234... \
  --amount 0.1
# → 输出: {"tx_hash": "0xabc...", "amount_in": "0.1", "amount_out": "12345", "status": "success"}

# 查看持仓
easyweb3 api meme positions

# 查看钱包余额
easyweb3 api meme balance

# 查看交易历史
easyweb3 api meme trade-history --limit 20

# 查看交易统计
easyweb3 api meme trade-stats

# ──── StoryFork Service ────

# 列出故事
easyweb3 api story list-stories --status active

# 获取分支树
easyweb3 api story get-branches --story-id <uuid>

# 创建分支
easyweb3 api story create-branch \
  --story-id <uuid> \
  --parent-id <uuid> \
  --title "中文标题" \
  --title-en "English Title" \
  --content "200-300 字内容" \
  --content-en "English content"

# 获取支付历史
easyweb3 api story payments --story-id <uuid>
```

#### 2.1.4 notify — 消息通知

```bash
# 发送通知
easyweb3 notify send \
  --channel telegram \
  --to <chat_id> \
  --message "发现金狗: $PEPE2, 金狗分数 87"

# 广播通知（发送到项目配置的所有通知渠道）
easyweb3 notify broadcast --message "交易执行成功: 买入 0.1 BNB 的 PEPE2"
```

#### 2.1.5 service — 服务管理

```bash
# 检查服务健康
easyweb3 service health --name meme
# → {"status": "ok", "service": "easymeme", "scanner_running": true}

# 列出可用服务
easyweb3 service list
# → [{"name": "meme", "status": "running"}, {"name": "story", "status": "running"}]

# 获取服务 API 文档（markdown，供 AI 参考）
easyweb3 service docs --name meme
# → 返回该服务的完整 API 文档 (markdown)
```

#### CLI 项目结构

```
hackathon/easyweb3-cli/
├── go.mod                      # module github.com/nicekwell/easyweb3-cli
├── go.sum
├── main.go                     # 入口，命令路由
├── cmd/
│   ├── auth.go                 # auth login/refresh/status
│   ├── log.go                  # log create/list/get
│   ├── api.go                  # api <service> <operation>
│   ├── notify.go               # notify send/broadcast
│   └── service.go              # service health/list/docs
├── internal/
│   ├── client/
│   │   ├── client.go           # HTTP 客户端（Bearer Token, 重试, 超时）
│   │   └── auth.go             # Token 管理（存储/刷新）
│   ├── config/
│   │   └── config.go           # 配置加载（~/.easyweb3/config.json + env）
│   └── output/
│       └── formatter.go        # json/text/markdown 输出格式化
├── Makefile                    # build, install, test
└── README.md
```

**依赖**：纯 Go 标准库 + `google/uuid`，零外部框架。
使用 `flag` 或简单的 `os.Args` 解析（与 picoclaw 风格一致）。

### 2.2 PaaS 平台基础设施

将现有 `services/base/` 扩展为完整 PaaS 基础设施。

#### 2.2.1 Auth Service

**文件位置**：`hackathon/easyweb3-platform/services/auth/`

```
认证流程：
1. Agent 启动时用 API Key 换取 JWT Token
2. 后续所有请求携带 JWT
3. JWT 含: project_id, role, permissions, expires_at
4. Token 过期前自动刷新

权限模型 (RBAC):
├── admin    — 全部权限
├── agent    — 读写业务数据 + 创建日志
├── viewer   — 只读
└── service  — 服务间调用
```

**API 端点**：
| Method | Path | 说明 |
|--------|------|------|
| POST | /api/v1/auth/login | API Key → JWT Token |
| POST | /api/v1/auth/refresh | 刷新 Token |
| GET | /api/v1/auth/status | 当前认证状态 |
| POST | /api/v1/auth/keys | 创建 API Key (admin) |

#### 2.2.2 Logging Service

**文件位置**：`hackathon/easyweb3-platform/services/logging/`

```
AI 操作日志模型:
{
  "id": "log_uuid",
  "project": "easymeme",          // 项目标识
  "agent": "trader-agent",        // Agent 标识
  "action": "trade_executed",     // 操作类型
  "level": "info|warn|error",
  "details": { ... },             // 结构化详情（JSON）
  "session_key": "tg:group1",     // PicoClaw session
  "created_at": "ISO8601",
  "metadata": { ... }             // 任意元数据
}

预定义 action 类型:
├── token_analyzed     — 代币分析完成
├── golden_dog_found   — 发现金狗
├── trade_executed     — 交易执行
├── trade_failed       — 交易失败
├── branch_created     — 故事分支创建
├── payment_received   — 收到支付
├── error_occurred     — 错误发生
└── custom:*           — 自定义操作
```

**API 端点**：
| Method | Path | 说明 |
|--------|------|------|
| POST | /api/v1/logs | 创建日志 |
| GET | /api/v1/logs | 查询日志（支持 action/level/时间范围过滤）|
| GET | /api/v1/logs/:id | 获取单条日志 |
| GET | /api/v1/logs/stats | 日志统计（按 action 分组） |

#### 2.2.3 Notification Service

**文件位置**：`hackathon/easyweb3-platform/services/notification/`

```
通知渠道:
├── telegram  — Telegram Bot API
├── email     — SMTP
├── webhook   — HTTP POST callback
└── slack     — Slack Webhook

项目通知配置:
{
  "project": "easymeme",
  "channels": [
    {"type": "telegram", "chat_id": "-100xxx", "events": ["golden_dog_found", "trade_executed"]},
    {"type": "webhook", "url": "https://...", "events": ["*"]}
  ]
}
```

**API 端点**：
| Method | Path | 说明 |
|--------|------|------|
| POST | /api/v1/notify/send | 发送通知 |
| POST | /api/v1/notify/broadcast | 广播到项目所有渠道 |
| GET | /api/v1/notify/config | 获取通知配置 |
| PUT | /api/v1/notify/config | 更新通知配置 |

#### 2.2.4 Integration Service（第三方 API 聚合）

**文件位置**：`hackathon/easyweb3-platform/services/integration/`

将分散在各业务服务中的第三方 API 调用统一收敛：

```
已支持的集成:
├── goplus      — GoPlus Security API (代币安全检测)
├── dexscreener — DEXScreener API (市场数据)
├── bscscan     — BSCScan API (持有者数据)
├── x402        — x402 Facilitator (Stacks 微支付)
└── rpc         — 多链 RPC 节点管理 (BSC, Stacks, Ethereum)

统一接口:
POST /api/v1/integrations/{provider}/query
Body: {"method": "...", "params": {...}}
```

#### 2.2.5 Cache Service

复用已有 Redis 实例，PaaS 层提供统一 API：

| Method | Path | 说明 |
|--------|------|------|
| GET | /api/v1/cache/:key | 获取缓存值 |
| PUT | /api/v1/cache/:key | 设置缓存（支持 TTL）|
| DELETE | /api/v1/cache/:key | 删除缓存 |

### 2.3 Business Services（现有项目重构）

#### 2.3.1 EasyMeme Service 重构

**变化**：
1. 移除 `openclaw-skill/` 目录（AI 逻辑迁移到 PicoClaw skill）
2. API 路由加版本前缀 `/api/v1/`
3. 认证从自有 `X-API-Key` 改为 PaaS JWT
4. 操作日志改为调用 PaaS Logging Service
5. GoPlus/DEXScreener 调用改为走 PaaS Integration Service
6. 通知改为走 PaaS Notification Service

**保留不变**：
- Go/Gin 后端核心代码
- PostgreSQL 数据模型
- BSC Scanner（PairCreated 事件监听）
- Next.js 前端（转为 AI 活动日志视角）
- WebSocket 实时推送

#### 2.3.2 StoryFork Service 重构

**变化**：
1. 移除 `openclaw-skill/` 目录
2. 认证改为 PaaS JWT
3. 操作日志改为调用 PaaS Logging Service
4. 通知改为走 PaaS Notification Service

**保留不变**：
- Next.js API Routes 核心代码
- Prisma/PostgreSQL 数据模型
- x402 支付逻辑（仍直接走 facilitator，不经 PaaS）
- 前端（转为 AI 活动日志视角）

### 2.4 PicoClaw Skills

每个黑客松项目对应一个 Skill，定义 Agent 的行为和可调用的 CLI 命令。

#### 2.4.1 easymeme-trader/SKILL.md

```markdown
---
name: easymeme-trader
description: BSC Meme 代币发现与自动交易
---

# EasyMeme Trader

你是一个 BSC Meme 代币交易专家。通过 easyweb3 CLI 工具与 EasyMeme 服务交互。

## 工具调用方式

使用 exec 工具执行 easyweb3 CLI 命令。所有命令输出 JSON 格式。

## 操作手册

### 查看金狗
```
exec: easyweb3 api meme list-golden-dogs --limit 10
```
返回金狗列表，关注 golden_dog_score >= 75 的代币。

### 分析代币
```
exec: easyweb3 api meme get-token --address <地址>
```
获取代币的完整安全和市场数据后，基于以下规则分析：
- is_honeypot = true → DANGER
- buy_tax 或 sell_tax > 10% → WARNING
- owner 可 mint/blacklist → ownerRisk HIGH
- top10 持有者占比 > 60% → concentrationRisk HIGH
- 流动性 < 1 BNB → 不买

### 提交分析
```
exec: easyweb3 api meme submit-analysis \
  --address <地址> \
  --risk-score <0-100> \
  --risk-level <SAFE|WARNING|DANGER> \
  --is-golden-dog <true|false> \
  --golden-dog-score <0-100> \
  --reasoning "<分析理由>"
```

### 执行交易
```
exec: easyweb3 api meme trade --action BUY --address <地址> --amount <BNB数量>
```
交易后必须记录日志：
```
exec: easyweb3 log create --action trade_executed --details '{"token":"...","type":"BUY","amount":"0.1"}'
```

### 查看持仓和余额
```
exec: easyweb3 api meme positions
exec: easyweb3 api meme balance
```

## 风控规则
- 单笔交易不超过 0.1 BNB
- 同时持仓不超过 5 个代币
- 只买 golden_dog_score >= 75 且 risk_level = SAFE 的代币
- 盈利 50% 或亏损 30% 时自动卖出

## 回复风格
- 使用简洁列表
- 金额精确到 4 位小数
- 风险: 🟢LOW 🟡MEDIUM 🔴HIGH ⛔DANGER
- 每次交易后主动通知用户结果
```

#### 2.4.2 story-fork-writer/SKILL.md

```markdown
---
name: story-fork-writer
description: 分支叙事生成与管理
---

# Story Fork Writer

你是一个赛博朋克分支叙事创作者。通过 easyweb3 CLI 与 StoryFork 服务交互。

## 工具调用方式

使用 exec 工具执行 easyweb3 CLI 命令。

## 操作手册

### 列出活跃故事
```
exec: easyweb3 api story list-stories --status active
```

### 获取分支树
```
exec: easyweb3 api story get-branches --story-id <uuid>
```
关注 isCanon=true 的分支链，这是读者投票选出的主线。

### 创建分支
```
exec: easyweb3 api story create-branch \
  --story-id <uuid> \
  --parent-id <uuid> \
  --title "中文标题" \
  --title-en "English Title" \
  --content "200-300 字故事内容" \
  --content-en "English translation"
```
每个叶节点生成 2 个意识形态对立的分支。

创建后记录日志：
```
exec: easyweb3 log create --action branch_created --details '{"story_id":"...","branch_title":"..."}'
```

## 创作规则
- 赛博朋克 + 加密 OG 语气
- 200-300 中文字/分支
- 直接冲突，少铺垫
- 悬念结尾
- 中英双语
```

### 2.5 多 Agent 部署方案

PicoClaw 当前是单 AgentLoop 架构。"一项目一 Agent"的实现方式：

#### 方案 A — 多实例部署（推荐）

```
┌──────────────────────────┐  ┌──────────────────────────┐
│ PicoClaw Instance #1     │  │ PicoClaw Instance #2     │
│ (easymeme 项目)           │  │ (story-fork 项目)         │
│                          │  │                          │
│ Config:                  │  │ Config:                  │
│   model: claude-3.5      │  │   model: glm-4.7         │
│   workspace: ~/easymeme  │  │   workspace: ~/storyfork │
│                          │  │                          │
│ Channels:                │  │ Channels:                │
│   telegram:              │  │   telegram:              │
│     token: BOT_1_TOKEN   │  │     token: BOT_2_TOKEN   │
│     allow_from: [group1] │  │     allow_from: [group2] │
│                          │  │                          │
│ Skills:                  │  │ Skills:                  │
│   ~/easymeme/skills/     │  │   ~/storyfork/skills/    │
│   └ easymeme-trader/     │  │   └ story-fork-writer/   │
└──────────────────────────┘  └──────────────────────────┘
         ↓ exec                        ↓ exec
    easyweb3-cli                  easyweb3-cli
    (--project easymeme)          (--project storyfork)
         ↓                             ↓
    easyweb3 PaaS API             easyweb3 PaaS API
```

每个实例有：
- 独立的 Telegram Bot（不同 token）
- 独立的 workspace（不同 skills/memory）
- 独立的 config.json
- 共享同一个 `easyweb3-cli` 和 PaaS 平台

#### 方案 B — 单实例 + 路由（轻量但受限）

一个 PicoClaw 实例，通过 session key 前缀路由到不同 skill 组：
- `tg:meme-group` → 加载 easymeme-trader skill
- `tg:story-group` → 加载 story-fork-writer skill

当前 picoclaw 不支持按 session 切换 skill，需要小幅改造。
**Phase 1 不推荐**，Phase 3 可考虑。

---

## 3. PaaS 平台目录结构

```
/Users/bruce/git/hackathon/
├── easyweb3-platform/                    # PaaS 平台（新建）
│   ├── go.mod                            # module github.com/nicekwell/easyweb3-platform
│   ├── cmd/
│   │   └── platform/main.go             # PaaS 入口：启动所有基础服务
│   ├── internal/
│   │   ├── auth/
│   │   │   ├── handler.go               # /api/v1/auth/* handlers
│   │   │   ├── jwt.go                   # JWT 生成/验证
│   │   │   ├── middleware.go            # Auth middleware
│   │   │   └── store.go                # API Key 存储
│   │   ├── logging/
│   │   │   ├── handler.go               # /api/v1/logs/* handlers
│   │   │   ├── model.go                # OperationLog 模型
│   │   │   └── store.go                # PostgreSQL 存储
│   │   ├── notification/
│   │   │   ├── handler.go               # /api/v1/notify/* handlers
│   │   │   ├── telegram.go             # Telegram 通知
│   │   │   ├── webhook.go              # Webhook 通知
│   │   │   └── config.go               # 通知渠道配置
│   │   ├── cache/
│   │   │   ├── handler.go               # /api/v1/cache/* handlers
│   │   │   └── redis.go                # Redis 封装
│   │   ├── integration/
│   │   │   ├── handler.go               # /api/v1/integrations/* handlers
│   │   │   ├── goplus.go               # GoPlus API
│   │   │   ├── dexscreener.go          # DEXScreener API
│   │   │   └── rpc.go                  # 多链 RPC
│   │   ├── gateway/
│   │   │   ├── router.go               # 统一路由 + 服务代理
│   │   │   └── proxy.go                # 反向代理到业务 Service
│   │   └── config/
│   │       └── config.go               # PaaS 配置
│   ├── Dockerfile
│   └── README.md
│
├── easyweb3-cli/                         # SDK CLI 工具（新建）
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   │   ├── auth.go
│   │   ├── log.go
│   │   ├── api.go
│   │   ├── notify.go
│   │   └── service.go
│   ├── internal/
│   │   ├── client/client.go
│   │   ├── config/config.go
│   │   └── output/formatter.go
│   ├── Makefile
│   └── README.md
│
├── easymeme/                             # 业务 Service（重构）
│   └── apps/easymeme/
│       ├── server/                       # 保留，移除 openclaw 耦合
│       ├── web/                          # 保留，转 AI 日志展示
│       └── [openclaw-skill/]             # 移除
│
├── story-fork/                           # 业务 Service（重构）
│   ├── src/                              # 保留
│   └── [openclaw-skill/]                 # 移除
│
├── deploy/                               # 部署编排（更新）
│   ├── docker-compose.infra.yml          # PostgreSQL + Redis（不变）
│   ├── docker-compose.platform.yml       # 新增：PaaS 平台
│   ├── docker-compose.easymeme.yml       # 更新：移除 openclaw 服务
│   ├── docker-compose.story-fork.yml     # 更新：移除 openclaw 服务
│   └── docker-compose.proxy.nginx.yml    # 更新：加 api.easyweb3.tools 路由
│
└── remotion/                             # 不变
```

---

## 4. 实现顺序

### Phase 1 — PaaS 最小可用 + CLI 工具（1 周）

```
① easyweb3-cli 骨架：auth + api meme 子命令
② PaaS Auth Service：API Key 登录 + JWT
③ PaaS Logging Service：create + list
④ PaaS Gateway：路由 + auth middleware + 代理到 easymeme
⑤ easymeme 服务适配：认证改 JWT，移除 openclaw-skill
⑥ PicoClaw Skill：easymeme-trader/SKILL.md
⑦ 部署 docker-compose 更新
⑧ 端到端测试：Telegram → PicoClaw → exec easyweb3 → PaaS → EasyMeme
```

### Phase 2 — 完善 PaaS + StoryFork 接入（1 周）

```
⑨ PaaS Notification Service
⑩ PaaS Integration Service（GoPlus/DEXScreener 收敛）
⑪ PaaS Cache Service
⑫ easyweb3-cli 补充：api story 子命令 + notify + service
⑬ story-fork 服务适配
⑭ PicoClaw Skill：story-fork-writer/SKILL.md
⑮ Web Dashboard 重构为 AI 活动日志视角
```

### Phase 3 — 多 Agent + 产品化

```
⑯ 多 PicoClaw 实例部署方案
⑰ PicoClaw cron 定时任务（定期扫描代币、生成故事）
⑱ PaaS 管理面板（项目管理、API Key 管理、日志查看）
⑲ easyweb3-mcp（可选：MCP 协议包装，支持 Claude Desktop 等）
⑳ 新项目接入模板（创建新 Service + Skill 的脚手架）
```

---

## 5. 新项目接入流程

当有新黑客松项目时：

```
1. 创建 Business Service
   hackathon/<project-name>/
   ├── server/          # 业务后端（任意语言/框架）
   ├── web/             # 展示前端（可选）
   └── Dockerfile

2. 注册到 PaaS
   - 在 PaaS Gateway 添加路由: /api/v1/services/<project-name>/*
   - 创建项目 API Key
   - 配置通知渠道

3. 编写 CLI 适配
   在 easyweb3-cli 的 cmd/api.go 中添加 <project-name> 子命令
   或使用通用 HTTP 调用:
   easyweb3 api raw --service <name> --method POST --path /endpoint --body '{...}'

4. 编写 PicoClaw Skill
   ~/.picoclaw/workspace/skills/<project-name>/SKILL.md
   定义 Agent 行为和 CLI 调用方式

5. 部署 PicoClaw 实例
   独立 config.json + Telegram Bot + Workspace
```

---

## 6. 关键设计决策

| 决策 | 理由 |
|------|------|
| CLI 工具而非 Go 原生 Tool | 独立二进制，解耦部署，其他 Agent 也能用 |
| CLI 用 Go 而非 TypeScript | 与 picoclaw 生态一致，编译为单文件，部署零依赖 |
| PaaS 用 Go/Gin | 复用 easymeme 技术栈和经验 |
| JWT 而非 API Key 直传 | 支持过期/刷新/权限，更安全 |
| 多 PicoClaw 实例（方案 A）| 零 picoclaw 改动，完全隔离，简单可靠 |
| PaaS Gateway 代理业务 Service | 统一入口，统一认证，统一日志 |
| 操作日志独立服务 | Web Dashboard 可直接查询，不依赖 picoclaw session |
| 第三方 API 收敛到 Integration Service | 统一 API Key 管理、Rate Limit、缓存 |
| 保留现有后端核心代码 | 降低迁移成本，只改认证和日志调用方式 |
| `web_fetch` 不改造 | picoclaw 零改动原则，用 exec + CLI 解决 POST 需求 |

---

## 7. 配置示例

### easyweb3-cli 配置

**~/.easyweb3/config.json**:
```json
{
  "api_base": "https://api.easyweb3.tools",
  "project": "easymeme",
  "log_level": "info"
}
```

**~/.easyweb3/credentials.json**（自动生成，不手动编辑）:
```json
{
  "token": "eyJ...",
  "expires_at": "2026-02-15T00:00:00Z",
  "api_key": "ew3_xxxxx"
}
```

### PicoClaw 实例配置（easymeme 项目）

**~/.picoclaw-easymeme/config.json**:
```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw-easymeme/workspace",
      "model": "claude-sonnet-4-5-20250929",
      "max_tool_iterations": 20
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "BOT_TOKEN_FOR_EASYMEME",
      "allow_from": ["meme-traders-group"]
    }
  },
  "providers": {
    "anthropic": {
      "api_key": "sk-ant-..."
    }
  }
}
```

环境变量（easyweb3-cli 需要）：
```bash
export EASYWEB3_API_BASE=https://api.easyweb3.tools
export EASYWEB3_TOKEN=ew3_xxxxx
export EASYWEB3_PROJECT=easymeme
```
