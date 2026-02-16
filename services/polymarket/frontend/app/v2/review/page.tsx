"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { ApiMeta, apiGet, apiPut } from "@/lib/api";

type ReviewItem = {
  ID: number;
  MarketID: string;
  EventID: string;
  OurAction: string;
  StrategyName: string;
  FinalOutcome: string;
  HypotheticalPnL: string;
  ActualPnL: string;
  Notes: string;
  LessonTags?: string[] | null;
  SettledAt: string;
};

type MissedSummary = {
  TotalDismissed: number;
  ProfitableDismissed: number;
  RegretRate: number;
  MissedAlphaUSD: number;
  AvgMissedEdge: number;
};

type LabelPerf = {
  Label: string;
  TradedCount: number;
  TradedPnL: number;
  MissedCount: number;
  MissedAlpha: number;
  WinRate: number;
};

function fmt(v: number, digits = 2) {
  if (!Number.isFinite(v)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(v);
}

function num(v: string | number, fallback = 0) {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : fallback;
}

export default function ReviewPage() {
  const [items, setItems] = useState<ReviewItem[]>([]);
  const [missed, setMissed] = useState<ReviewItem[]>([]);
  const [summary, setSummary] = useState<MissedSummary | null>(null);
  const [labels, setLabels] = useState<LabelPerf[]>([]);
  const [meta, setMeta] = useState<ApiMeta>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [ourAction, setOurAction] = useState("");
  const [strategyName, setStrategyName] = useState("");
  const [editing, setEditing] = useState<Record<number, { notes: string; tags: string }>>({});

  const query = useMemo(() => {
    const q = new URLSearchParams();
    q.set("limit", "100");
    if (ourAction.trim()) q.set("our_action", ourAction.trim());
    if (strategyName.trim()) q.set("strategy_name", strategyName.trim());
    return q.toString();
  }, [ourAction, strategyName]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [listRes, missedRes, summaryRes, labelsRes] = await Promise.all([
        apiGet<ReviewItem[]>(`/api/v2/review?${query}`, { cache: "no-store" }),
        apiGet<ReviewItem[]>("/api/v2/review/missed?limit=50", { cache: "no-store" }),
        apiGet<MissedSummary>("/api/v2/review/regret-index", { cache: "no-store" }),
        apiGet<LabelPerf[]>("/api/v2/review/label-performance", { cache: "no-store" }),
      ]);
      setItems(listRes.data ?? []);
      setMeta(listRes.meta);
      setMissed(missedRes.data ?? []);
      setSummary(summaryRes.data);
      setLabels(labelsRes.data ?? []);

      const editState: Record<number, { notes: string; tags: string }> = {};
      for (const it of listRes.data ?? []) {
        editState[it.ID] = {
          notes: it.Notes ?? "",
          tags: Array.isArray(it.LessonTags) ? it.LessonTags.join(",") : "",
        };
      }
      setEditing(editState);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function saveNotes(id: number) {
    const row = editing[id];
    if (!row) return;
    try {
      const tags = row.tags
        .split(",")
        .map((v) => v.trim())
        .filter(Boolean);
      await apiPut(`/api/v2/review/${id}/notes`, {
        notes: row.notes,
        lesson_tags: tags,
      });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Market Review</h1>
          </div>
          <input
            className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
            placeholder="our_action"
            value={ourAction}
            onChange={(e) => setOurAction(e.target.value)}
          />
          <input
            className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
            placeholder="strategy_name"
            value={strategyName}
            onChange={(e) => setStrategyName(e.target.value)}
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
        <p className="mt-2 text-xs text-[var(--muted)]">total: {meta?.total ?? items.length}</p>
        {error ? <div className="mt-3 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">
          <div className="text-xs text-[var(--muted)]">total_dismissed</div>
          <div className="mt-1 text-xl font-semibold">{summary?.TotalDismissed ?? 0}</div>
        </div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">
          <div className="text-xs text-[var(--muted)]">profitable_dismissed</div>
          <div className="mt-1 text-xl font-semibold">{summary?.ProfitableDismissed ?? 0}</div>
        </div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">
          <div className="text-xs text-[var(--muted)]">regret_rate</div>
          <div className="mt-1 text-xl font-semibold">{fmt((summary?.RegretRate ?? 0) * 100)}%</div>
        </div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">
          <div className="text-xs text-[var(--muted)]">missed_alpha_usd</div>
          <div className="mt-1 text-xl font-semibold">${fmt(summary?.MissedAlphaUSD ?? 0)}</div>
        </div>
      </section>

      <section className="glass-panel overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4 text-sm font-semibold">Missed Opportunities (top 50)</div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-4 py-3">market</th>
                <th className="px-4 py-3">action</th>
                <th className="px-4 py-3">strategy</th>
                <th className="px-4 py-3">hypothetical_pnl</th>
                <th className="px-4 py-3">settled_at</th>
              </tr>
            </thead>
            <tbody>
              {missed.map((it) => (
                <tr key={`missed-${it.ID}`} className="border-t border-[color:var(--border)]">
                  <td className="px-4 py-3 font-mono text-xs">{it.MarketID}</td>
                  <td className="px-4 py-3">{it.OurAction}</td>
                  <td className="px-4 py-3">{it.StrategyName || "--"}</td>
                  <td className="px-4 py-3">${fmt(num(it.HypotheticalPnL))}</td>
                  <td className="px-4 py-3">{it.SettledAt ? new Date(it.SettledAt).toLocaleString() : "--"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="glass-panel overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4 text-sm font-semibold">Label Performance</div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-4 py-3">label</th>
                <th className="px-4 py-3">traded_count</th>
                <th className="px-4 py-3">traded_pnl</th>
                <th className="px-4 py-3">missed_count</th>
                <th className="px-4 py-3">missed_alpha</th>
                <th className="px-4 py-3">win_rate</th>
              </tr>
            </thead>
            <tbody>
              {labels.map((row) => (
                <tr key={row.Label} className="border-t border-[color:var(--border)]">
                  <td className="px-4 py-3">{row.Label}</td>
                  <td className="px-4 py-3">{row.TradedCount}</td>
                  <td className="px-4 py-3">${fmt(row.TradedPnL)}</td>
                  <td className="px-4 py-3">{row.MissedCount}</td>
                  <td className="px-4 py-3">${fmt(row.MissedAlpha)}</td>
                  <td className="px-4 py-3">{fmt(row.WinRate * 100)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="glass-panel overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4 text-sm font-semibold">All Reviews</div>
        <div className="divide-y divide-[color:var(--border)]">
          {items.map((it) => {
            const row = editing[it.ID] ?? { notes: "", tags: "" };
            return (
              <div key={it.ID} className="px-6 py-4">
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="font-mono">review#{it.ID}</span>
                    <span className="font-mono">{it.MarketID}</span>
                    <span>{it.OurAction}</span>
                    <span>{it.StrategyName || "--"}</span>
                    <span>hypo=${fmt(num(it.HypotheticalPnL))}</span>
                    <span>actual=${fmt(num(it.ActualPnL))}</span>
                  </div>
                  <span className="text-[var(--muted)]">{it.SettledAt ? new Date(it.SettledAt).toLocaleString() : "--"}</span>
                </div>
                <div className="mt-3 grid gap-2 md:grid-cols-[1fr_240px_auto]">
                  <input
                    className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
                    value={row.notes}
                    onChange={(e) => setEditing((prev) => ({ ...prev, [it.ID]: { ...row, notes: e.target.value } }))}
                    placeholder="notes"
                  />
                  <input
                    className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
                    value={row.tags}
                    onChange={(e) => setEditing((prev) => ({ ...prev, [it.ID]: { ...row, tags: e.target.value } }))}
                    placeholder="lesson tags comma separated"
                  />
                  <button
                    className="rounded-full border border-[color:var(--border)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                    onClick={() => void saveNotes(it.ID)}
                  >
                    Save
                  </button>
                </div>
              </div>
            );
          })}
          {items.length === 0 ? <div className="px-6 py-8 text-sm text-[var(--muted)]">暂无复盘数据</div> : null}
        </div>
      </section>
    </div>
  );
}
