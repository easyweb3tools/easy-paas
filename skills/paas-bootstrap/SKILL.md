---
name: paas-bootstrap
description: 一键完成 注册+授权+登录+自检（通过 easyweb3-platform，适用于正式环境）
---

# PaaS Bootstrap

目标：在正式环境用 `openclaw-gateway` 完成以下流程，并输出可复用的排障信息。

- 注册一个用户（默认无权限）
- 用 OpenClaw 的 admin/agent 能力给该用户授予 `polymarket` 的 `viewer` 或 `agent`
- 用该用户登录拿到 JWT（可选）
- 自检：验证 `/api/catalog/*`、`/api/v2/*` 能正常访问

## 依赖

本 skill 只通过 `exec easyweb3 ...` 调用 PaaS API。

运行环境需要：

- `EASYWEB3_API_BASE`（在 compose 内通常是 `http://easyweb3-platform:8080`）
- `EASYWEB3_BOOTSTRAP_API_KEY`（openclaw-gateway 的 admin/agent key）

> 提示：如果 `openclaw-entrypoint.sh` 检测到 `EASYWEB3_BOOTSTRAP_API_KEY`，会自动执行一次 `easyweb3 auth login --api-key ...` 并持久化 token。

## 参数（你可以先按默认值执行）

- 用户名：`demo_viewer`
- 密码：`demo_pass_change_me`
- 项目：`polymarket`
- 角色：`viewer`

## 流程

### 1) 管理员登录（用于授权）

```
exec: easyweb3 auth login --api-key ${EASYWEB3_BOOTSTRAP_API_KEY}
```

### 2) 注册用户（公开接口）

```
exec: easyweb3 auth register --username demo_viewer --password demo_pass_change_me
```

如果用户已存在，允许忽略错误继续执行下一步。

### 3) 给用户授权（viewer 只读；agent 可写）

```
exec: easyweb3 auth grant --user demo_viewer --project-id polymarket --role viewer
```

### 4) 用该用户登录（可选，用于拿到用户侧 JWT）

```
exec: easyweb3 auth login --username demo_viewer --password demo_pass_change_me --project-id polymarket
```

### 5) 自检（不依赖浏览器）

健康检查：

```
exec: easyweb3 api raw --service polymarket --method GET --path /healthz
```

目录查询：

```
exec: easyweb3 api polymarket catalog-events --limit 1
exec: easyweb3 api polymarket catalog-markets --limit 1
```

V2 查询：

```
exec: easyweb3 api polymarket strategies
exec: easyweb3 api polymarket opportunities --limit 1
exec: easyweb3 api polymarket executions --limit 1
exec: easyweb3 api polymarket analytics-overview
```

## 期望结果

- 第 2 步成功后：用户存在，但没有 grants（这是预期）
- 第 3 步成功后：用户能用 project `polymarket` 登录
- 第 5 步全部返回 `code: 0`（或对应 JSON data）

