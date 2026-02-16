"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { apiGet } from "@/lib/api";

type TabKey = "overview" | "performance" | "attribution";

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

type DailyStat = {
  StrategyName: string;
  Date: string;
  PnLUSD: string;
  CumulativePnL: string;
};

type Drawdown = {
  MaxDrawdownUSD: number;
  MaxDrawdownPct: number;
  DrawdownDurationDays: number;
  CurrentDrawdownUSD: number;
  PeakPnL: number;
  TroughPnL: number;
};

type Ratio = {
  SharpeRatio: number;
  SortinoRatio: number;
  WinRate: number;
  ProfitFactor: number;
  AvgWin: number;
  AvgLoss: number;
  Expectancy: number;
};

type Attribution = {
  EdgeContribution: number;
  SlippageCost: number;
  FeeCost: number;
  TimingValue: number;
  NetPnL: number;
};

function fmt(v: number, digits = 2) {
  if (!Number.isFinite(v)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(v);
}

function num(v: string | number | null | undefined, fallback = 0) {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function dateToRFC3339(date: string, end = false): string {
  if (!date) return "";
  return `${date}${end ? "T23:59:59Z" : "T00:00:00Z"}`;
}

function statRange(series: number[]) {
  if (series.length === 0) return { min: 0, max: 1 };
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  for (const n of series) {
    if (n < min) min = n;
    if (n > max) max = n;
  }
  if (min === max) max = min + 1;
  return { min, max };
}

export default function AnalyticsPage() {
  const [tab, setTab] = useState<TabKey>("overview");

  const [overview, setOverview] = useState<Overview | null>(null);
  const [byStrategy, setByStrategy] = useState<ByStrategy[]>([]);
  const [failures, setFailures] = useState<Failure[]>([]);

  const [daily, setDaily] = useState<DailyStat[]>([]);
  const [drawdown, setDrawdown] = useState<Drawdown | null>(null);
  const [ratios, setRatios] = useState<Ratio | null>(null);

  const [attrStrategy, setAttrStrategy] = useState("");
  const [attribution, setAttribution] = useState<Attribution | null>(null);

  const [sinceDate, setSinceDate] = useState("");
  const [untilDate, setUntilDate] = useState("");

  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const loadingRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);

  const rangeQuery = useMemo(() => {
    const q = new URLSearchParams();
    const since = dateToRFC3339(sinceDate);
    const until = dateToRFC3339(untilDate, true);
    if (since) q.set("since", since);
    if (until) q.set("until", until);
    return q.toString();
  }, [sinceDate, untilDate]);

  const refresh = useCallback(async () => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const signal = controller.signal;
    setLoading(true);
    setError(null);
    try {
      const [o, s, f, d, dd, r] = await Promise.all([
        apiGet<Overview>("/api/v2/analytics/overview", { cache: "no-store", signal }),
        apiGet<ByStrategy[]>("/api/v2/analytics/by-strategy", { cache: "no-store", signal }),
        apiGet<Failure[]>("/api/v2/analytics/failures", { cache: "no-store", signal }),
        apiGet<DailyStat[]>(`/api/v2/analytics/daily?limit=365${rangeQuery ? `&${rangeQuery}` : ""}`, {
          cache: "no-store",
          signal,
        }),
        apiGet<Drawdown>(`/api/v2/analytics/drawdown${rangeQuery ? `?${rangeQuery}` : ""}`, { cache: "no-store", signal }),
        apiGet<Ratio>(`/api/v2/analytics/ratios${rangeQuery ? `?${rangeQuery}` : ""}`, { cache: "no-store", signal }),
      ]);
      setOverview(o.data);
      setByStrategy(s.data ?? []);
      setFailures(f.data ?? []);
      setDaily(d.data ?? []);
      setDrawdown(dd.data);
      setRatios(r.data);

      const defaultStrategy = attrStrategy || (s.data?.[0]?.StrategyName ?? "");
      setAttrStrategy(defaultStrategy);
      if (defaultStrategy) {
        const a = await apiGet<Attribution>(
          `/api/v2/analytics/strategy/${encodeURIComponent(defaultStrategy)}/attribution${rangeQuery ? `?${rangeQuery}` : ""}`,
          { cache: "no-store", signal }
        );
        setAttribution(a.data);
      } else {
        setAttribution(null);
      }
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [rangeQuery, attrStrategy]);

  useEffect(() => {
    void refresh();
    return () => abortRef.current?.abort();
  }, [refresh]);

  async function refreshAttribution(strategyName: string) {
    if (!strategyName) {
      setAttribution(null);
      return;
    }
    try {
      const signal = abortRef.current?.signal;
      const a = await apiGet<Attribution>(
        `/api/v2/analytics/strategy/${encodeURIComponent(strategyName)}/attribution${rangeQuery ? `?${rangeQuery}` : ""}`,
        { cache: "no-store", signal }
      );
      setAttribution(a.data);
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  const cumulativeSeries = useMemo(() => {
    const m = new Map<string, number>();
    for (const row of daily) {
      const key = row.Date.slice(0, 10);
      const value = m.get(key) ?? 0;
      m.set(key, value + num(row.PnLUSD));
    }
    const sorted = [...m.entries()].sort((a, b) => a[0].localeCompare(b[0]));
    const out: Array<{ date: string; cumulative: number }> = [];
    let acc = 0;
    for (const [date, pnl] of sorted) {
      acc += pnl;
      out.push({ date, cumulative: acc });
    }
    return out;
  }, [daily]);

  const curveRange = useMemo(() => statRange(cumulativeSeries.map((x) => x.cumulative)), [cumulativeSeries]);

  const drawdownCurve = useMemo(() => {
    let peak = Number.NEGATIVE_INFINITY;
    return cumulativeSeries.map((pt) => {
      peak = Math.max(peak, pt.cumulative);
      return { date: pt.date, dd: pt.cumulative - peak };
    });
  }, [cumulativeSeries]);
  const drawdownRange = useMemo(() => statRange(drawdownCurve.map((x) => x.dd)), [drawdownCurve]);

  const waterfall = useMemo(() => {
    if (!attribution) return [] as Array<{ label: string; value: number }>;
    return [
      { label: "Expected Edge", value: attribution.EdgeContribution },
      { label: "Slippage", value: -Math.abs(attribution.SlippageCost) },
      { label: "Fees", value: -Math.abs(attribution.FeeCost) },
      { label: "Timing", value: attribution.TimingValue },
      { label: "Net PnL", value: attribution.NetPnL },
    ];
  }, [attribution]);
  const wfMax = useMemo(() => {
    const m = Math.max(...waterfall.map((x) => Math.abs(x.value)), 1);
    return m;
  }, [waterfall]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Analytics</h1>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <input
              type="date"
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
              value={sinceDate}
              onChange={(e) => setSinceDate(e.target.value)}
            />
            <input
              type="date"
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
              value={untilDate}
              onChange={(e) => setUntilDate(e.target.value)}
            />
            <button
              type="button"
              onClick={() => void refresh()}
              className="rounded-full border border-[color:var(--border)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              disabled={loading}
            >
              {loading ? "刷新中..." : "刷新"}
            </button>
          </div>
        </div>
        <div className="mt-4 flex flex-wrap gap-2 text-xs">
          {(["overview", "performance", "attribution"] as TabKey[]).map((k) => (
            <button
              key={k}
              className={`rounded-full border px-3 py-1 ${
                tab === k
                  ? "border-[color:var(--text)] bg-[var(--surface-strong)]"
                  : "border-[color:var(--border)] hover:bg-[var(--surface-strong)]"
              }`}
              onClick={() => setTab(k)}
            >
              {k}
            </button>
          ))}
        </div>
        {error ? <div className="mt-4 text-sm text-red-500">{error}</div> : null}
      </section>

      {tab === "overview" ? (
        <>
          <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
            <div className="grid gap-3 px-6 py-5 text-sm sm:grid-cols-3">
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Total Plans</div>
                <div className="mt-1 text-xl font-semibold">{overview?.TotalPlans ?? 0}</div>
              </div>
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Total P&amp;L</div>
                <div className={(overview?.TotalPnLUSD ?? 0) >= 0 ? "mt-1 text-xl font-semibold text-profit" : "mt-1 text-xl font-semibold text-loss"}>
                  ${fmt(overview?.TotalPnLUSD ?? 0)}
                </div>
              </div>
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Avg ROI</div>
                <div className="mt-1 text-xl font-semibold">{fmt((overview?.AvgROI ?? 0) * 100)}%</div>
              </div>
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Wins</div>
                <div className="mt-1 text-xl font-semibold">{overview?.WinCount ?? 0}</div>
              </div>
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Losses</div>
                <div className="mt-1 text-xl font-semibold">{overview?.LossCount ?? 0}</div>
              </div>
              <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
                <div className="text-xs text-[var(--muted)]">Pending</div>
                <div className="mt-1 text-xl font-semibold">{overview?.PendingCount ?? 0}</div>
              </div>
            </div>
          </section>

          <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
            <div className="border-b border-[color:var(--border)] px-6 py-4">
              <p className="text-sm font-semibold">By Strategy</p>
            </div>
            <div className="hidden overflow-x-auto md:block">
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
            <div className="space-y-3 p-4 md:hidden">
              {byStrategy.map((r) => (
                <div key={`m-${r.StrategyName}`} className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-sm">
                  <div className="font-medium">{r.StrategyName}</div>
                  <div className="mt-1 text-xs text-[var(--muted)]">plans {r.Plans} · avg ROI {fmt(r.AvgROI * 100)}%</div>
                  <div className={r.TotalPnLUSD >= 0 ? "mt-1 text-sm text-profit" : "mt-1 text-sm text-loss"}>${fmt(r.TotalPnLUSD)}</div>
                </div>
              ))}
            </div>
          </section>

          <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
            <div className="border-b border-[color:var(--border)] px-6 py-4">
              <p className="text-sm font-semibold">Failures</p>
            </div>
            <div className="hidden overflow-x-auto md:block">
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
            <div className="space-y-3 p-4 md:hidden">
              {failures.map((r) => (
                <div key={`m-${r.FailureReason}-${r.Count}`} className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-sm">
                  <div className="text-xs text-[var(--muted)]">{r.FailureReason || "(empty)"}</div>
                  <div className="mt-1 font-semibold">{r.Count}</div>
                </div>
              ))}
            </div>
          </section>
        </>
      ) : null}

      {tab === "performance" ? (
        <>
          <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-5">
            <p className="mb-3 text-sm font-semibold">Cumulative PnL Curve</p>
            <div className="grid grid-cols-12 items-end gap-1">
              {cumulativeSeries.map((pt) => {
                const ratio = (pt.cumulative - curveRange.min) / (curveRange.max - curveRange.min || 1);
                const h = Math.max(4, Math.round(ratio * 140));
                return (
                  <div key={pt.date} className="group relative h-36">
                    <div className="absolute bottom-0 w-full rounded bg-black/70" style={{ height: `${h}px` }} />
                    <div className="pointer-events-none absolute bottom-full left-1/2 z-10 hidden -translate-x-1/2 rounded bg-black px-2 py-1 text-[10px] text-white group-hover:block">
                      {pt.date}: ${fmt(pt.cumulative)}
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-5">
            <p className="mb-3 text-sm font-semibold">Drawdown Curve</p>
            <div className="grid grid-cols-12 items-end gap-1">
              {drawdownCurve.map((pt) => {
                const ratio = (pt.dd - drawdownRange.min) / (drawdownRange.max - drawdownRange.min || 1);
                const h = Math.max(4, Math.round(ratio * 140));
                return (
                  <div key={`dd-${pt.date}`} className="group relative h-36">
                    <div className="absolute bottom-0 w-full rounded bg-red-500/70" style={{ height: `${h}px` }} />
                    <div className="pointer-events-none absolute bottom-full left-1/2 z-10 hidden -translate-x-1/2 rounded bg-black px-2 py-1 text-[10px] text-white group-hover:block">
                      {pt.date}: {fmt(pt.dd)}
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">max_drawdown_usd: ${fmt(drawdown?.MaxDrawdownUSD ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">max_drawdown_pct: {fmt((drawdown?.MaxDrawdownPct ?? 0) * 100)}%</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">drawdown_days: {drawdown?.DrawdownDurationDays ?? 0}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">current_drawdown: ${fmt(drawdown?.CurrentDrawdownUSD ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">peak_pnl: ${fmt(drawdown?.PeakPnL ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">trough_pnl: ${fmt(drawdown?.TroughPnL ?? 0)}</div>
          </section>
        </>
      ) : null}

      {tab === "attribution" ? (
        <>
          <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-5">
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <select
                className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                value={attrStrategy}
                onChange={(e) => {
                  setAttrStrategy(e.target.value);
                  void refreshAttribution(e.target.value);
                }}
              >
                {byStrategy.map((s) => (
                  <option key={s.StrategyName} value={s.StrategyName}>
                    {s.StrategyName}
                  </option>
                ))}
              </select>
              <button
                className="rounded-full border border-[color:var(--border)] px-3 py-1 text-xs hover:bg-[var(--surface-strong)]"
                onClick={() => void refreshAttribution(attrStrategy)}
              >
                刷新 Attribution
              </button>
            </div>
            <p className="mb-3 text-sm font-semibold">Attribution Waterfall</p>
            <div className="space-y-2">
              {waterfall.map((row) => {
                const width = Math.max(6, Math.round((Math.abs(row.value) / wfMax) * 100));
                const positive = row.value >= 0;
                return (
                  <div key={row.label} className="flex items-center gap-3 text-xs">
                    <div className="w-28 shrink-0 text-[var(--muted)]">{row.label}</div>
                    <div className="h-3 flex-1 rounded bg-[var(--surface-strong)]">
                      <div
                        className={`h-3 rounded ${positive ? "bg-emerald-500/80" : "bg-red-500/80"}`}
                        style={{ width: `${width}%` }}
                      />
                    </div>
                    <div className="w-28 text-right font-medium">{positive ? "+" : ""}${fmt(row.value)}</div>
                  </div>
                );
              })}
            </div>
          </section>

          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Sharpe: {fmt(ratios?.SharpeRatio ?? 0, 3)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Sortino: {fmt(ratios?.SortinoRatio ?? 0, 3)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Win Rate: {fmt((ratios?.WinRate ?? 0) * 100)}%</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Profit Factor: {fmt(ratios?.ProfitFactor ?? 0, 3)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Avg Win: ${fmt(ratios?.AvgWin ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Avg Loss: ${fmt(ratios?.AvgLoss ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Expectancy: ${fmt(ratios?.Expectancy ?? 0)}</div>
            <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Net PnL: ${fmt(attribution?.NetPnL ?? 0)}</div>
          </section>
        </>
      ) : null}
    </div>
  );
}
