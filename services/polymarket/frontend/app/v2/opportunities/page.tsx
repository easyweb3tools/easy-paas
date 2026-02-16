"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { apiGet, apiPost, ApiResponse } from "@/lib/api";
import { DEFAULTS } from "@/lib/constants";

type OpportunityLeg = {
  token_id: string;
  direction: "BUY_YES" | "BUY_NO" | "SELL_YES" | "SELL_NO";
  target_price?: number;
  size_usd?: number;
};

type Strategy = {
  Name: string;
  DisplayName: string;
  Category: string;
  Enabled: boolean;
  Priority: number;
};

type Opportunity = {
  ID: number;
  Status: string;
  StrategyID: number;
  Strategy?: Strategy;
  PrimaryMarketID?: string | null;
  EdgePct: string;
  EdgeUSD: string;
  MaxSize: string;
  Confidence: number;
  RiskScore: number;
  Reasoning: string;
  Legs: OpportunityLeg[];
  CreatedAt?: string;
};

function fmt(v: string | number, digits = 3) {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(n);
}

function pct(v: string) {
  const n = Number(v);
  if (!Number.isFinite(n)) return "--";
  return `${(n * 100).toFixed(2)}%`;
}

export default function OpportunitiesPage() {
  const [items, setItems] = useState<Opportunity[]>([]);
  const [meta, setMeta] = useState<ApiResponse<Opportunity[]>["meta"]>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState("active");
  const [strategy, setStrategy] = useState("");
  const [category, setCategory] = useState("");
  const [minEdge, setMinEdge] = useState<string>(DEFAULTS.MIN_EDGE);
  const [minConfidence, setMinConfidence] = useState("");
  const loadingRef = useRef(false);

  const query = useMemo(() => {
    const sp = new URLSearchParams();
    if (status) sp.set("status", status);
    if (strategy) sp.set("strategy", strategy);
    if (category) sp.set("category", category);
    if (minEdge) sp.set("min_edge", minEdge);
    if (minConfidence) sp.set("min_confidence", minConfidence);
    sp.set("sort_by", "edge_usd");
    sp.set("order", "desc");
    sp.set("limit", String(DEFAULTS.OPPORTUNITY_PAGE_LIMIT));
    sp.set("offset", "0");
    return sp.toString();
  }, [status, strategy, category, minEdge, minConfidence]);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<Opportunity[]>(`/api/v2/opportunities?${query}`, { cache: "no-store", signal });
      setItems(body.data ?? []);
      setMeta(body.meta);
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  async function dismiss(id: number) {
    try {
      await apiPost(`/api/v2/opportunities/${id}/dismiss`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function execute(id: number) {
    try {
      await apiPost(`/api/v2/opportunities/${id}/execute`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Opportunities</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              来自多策略引擎的活跃机会，支持一键生成执行计划。
            </p>
          </div>
          <button
            type="button"
            onClick={() => void refresh()}
            className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium text-[var(--text)] transition hover:bg-[var(--surface-strong)]"
            disabled={loading}
          >
            {loading ? "刷新中..." : "刷新"}
          </button>
        </div>
        <div className="mt-4 grid grid-cols-2 gap-3 text-xs sm:grid-cols-6">
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">status</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">strategy</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={strategy}
              onChange={(e) => setStrategy(e.target.value)}
              placeholder="arb_sum"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">category</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              placeholder="arbitrage"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">min_edge</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={minEdge}
              onChange={(e) => setMinEdge(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">min_confidence</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={minConfidence}
              onChange={(e) => setMinConfidence(e.target.value)}
              placeholder="0.6"
            />
          </label>
          <div className="flex flex-col justify-end">
            <div className="text-[var(--muted)]">
              total: {meta?.total ?? items.length}
            </div>
          </div>
        </div>
      </section>

      <section className="glass-panel relative overflow-hidden rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="relative z-10 border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">列表</p>
              <p className="text-xs text-[var(--muted)]">按 edge_usd 降序。</p>
            </div>
            {error ? <div className="text-xs text-red-500">{error}</div> : null}
          </div>
        </div>

        {loading ? (
          <div className="space-y-2 px-6 py-6">
            <div className="skeleton h-6 w-1/2" />
            <div className="skeleton h-16 w-full" />
            <div className="skeleton h-16 w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">No active opportunities. The strategy engine will scan automatically.</div>
        ) : (
          <>
          <div className="hidden overflow-x-auto md:block">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
                <tr>
                  <th className="px-6 py-3">id ↑↓</th>
                  <th className="px-4 py-3">strategy</th>
                  <th className="px-4 py-3">edge ↑↓</th>
                  <th className="px-4 py-3">max_size</th>
                  <th className="px-4 py-3">confidence</th>
                  <th className="px-4 py-3">risk</th>
                  <th className="px-4 py-3">reasoning</th>
                  <th className="px-4 py-3">actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.ID} className="border-t border-[color:var(--border)] align-top">
                    <td className="px-6 py-4 font-mono text-xs">{it.ID}</td>
                    <td className="px-4 py-4">
                      <div className="text-sm font-medium">{it.Strategy?.Name ?? "--"}</div>
                      <div className="text-xs text-[var(--muted)]">{it.Strategy?.Category ?? ""}</div>
                    </td>
                    <td className="px-4 py-4">
                      <div className="text-sm font-semibold">${fmt(it.EdgeUSD, 2)}</div>
                      <div className="text-xs text-[var(--muted)]">{pct(it.EdgePct)}</div>
                    </td>
                    <td className="px-4 py-4">${fmt(it.MaxSize, 2)}</td>
                    <td className="px-4 py-4">{fmt(it.Confidence, 2)}</td>
                    <td className="px-4 py-4">{fmt(it.RiskScore, 2)}</td>
                    <td className="px-4 py-4 max-w-[520px]">
                      <div className="text-xs text-[var(--muted)] whitespace-pre-wrap">{it.Reasoning}</div>
                      <details className="mt-2">
                        <summary className="cursor-pointer text-xs text-[var(--text)]">legs</summary>
                        <pre className="mt-2 overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
                          {JSON.stringify(it.Legs, null, 2)}
                        </pre>
                      </details>
                    </td>
                    <td className="px-4 py-4">
                      <div className="flex flex-col gap-2">
                        <button
                          className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                          onClick={() => void execute(it.ID)}
                        >
                          Execute
                        </button>
                        <button
                          className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                          onClick={() => void dismiss(it.ID)}
                        >
                          Dismiss
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="space-y-3 p-4 md:hidden">
            {items.map((it) => (
              <div key={`m-${it.ID}`} className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-xs">#{it.ID}</span>
                  <span className="text-xs">{it.Strategy?.Name ?? "--"}</span>
                </div>
                <div className="mt-2 text-xs text-[var(--muted)]">{it.Reasoning}</div>
                <div className="mt-2 text-sm font-semibold text-profit">${fmt(it.EdgeUSD, 2)}</div>
                <div className="mt-1 text-xs text-[var(--muted)]">{pct(it.EdgePct)} · conf {fmt(it.Confidence, 2)}</div>
                <div className="mt-3 flex gap-2">
                  <button
                    className="rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs"
                    onClick={() => void execute(it.ID)}
                  >
                    Execute
                  </button>
                  <button
                    className="rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs"
                    onClick={() => void dismiss(it.ID)}
                  >
                    Dismiss
                  </button>
                </div>
              </div>
            ))}
          </div>
          </>
        )}
      </section>
    </div>
  );
}
