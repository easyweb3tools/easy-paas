"use client";

import { useCallback, useEffect, useState } from "react";

type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  meta?: {
    limit?: number;
    offset?: number;
    total?: number;
    has_next?: boolean;
  };
};

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

function apiUrl(path: string) {
  if (API_BASE) {
    return `${API_BASE}${path}`;
  }
  return path;
}

function formatNumber(value: string | number, digits = 1) {
  const num = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(num)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(num);
}

function formatTimestamp(value?: string | null) {
  if (!value) return "--";
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "--";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

type EventRow = {
  ID: string;
  Slug: string;
  Title: string;
  Active: boolean;
  Closed: boolean;
  EndTime?: string | null;
  ExternalUpdatedAt?: string | null;
  LastSeenAt?: string | null;
};

type MarketRow = {
  ID: string;
  EventID: string;
  Slug?: string | null;
  Question: string;
  Active: boolean;
  Closed: boolean;
  Liquidity?: string | null;
  Volume?: string | null;
  ExternalUpdatedAt?: string | null;
  LastSeenAt?: string | null;
};

function statusBadge(active: boolean, closed: boolean) {
  if (closed) return { label: "已关闭", color: "bg-slate-300 text-slate-700" };
  if (active) return { label: "活跃", color: "bg-emerald-500 text-white" };
  return { label: "暂停", color: "bg-amber-400 text-amber-950" };
}

export default function HomePage() {
  const [events, setEvents] = useState<EventRow[]>([]);
  const [markets, setMarkets] = useState<MarketRow[]>([]);
  const [eventMeta, setEventMeta] = useState<ApiResponse<EventRow[]>["meta"]>();
  const [marketMeta, setMarketMeta] = useState<ApiResponse<MarketRow[]>["meta"]>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchAll = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const [eventRes, marketRes] = await Promise.all([
        fetch(apiUrl(`/api/catalog/events?limit=20&order_by=external_updated_at`), { signal }),
        fetch(apiUrl(`/api/catalog/markets?limit=20&order_by=external_updated_at`), { signal }),
      ]);
      if (!eventRes.ok) throw new Error(`Events HTTP ${eventRes.status}`);
      if (!marketRes.ok) throw new Error(`Markets HTTP ${marketRes.status}`);
      const eventBody = (await eventRes.json()) as ApiResponse<EventRow[]>;
      const marketBody = (await marketRes.json()) as ApiResponse<MarketRow[]>;
      if (eventBody.code !== 0) throw new Error(eventBody.message || "事件请求失败");
      if (marketBody.code !== 0) throw new Error(marketBody.message || "市场请求失败");
      setEvents(eventBody.data || []);
      setMarkets(marketBody.data || []);
      setEventMeta(eventBody.meta);
      setMarketMeta(marketBody.meta);
    } catch (err: unknown) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      setError(err instanceof Error ? err.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void fetchAll(controller.signal);
    return () => controller.abort();
  }, [fetchAll]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">Polymarket Data Fetch</p>
            <h1 className="text-2xl font-semibold tracking-tight">Polymarket 数据抓取</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              实时展示抓取到的事件与市场概览，优先关注数据同步与抓取稳定性。
            </p>
          </div>
          <div className="flex items-center gap-3">
            <div className="rounded-full border border-[color:var(--border)] bg-[var(--surface-strong)] px-4 py-2 text-xs text-[var(--muted)]">
              数据来源：Polymarket Gamma / CLOB
            </div>
            <button
              type="button"
              onClick={() => void fetchAll()}
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium text-[var(--text)] transition hover:bg-[var(--surface-strong)]"
              disabled={loading}
            >
              {loading ? "刷新中..." : "刷新数据"}
            </button>
          </div>
        </div>
      </section>

      <section className="glass-panel relative overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="relative z-10 border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">事件列表</p>
              <p className="text-xs text-[var(--muted)]">最近更新的事件（按 external_updated_at 排序）。</p>
            </div>
            <div className="text-xs text-[var(--muted)]">
              共 {loading ? "--" : formatNumber(eventMeta?.total ?? events.length, 0)} 条
            </div>
          </div>
        </div>

        {error ? (
          <div className="px-6 py-6 text-sm text-red-500">加载失败：{error}</div>
        ) : loading ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">正在加载事件数据...</div>
        ) : events.length === 0 ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">暂无事件数据，请先运行同步。</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
                <tr>
                  <th className="px-6 py-3">事件</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">结束时间</th>
                  <th className="px-4 py-3">外部更新时间</th>
                  <th className="px-4 py-3">同步时间</th>
                </tr>
              </thead>
              <tbody>
                {events.map((item) => {
                  const badge = statusBadge(item.Active, item.Closed);
                  return (
                    <tr key={item.ID} className="border-t border-[color:var(--border)]">
                      <td className="px-6 py-4">
                        <div>
                          <p className="font-medium text-[var(--text)]">{item.Title}</p>
                          <p className="text-xs text-[var(--muted)]">{item.Slug}</p>
                        </div>
                      </td>
                      <td className="px-4 py-4">
                        <span className={`rounded-full px-2 py-1 text-xs font-medium ${badge.color}`}>
                          {badge.label}
                        </span>
                      </td>
                      <td className="px-4 py-4">{formatTimestamp(item.EndTime)}</td>
                      <td className="px-4 py-4">{formatTimestamp(item.ExternalUpdatedAt)}</td>
                      <td className="px-4 py-4">{formatTimestamp(item.LastSeenAt)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="glass-panel relative overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="relative z-10 border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">市场列表</p>
              <p className="text-xs text-[var(--muted)]">最近更新的市场（按 external_updated_at 排序）。</p>
            </div>
            <div className="text-xs text-[var(--muted)]">
              共 {loading ? "--" : formatNumber(marketMeta?.total ?? markets.length, 0)} 条
            </div>
          </div>
        </div>

        {error ? (
          <div className="px-6 py-6 text-sm text-red-500">加载失败：{error}</div>
        ) : loading ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">正在加载市场数据...</div>
        ) : markets.length === 0 ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">暂无市场数据，请先运行同步。</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
                <tr>
                  <th className="px-6 py-3">市场</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">流动性</th>
                  <th className="px-4 py-3">成交量</th>
                  <th className="px-4 py-3">外部更新时间</th>
                  <th className="px-4 py-3">同步时间</th>
                </tr>
              </thead>
              <tbody>
                {markets.map((item) => {
                  const badge = statusBadge(item.Active, item.Closed);
                  return (
                    <tr key={item.ID} className="border-t border-[color:var(--border)]">
                      <td className="px-6 py-4">
                        <div>
                          <p className="font-medium text-[var(--text)]">{item.Question}</p>
                          <p className="text-xs text-[var(--muted)]">
                            {item.Slug ?? "--"} · Event {item.EventID}
                          </p>
                        </div>
                      </td>
                      <td className="px-4 py-4">
                        <span className={`rounded-full px-2 py-1 text-xs font-medium ${badge.color}`}>
                          {badge.label}
                        </span>
                      </td>
                      <td className="px-4 py-4">{formatNumber(item.Liquidity ?? "--", 2)}</td>
                      <td className="px-4 py-4">{formatNumber(item.Volume ?? "--", 2)}</td>
                      <td className="px-4 py-4">{formatTimestamp(item.ExternalUpdatedAt)}</td>
                      <td className="px-4 py-4">{formatTimestamp(item.LastSeenAt)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
