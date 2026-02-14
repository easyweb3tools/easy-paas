"use client";

import { useCallback, useEffect, useState } from "react";

import { apiGet } from "@/lib/api";

type Overview = {
  TotalPlans: number;
  TotalPnLUSD: number;
  AvgROI: number;
  WinCount: number;
  LossCount: number;
  PendingCount: number;
};

type ByStrategy = {
  StrategyName: string;
  Plans: number;
  TotalPnLUSD: number;
  AvgROI: number;
};

type Failure = {
  FailureReason: string;
  Count: number;
};

function fmt(v: number, digits = 2) {
  if (!Number.isFinite(v)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(v);
}

export default function AnalyticsPage() {
  const [overview, setOverview] = useState<Overview | null>(null);
  const [byStrategy, setByStrategy] = useState<ByStrategy[]>([]);
  const [failures, setFailures] = useState<Failure[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [o, s, f] = await Promise.all([
        apiGet<Overview>("/api/v2/analytics/overview", { cache: "no-store" }),
        apiGet<ByStrategy[]>("/api/v2/analytics/by-strategy", { cache: "no-store" }),
        apiGet<Failure[]>("/api/v2/analytics/failures", { cache: "no-store" }),
      ]);
      setOverview(o.data);
      setByStrategy(s.data ?? []);
      setFailures(f.data ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Analytics</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              基于 `pnl_records` 的统计（MVP）。
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

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Overview</p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-sm sm:grid-cols-3">
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">total_plans</div>
            <div className="mt-1 text-xl font-semibold">{overview?.TotalPlans ?? 0}</div>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">total_pnl_usd</div>
            <div className="mt-1 text-xl font-semibold">${fmt(overview?.TotalPnLUSD ?? 0)}</div>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">avg_roi</div>
            <div className="mt-1 text-xl font-semibold">{fmt((overview?.AvgROI ?? 0) * 100)}%</div>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">win</div>
            <div className="mt-1 text-xl font-semibold">{overview?.WinCount ?? 0}</div>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">loss</div>
            <div className="mt-1 text-xl font-semibold">{overview?.LossCount ?? 0}</div>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <div className="text-xs text-[var(--muted)]">pending</div>
            <div className="mt-1 text-xl font-semibold">{overview?.PendingCount ?? 0}</div>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">By Strategy</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">strategy</th>
                <th className="px-4 py-3">plans</th>
                <th className="px-4 py-3">total_pnl</th>
                <th className="px-4 py-3">avg_roi</th>
              </tr>
            </thead>
            <tbody>
              {byStrategy.map((r) => (
                <tr key={r.StrategyName} className="border-t border-[color:var(--border)]">
                  <td className="px-6 py-4">{r.StrategyName}</td>
                  <td className="px-4 py-4">{r.Plans}</td>
                  <td className="px-4 py-4">${fmt(r.TotalPnLUSD)}</td>
                  <td className="px-4 py-4">{fmt(r.AvgROI * 100)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Failures</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">reason</th>
                <th className="px-4 py-3">count</th>
              </tr>
            </thead>
            <tbody>
              {failures.map((r) => (
                <tr key={`${r.FailureReason}-${r.Count}`} className="border-t border-[color:var(--border)]">
                  <td className="px-6 py-4">{r.FailureReason || "(empty)"}</td>
                  <td className="px-4 py-4">{r.Count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

