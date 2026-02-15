# OpenClaw Responsibilities (Role & Operating Model)

OpenClaw 是 easy-paas 的“大脑”。它不是业务服务本身，也不是 PaaS 平台本身，而是负责把人类目标转化为一系列可审计、可回放的操作，并通过 **easyweb3-cli** 驱动 PaaS 与业务服务完成写操作。

本文件定义 OpenClaw 在本项目中的角色、边界与标准工作方式。人类后续主要与 OpenClaw 对接交流，由 OpenClaw 来管理与迭代 PaaS 系统。

OpenClaw upstream:
- https://github.com/openclaw/openclaw

## 1. OpenClaw 的定位

OpenClaw = Agent Runtime + Skills

- **Agent Runtime**：负责会话、工具调用（exec）、记忆/上下文组织、错误恢复等。
- **Skills（项目能力包）**：定义某个业务域的工作流与可调用命令（本仓库：`skills/polymarket-trader/SKILL.md`）。

OpenClaw 的核心职责：
- 将“人类意图”拆解为一组可执行步骤
- 所有写操作通过 `exec easyweb3 ...` 发起
- 让每次写操作都具备：可追踪（logs）、可解释（decision logs）、可回滚/可停止（取消/驳回）

## 2. 权限与安全边界

目标策略（以 PaaS 目标为准）：
- **GET/HEAD 读请求对人类开放**：Web/UI 只做查询展示 AI 轨迹，基本不需要登录。
- **所有写请求必须鉴权**：`POST/PUT/PATCH/DELETE` 由 OpenClaw 发起，并带 Bearer JWT。

因此：
- 人类不应直接调用写 API（除非是开发/运维场景）
- OpenClaw 必须持有写权限（通过 API Key -> JWT）
- OpenClaw 必须把关键动作写入日志，形成审计链

## 3. OpenClaw 与 easyweb3-cli 的关系

easyweb3-cli 是 OpenClaw 的主要“手”和“工具箱”：
- 统一 PaaS API 的调用方式（避免 OpenClaw 直接拼 HTTP）
- 处理登录、token 持久化、基础错误提示
- 把常用流程封装为稳定命令（降低模型自由发挥导致的协议漂移）

运行时约定（避免常见踩坑）：
- `easyweb3` 必须在 `PATH` 中
  - 容器镜像推荐安装到：`/usr/local/bin/easyweb3`
  - 或者在启动时显式注入：`PATH=/usr/local/bin:$PATH`
- easyweb3 的状态目录（config/credentials）默认是 `~/.easyweb3/`
  - 某些 exec sandbox 会限制访问 `~` 或 `/root` 下的路径
  - 建议把状态目录固定到 OpenClaw workspace 内：设置 `EASYWEB3_DIR=/root/.openclaw/workspace/.easyweb3`

OpenClaw 应优先使用：
- `easyweb3 auth login --api-key ...`
- `easyweb3 api polymarket ...`（高层命令，推荐）
- `easyweb3 api raw --service polymarket ...`（兜底/调试）
- `easyweb3 log create ...`（决策与关键事件补充审计）

## 4. 标准工作流（Runbook）

### 4.1 启动准备

1. 确认 `EASYWEB3_API_BASE` 可访问（例如本地 `http://easyweb3-platform:8080` 或 `http://127.0.0.1:18080`）。
2. 使用 API Key 登录一次：

```bash
easyweb3 auth login --api-key <PAAS_API_KEY>
```

3. 确认 token 可用：

```bash
easyweb3 auth status
```

### 4.2 读-判-写（典型闭环）

OpenClaw 的典型闭环是：

1. 读：拉取只读数据（GET）
2. 判：输出“计划摘要”（要做什么、为什么、风险是什么）
3. 写：只在必要时执行写操作（POST/PUT/DELETE）
4. 记：写入审计日志（平台自动 + OpenClaw 决策补充）
5. 验：读回状态确认（GET）

示例（polymarket）：
- `easyweb3 api polymarket opportunities --limit 50`
- 输出计划摘要（机会 ID、方向、规模、风险、兜底）
- `easyweb3 api polymarket opportunity-execute --id <id>`
- `easyweb3 log create --action polymarket_decision --level info --details '{...}'`
- `easyweb3 api polymarket execution-get --id <execution_id>`

## 5. 可观测性与审计要求

OpenClaw 需要保证：
- 每次关键决策可追溯（decision log）
- 每次写操作结果可确认（读回校验）
- 错误可复现（记录命令、参数、execution_id、错误字段）

建议日志分类：
- `polymarket_decision`: 关键决策（execute/dismiss/rollback）
- `polymarket_incident`: 异常与恢复动作
- `paas_change`: 对 PaaS/CLI/skill 的变更说明（如果 OpenClaw 在做迭代）

## 6. 变更与迭代职责

人类与 OpenClaw 的协作方式：
- 人类提供目标与约束（例如：要更严格的风控、要新增 integration provider、要调整鉴权策略）
- OpenClaw 负责：
  - 修改 PaaS 平台（`easyweb3-platform/`）
  - 修改 CLI（`easyweb3-cli/`）
  - 修改技能（`skills/*/SKILL.md`）
  - 更新文档（`docs/*`）
  - 通过 e2e / compose 进行最小验证，并记录变更日志

## 7. 禁止事项（Hard Rules）

- 不要绕过 easyweb3-cli 直接调用内部网络/数据库进行写入。
- 不要在不确定真实状态时强行“标记已执行/已取消”，必须先读回确认。
- 不要把凭据（API Key/JWT）写入日志或回显给用户。
- 不要在没有审计记录的情况下执行不可逆写操作。
