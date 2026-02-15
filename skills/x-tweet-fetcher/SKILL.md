---
name: x-tweet-fetcher
description: Fetch X/Twitter tweet JSON (including long tweets, quote tweets, and article content) without login or API keys, via FxTwitter public API.
---

# X Tweet Fetcher

你负责在不登录、不使用 X API key 的前提下，抓取单条推文的结构化内容（文本、作者、统计、引用推文、长文/Article）。

本 skill 设计目标是让 OpenClaw 能稳定地“拉取事实”，再由你在对话中做摘要/引用/归档。

## 依赖

- `python3`（OpenClaw 容器镜像应包含）

## 输入

- 推文 URL（支持 `x.com` 或 `twitter.com`）：
  - `https://x.com/<username>/status/<tweet_id>`
  - `https://twitter.com/<username>/status/<tweet_id>`

## 核心做法（Python 脚本）

使用 vendored 脚本（来自 `ythx-101/x-tweet-fetcher`，基于 FxTwitter 公共 API）：

脚本路径（OpenClaw workspace 内）：

`/root/.openclaw/workspace/skills/x-tweet-fetcher/scripts/fetch_tweet.py`

## 工具调用方式（exec）

### 1) 拉取推文 JSON（推荐）

```bash
exec: python3 /root/.openclaw/workspace/skills/x-tweet-fetcher/scripts/fetch_tweet.py --url "<tweet_url>" --pretty
```

### 2) 只输出可读文本（可选）

```bash
exec: python3 /root/.openclaw/workspace/skills/x-tweet-fetcher/scripts/fetch_tweet.py --url "<tweet_url>" --text-only
```

### 2) 常见错误处理

- 如果返回的 JSON `code != 200` 或包含 `error` 字段：
  - 输出简短错误总结（tweet 不存在/私有/被删/FxTwitter 服务不可用）
  - 不要编造推文内容
  - 如果需要，可提示用户稍后重试或提供备用 URL

## 产出规范

当用户要“看内容”而不是“看原始 JSON”时，你应在回复里输出：
- 推文作者（`@screen_name`）与时间（如果有）
- 推文全文（包括长文 article 的 `full_text`，如存在）
- 关键统计（likes/retweets/views）
- 引用推文摘要（如存在，标注是 quote）

## 限制

- 不支持拉取回复线程内容（只能看到回复数）
- 受 FxTwitter 服务可用性影响
