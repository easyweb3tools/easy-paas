# easy-paas (easyweb3) Architecture

easy-paas 的目标是建立一个 **AI Agent 友好的 PaaS 平台**：

- **API 面向 AI**：以确定性、可脚本化、可审计为第一优先级（CLI 优先于 Web UI）。
- **Web 面向人类**：主要用于查询与展示 AI 行为轨迹（Logs/只读数据），基本不需要注册/登录。
- **写操作只能由 OpenClaw 发起**：增删改（POST/PUT/PATCH/DELETE）全部需要鉴权，确保可控与可审计。

本仓库当前落地的业务服务为：`polymarket`（见 `services/polymarket`）。

## 1. Core Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Human (Read Only)                         │
│  Web UI / Dashboard / curl                                       │
│  - Mostly reads: logs, public GET routes                          │
└─────────────────────────────┬───────────────────────────────────┘
                              │ HTTP (mostly GET, public)
                              v
┌─────────────────────────────────────────────────────────────────┐
│                        easyweb3-platform                         │
│  - Gateway + Auth + Logs + Notify + Cache + Integrations         │
│  - Proxies business services under /api/v1/services/{name}/*     │
│                                                                   │
│  Public GET/HEAD: open by default                                 │
│  Write methods: Bearer JWT required (agent/admin)                 │
└─────────────────────────────┬───────────────────────────────────┘
                              │ reverse proxy
                              v
┌─────────────────────────────────────────────────────────────────┐
│                      Business Services (Upstream)                 │
│  - polymarket backend/frontend                                    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     OpenClaw (The Brain)                          │
│  - one skill per project/service (this repo: polymarket-trader)   │
│  - executes: easyweb3-cli via exec                                 │
│  - all write operations originate from OpenClaw                    │
└─────────────────────────────┬───────────────────────────────────┘
                              │ exec: easyweb3 ...
                              v
┌─────────────────────────────────────────────────────────────────┐
│                        easyweb3-cli (SDK)                          │
│  - deterministic command interface for AI                          │
│  - handles auth/login, token persistence, request formatting        │
└─────────────────────────────┬───────────────────────────────────┘
                              │ HTTP (Bearer token on writes)
                              v
                        easyweb3-platform
```

## 2. Auth & Access Policy (Updated Goal)

### 2.1 Principle

- **GET/HEAD 请求默认全部放开**：便于人类直接打开 Web、直接查询只读数据。
- **写请求必须鉴权**：`POST/PUT/PATCH/DELETE` 必须带 Bearer JWT，且要求角色为 `agent/admin`。
- 平台不以“人类账号体系”作为核心能力：Web 不依赖注册登录；AI 通过 API Key 或账号（可选）获得写权限。

### 2.2 Roles (RBAC)

- `admin`: 全权限（创建 API Key、授权等）。
- `agent`: 允许发起写操作、记录日志、调用通知等。
- `viewer`: 只读（如果未来需要对 GET 做细分时使用；当前目标是 GET 全开放）。

### 2.3 Token Flow (AI)

1. OpenClaw（或服务端组件）使用 API Key 换取 JWT：
   - `POST /api/v1/auth/login` with `{"api_key":"..."}`
2. easyweb3-cli 把 token 存到 `~/.easyweb3/credentials.json`，后续写操作自动携带。

## 3. Components

### 3.1 easyweb3-platform (PaaS)

代码位置：`easyweb3-platform/`

职责：
- 提供统一的 PaaS API（auth/logs/notify/cache/integrations）
- 作为 **Gateway**：将业务服务统一暴露在 `/api/v1/services/{name}/*`
- 将鉴权策略集中在网关层，业务服务可通过网关头部获取上下文（例如 `X-Easyweb3-Project`, `X-Easyweb3-Role`）

关键端点（当前实现）：
- Auth:
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh` (write; requires Bearer)
  - `GET /api/v1/auth/status` (public)
  - `POST /api/v1/auth/keys` (admin write)
  - `POST /api/v1/auth/register` / `POST /api/v1/auth/grants` / `GET /api/v1/auth/users`（可选能力，更多用于内部/测试）
- Logs:
  - `POST /api/v1/logs` (write)
  - `GET /api/v1/logs` (public read in future; currently gated by role in code)
  - `GET /api/v1/logs/:id` (read)
  - `GET /api/v1/logs/stats` (read)
- Notify:
  - `POST /api/v1/notify/send` (write)
  - `POST /api/v1/notify/broadcast` (write)
  - `GET/PUT /api/v1/notify/config` (read/write)
- Cache:
  - `GET/PUT/DELETE /api/v1/cache/:key` (read/write)
- Integrations:
  - `POST /api/v1/integrations/{provider}/query`
  - providers: `dexscreener`, `goplus`
- Service registry:
  - `GET /api/v1/service/list`
  - `GET /api/v1/service/health?name=polymarket`
  - `GET /api/v1/service/docs?name=polymarket`
- Gateway proxy:
  - `/api/v1/services/{name}/*` -> upstream service

存储策略（当前实现）：
- PaaS 自身的数据（API keys/users/logs/notify config）目前使用文件落盘（MVP 方案）。
- 后续可替换为 PostgreSQL 等更可靠存储，不改变外部 API。

### 3.2 easyweb3-cli (AI SDK / Exec Tool)

代码位置：`easyweb3-cli/`

定位：让 OpenClaw 通过 `exec` 调用一个稳定、可复现的命令接口，避免 agent 直接拼 HTTP、直接操作内部网络。

核心命令（当前实现）：
- `easyweb3 auth ...`
- `easyweb3 log ...`
- `easyweb3 notify ...`
- `easyweb3 integrations query ...`
- `easyweb3 cache ...`
- `easyweb3 api raw --service polymarket ...`（通过网关访问业务服务）
- `easyweb3 api polymarket ...`（面向 polymarket 的高层命令封装）

目标（接下来要继续完善）：
- easyweb3-cli 成为 OpenClaw 的主要工具：把常用读写流程封装成确定的子命令，减少“自由形 HTTP”的概率。

### 3.3 Business Service: polymarket

代码位置：
- 后端：`services/polymarket/backend`
- 前端：`services/polymarket/frontend`

接入方式：
- 在 `services/services.local.json` 注册：
  - `polymarket.base_url` 指向 upstream backend
  - `health_path` / `docs_path` 可选
- 通过 PaaS 网关统一访问：
  - `/api/v1/services/polymarket/...`

服务侧约定：
- upstream 后端可要求存在 `Authorization: Bearer ...`，并可要求“必须经由网关”头（例如 `X-Easyweb3-Project`）。
- upstream 不需要自己实现完整鉴权与权限模型，优先依赖网关集中处理。

## 4. Integrations Roadmap (Updated)

当前 Integrations 已接入：
- `dexscreener`（市场数据）
- `goplus`（安全检查）

希望新增接入：
- `polymarket` API 作为 Integration provider

动机：
- 让 AI 以“provider/method/params”的稳定模式调用 polymarket 的常用读接口或组合接口
- 在 PaaS 层统一做：缓存、超时、速率限制、结构化错误、字段裁剪与稳定输出
- 减少 agent 直接访问 `/api/v1/services/polymarket/...` 时的路径拼接与协议漂移成本

建议形态（示例）：
- `POST /api/v1/integrations/polymarket/query`
  - `{"method":"opportunities","params":{"limit":50}}`
  - `{"method":"catalog-sync","params":{"scope":"all"}}`

注：具体 method 列表以后以 easyweb3-cli 的 `easyweb3 api polymarket ...` 命令为事实来源，Integration 做底层聚合/加速。

CLI 侧推荐用更稳定的封装（避免手写 method/params）：
- `easyweb3 integrations polymarket opportunities --limit 50`
- `easyweb3 integrations polymarket catalog-events --limit 50`
- `easyweb3 integrations polymarket catalog-markets --limit 50`
- `easyweb3 integrations polymarket catalog-sync --scope all`

## 5. OpenClaw Skills

本仓库技能目录：`skills/`

当前示例：
- `skills/polymarket-trader/SKILL.md`

原则：
- “一服务一 skill”：skill 只使用 `exec easyweb3 ...` 与平台交互
- skill 不直接访问 service 的内部网络/数据库，不直接写文件存储业务状态（状态应落在服务或 PaaS）
- 关键决策可补充写入 `easyweb3 log create`，形成可审计轨迹

## 6. Deployment Notes

- 本地编排：`docker-compose.yml`
  - `easyweb3-platform`（PaaS）
  - `services/polymarket/backend` + Postgres
  - 可选：`nginx`（`--profile web`）统一入口
  - 可选：`openclaw-gateway`（`--profile openclaw`）

Nginx 路由（本地）：
- `/api/v1/*` -> easyweb3-platform
- `/` -> polymarket frontend（可选）
