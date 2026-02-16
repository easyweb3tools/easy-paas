"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import { apiGet, ApiResponse } from "@/lib/api";
import { DEFAULTS } from "@/lib/constants";

type ExecutionPlan = {
  ID: number;
  OpportunityID: number;
  Status: string;
  StrategyName: string;
  PlannedSizeUSD: string;
  CreatedAt?: string;
  UpdatedAt?: string;
};

function fmt(v: string | number, digits = 2) {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(n);
}

export default function ExecutionsPage() {
  const [items, setItems] = useState<ExecutionPlan[]>([]);
  const [meta, setMeta] = useState<ApiResponse<ExecutionPlan[]>["meta"]>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const loadingRef = useRef(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<ExecutionPlan[]>(`/api/v2/executions?limit=${DEFAULTS.OPPORTUNITY_PAGE_LIMIT}&order_by=created_at&order=desc`, {
        cache: "no-store",
        signal,
      });
      setItems(body.data ?? []);
      setMeta(body.meta);
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Executions</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              执行计划列表。点击进入详情可 preflight / 录入 fills / settle 计算 PnL。
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
        {error ? <div className="mt-4 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="glass-panel relative overflow-hidden rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="relative z-10 border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">列表</p>
              <p className="text-xs text-[var(--muted)]">total: {meta?.total ?? items.length}</p>
            </div>
          </div>
        </div>

        {loading ? (
          <div className="space-y-2 px-6 py-6">
            <div className="skeleton h-6 w-1/2" />
            <div className="skeleton h-16 w-full" />
            <div className="skeleton h-16 w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">No execution plans yet.</div>
        ) : (
          <>
          <div className="hidden overflow-x-auto md:block">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
                <tr>
                  <th className="px-6 py-3">id</th>
                  <th className="px-4 py-3">status</th>
                  <th className="px-4 py-3">strategy</th>
                  <th className="px-4 py-3">planned_usd</th>
                  <th className="px-4 py-3">opportunity</th>
                  <th className="px-4 py-3">open</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.ID} className="border-t border-[color:var(--border)]">
                    <td className="px-6 py-4 font-mono text-xs">{it.ID}</td>
                    <td className="px-4 py-4">{it.Status}</td>
                    <td className="px-4 py-4">{it.StrategyName}</td>
                    <td className="px-4 py-4">${fmt(it.PlannedSizeUSD)}</td>
                    <td className="px-4 py-4 font-mono text-xs">{it.OpportunityID}</td>
                    <td className="px-4 py-4">
                      <Link
                        href={`/v2/executions/${it.ID}`}
                        className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                      >
                        Detail
                      </Link>
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
                  <span className="text-xs">{it.Status}</span>
                </div>
                <div className="mt-1 text-xs text-[var(--muted)]">{it.StrategyName}</div>
                <div className="mt-1 text-xs">${fmt(it.PlannedSizeUSD)} · opp#{it.OpportunityID}</div>
                <Link
                  href={`/v2/executions/${it.ID}`}
                  className="mt-2 inline-flex rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs"
                >
                  Detail
                </Link>
              </div>
            ))}
          </div>
          </>
        )}
      </section>
    </div>
  );
}
