"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { apiDelete, apiGet, apiPut } from "@/lib/api";
import { DEFAULTS } from "@/lib/constants";

type ExecutionRule = {
  ID: number;
  StrategyName: string;
  AutoExecute: boolean;
  MinConfidence: number;
  MinEdgePct: string;
  StopLossPct: string;
  TakeProfitPct: string;
  MaxHoldHours: number;
  MaxDailyTrades: number;
};

type Strategy = {
  Name: string;
};

type Draft = {
  auto_execute: boolean;
  min_confidence: string;
  min_edge_pct: string;
  stop_loss_pct: string;
  take_profit_pct: string;
  max_hold_hours: string;
  max_daily_trades: string;
};

function toDraft(rule?: ExecutionRule): Draft {
  return {
    auto_execute: !!rule?.AutoExecute,
    min_confidence: String(rule?.MinConfidence ?? DEFAULTS.MIN_CONFIDENCE),
    min_edge_pct: String(rule?.MinEdgePct ?? DEFAULTS.MIN_EDGE),
    stop_loss_pct: String(rule?.StopLossPct ?? DEFAULTS.STOP_LOSS_PCT),
    take_profit_pct: String(rule?.TakeProfitPct ?? DEFAULTS.TAKE_PROFIT_PCT),
    max_hold_hours: String(rule?.MaxHoldHours ?? DEFAULTS.MAX_HOLD_HOURS),
    max_daily_trades: String(rule?.MaxDailyTrades ?? DEFAULTS.MAX_DAILY_TRADES),
  };
}

export default function AutomationPage() {
  const [rules, setRules] = useState<ExecutionRule[]>([]);
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const loadingRef = useRef(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const [rulesBody, strategyBody] = await Promise.all([
        apiGet<ExecutionRule[]>("/api/v2/execution-rules", { cache: "no-store", signal }),
        apiGet<Strategy[]>("/api/v2/strategies", { cache: "no-store", signal }),
      ]);
      const nextRules = rulesBody.data ?? [];
      const nextStrategies = strategyBody.data ?? [];
      setRules(nextRules);
      setStrategies(nextStrategies);
      const m: Record<string, Draft> = {};
      for (const st of nextStrategies) {
        const rule = nextRules.find((r) => r.StrategyName === st.Name);
        m[st.Name] = toDraft(rule);
      }
      setDrafts(m);
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

  const names = useMemo(() => {
    const set = new Set<string>();
    for (const st of strategies) set.add(st.Name);
    for (const r of rules) set.add(r.StrategyName);
    return Array.from(set).sort((a, b) => a.localeCompare(b));
  }, [rules, strategies]);

  async function save(name: string) {
    const d = drafts[name];
    if (!d) return;
    try {
      const safeName = encodeURIComponent(name);
      await apiPut(`/api/v2/execution-rules/${safeName}`, {
        auto_execute: d.auto_execute,
        min_confidence: Number(d.min_confidence),
        min_edge_pct: d.min_edge_pct,
        stop_loss_pct: d.stop_loss_pct,
        take_profit_pct: d.take_profit_pct,
        max_hold_hours: Number(d.max_hold_hours),
        max_daily_trades: Number(d.max_daily_trades),
      });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function remove(name: string) {
    try {
      const safeName = encodeURIComponent(name);
      await apiDelete(`/api/v2/execution-rules/${safeName}`);
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
            <h1 className="text-2xl font-semibold tracking-tight">Automation Rules</h1>
            <p className="mt-2 text-sm text-[var(--muted)]">按策略设置自动执行阈值，系统会周期扫描 active opportunities 自动下发执行。</p>
          </div>
          <button
            type="button"
            onClick={() => void refresh()}
            className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
            disabled={loading}
          >
            {loading ? "刷新中..." : "刷新"}
          </button>
        </div>
        {error ? <div className="mt-3 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="glass-panel overflow-hidden rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        {loading ? (
          <div className="space-y-2 px-6 py-6">
            <div className="skeleton h-6 w-1/2" />
            <div className="skeleton h-16 w-full" />
            <div className="skeleton h-16 w-full" />
          </div>
        ) : null}
        {names.length === 0 && !loading ? (
          <div className="px-6 py-8 text-sm text-[var(--muted)]">No strategy rules yet.</div>
        ) : null}
        {names.length > 0 && !loading ? (
          <>
        <div className="hidden overflow-x-auto md:block">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">Strategy</th>
                <th className="px-4 py-3">Auto</th>
                <th className="px-4 py-3">Min Conf</th>
                <th className="px-4 py-3">Min Edge</th>
                <th className="px-4 py-3">Stop Loss</th>
                <th className="px-4 py-3">Take Profit</th>
                <th className="px-4 py-3">Max Hold H</th>
                <th className="px-4 py-3">Daily Trades</th>
                <th className="px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {names.map((name) => {
                const d = drafts[name] ?? toDraft();
                return (
                  <tr key={name} className="border-t border-[color:var(--border)]">
                    <td className="px-6 py-3 font-mono text-xs">{name}</td>
                    <td className="px-4 py-3">
                      <input
                        type="checkbox"
                        checked={d.auto_execute}
                        onChange={(e) =>
                          setDrafts((prev) => ({ ...prev, [name]: { ...d, auto_execute: e.target.checked } }))
                        }
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.min_confidence}
                        onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, min_confidence: e.target.value } }))}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.min_edge_pct}
                        onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, min_edge_pct: e.target.value } }))}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.stop_loss_pct}
                        onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, stop_loss_pct: e.target.value } }))}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.take_profit_pct}
                        onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, take_profit_pct: e.target.value } }))}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.max_hold_hours}
                        onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, max_hold_hours: e.target.value } }))}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        className="w-20 rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1 text-xs"
                        value={d.max_daily_trades}
                        onChange={(e) =>
                          setDrafts((prev) => ({ ...prev, [name]: { ...d, max_daily_trades: e.target.value } }))
                        }
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2">
                        <button
                          className="rounded-full border border-[color:var(--border)] px-3 py-1 text-xs hover:bg-[var(--surface-strong)]"
                          onClick={() => void save(name)}
                        >
                          Save
                        </button>
                        <button
                          className="rounded-full border border-[color:var(--border)] px-3 py-1 text-xs hover:bg-[var(--surface-strong)]"
                          onClick={() => void remove(name)}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="space-y-3 p-4 md:hidden">
          {names.map((name) => {
            const d = drafts[name] ?? toDraft();
            return (
              <div key={`m-${name}`} className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-sm">
                <div className="font-mono text-xs">{name}</div>
                <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={d.auto_execute}
                      onChange={(e) => setDrafts((prev) => ({ ...prev, [name]: { ...d, auto_execute: e.target.checked } }))}
                    />
                    auto
                  </label>
                  <span>conf {d.min_confidence}</span>
                  <span>edge {d.min_edge_pct}</span>
                  <span>hold {d.max_hold_hours}h</span>
                </div>
                <div className="mt-3 flex gap-2">
                  <button className="rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs" onClick={() => void save(name)}>
                    Save
                  </button>
                  <button className="rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs" onClick={() => void remove(name)}>
                    Delete
                  </button>
                </div>
              </div>
            );
          })}
        </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
