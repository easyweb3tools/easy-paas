"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { apiGet, apiPut, ApiMeta } from "@/lib/api";

type JournalItem = {
  ExecutionPlanID: number;
  OpportunityID: number;
  StrategyName: string;
  Outcome: string;
  EntryReasoning: string;
  ExitReasoning: string;
  PnLUSD?: string | null;
  ROI?: string | null;
  Notes: string;
  Tags?: string[] | null;
  SignalSnapshot?: unknown;
  MarketSnapshot?: unknown;
  EntryParams?: unknown;
  OutcomeSnapshot?: unknown;
  CreatedAt: string;
  UpdatedAt: string;
};

export default function JournalPage() {
  const [items, setItems] = useState<JournalItem[]>([]);
  const [meta, setMeta] = useState<ApiMeta>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [strategy, setStrategy] = useState("");
  const [outcome, setOutcome] = useState("");
  const [editing, setEditing] = useState<Record<number, { notes: string; tags: string }>>({});
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});

  const query = useMemo(() => {
    const q = new URLSearchParams();
    q.set("limit", "100");
    if (strategy.trim()) q.set("strategy_name", strategy.trim());
    if (outcome.trim()) q.set("outcome", outcome.trim());
    return q.toString();
  }, [strategy, outcome]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<JournalItem[]>(`/api/v2/journal?${query}`, { cache: "no-store" });
      setItems(body.data ?? []);
      setMeta(body.meta);
      const next: Record<number, { notes: string; tags: string }> = {};
      for (const it of body.data ?? []) {
        next[it.ExecutionPlanID] = { notes: it.Notes ?? "", tags: Array.isArray(it.Tags) ? it.Tags.join(",") : "" };
      }
      setEditing(next);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function save(planID: number) {
    const row = editing[planID];
    if (!row) return;
    try {
      const tags = row.tags
        .split(",")
        .map((v) => v.trim())
        .filter(Boolean);
      await apiPut(`/api/v2/journal/${planID}/notes`, { notes: row.notes, tags });
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
            <h1 className="text-2xl font-semibold tracking-tight">Trade Journal</h1>
          </div>
          <input
            className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
            placeholder="strategy_name"
            value={strategy}
            onChange={(e) => setStrategy(e.target.value)}
          />
          <input
            className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
            placeholder="outcome"
            value={outcome}
            onChange={(e) => setOutcome(e.target.value)}
          />
          <button
            type="button"
            onClick={() => void refresh()}
            className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
            disabled={loading}
          >
            {loading ? "刷新中..." : "刷新"}
          </button>
        </div>
        <p className="mt-2 text-xs text-[var(--muted)]">total: {meta?.total ?? items.length}</p>
        {error ? <div className="mt-3 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="glass-panel overflow-hidden rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        {loading ? (
          <div className="px-6 py-8 text-sm text-[var(--muted)]">加载中...</div>
        ) : items.length === 0 ? (
          <div className="px-6 py-8 text-sm text-[var(--muted)]">暂无复盘记录</div>
        ) : (
          <div className="divide-y divide-[color:var(--border)]">
            {items.map((it) => {
              const row = editing[it.ExecutionPlanID] ?? { notes: "", tags: "" };
              return (
                <div key={it.ExecutionPlanID} className="px-6 py-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="flex flex-wrap items-center gap-3 text-xs">
                      <span className="font-mono">plan#{it.ExecutionPlanID}</span>
                      <span>{it.StrategyName}</span>
                      <span>outcome={it.Outcome || "pending"}</span>
                      <span>pnl={it.PnLUSD ?? "--"}</span>
                      <span>roi={it.ROI ?? "--"}</span>
                    </div>
                    <span className="text-xs text-[var(--muted)]">{new Date(it.CreatedAt).toLocaleString()}</span>
                  </div>
                  <div className="mt-2 text-xs text-[var(--muted)]">{it.EntryReasoning || "no entry reasoning"}</div>
                  <div className="mt-1 text-xs text-[var(--muted)]">{it.ExitReasoning || "no exit reasoning"}</div>
                  <div className="mt-3 grid gap-2 md:grid-cols-[1fr_240px_auto]">
                    <input
                      className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
                      value={row.notes}
                      onChange={(e) =>
                        setEditing((prev) => ({ ...prev, [it.ExecutionPlanID]: { ...row, notes: e.target.value } }))
                      }
                      placeholder="notes"
                    />
                    <input
                      className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 text-xs"
                      value={row.tags}
                      onChange={(e) =>
                        setEditing((prev) => ({ ...prev, [it.ExecutionPlanID]: { ...row, tags: e.target.value } }))
                      }
                      placeholder="tags comma separated"
                    />
                    <button
                      className="rounded-full border border-[color:var(--border)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                      onClick={() => void save(it.ExecutionPlanID)}
                    >
                      Save Notes
                    </button>
                  </div>
                  <div className="mt-2">
                    <button
                      className="rounded-full border border-[color:var(--border)] px-3 py-1 text-xs hover:bg-[var(--surface-strong)]"
                      onClick={() =>
                        setExpanded((prev) => ({
                          ...prev,
                          [it.ExecutionPlanID]: !prev[it.ExecutionPlanID],
                        }))
                      }
                    >
                      {expanded[it.ExecutionPlanID] ? "Hide Decision Chain" : "Show Decision Chain"}
                    </button>
                  </div>
                  {expanded[it.ExecutionPlanID] ? (
                    <div className="mt-3 grid gap-2 text-[11px] md:grid-cols-2">
                      <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3">
                        <p className="mb-1 text-xs font-semibold">Signals</p>
                        <pre className="overflow-auto whitespace-pre-wrap break-all text-[10px] text-[var(--muted)]">
                          {JSON.stringify(it.SignalSnapshot ?? {}, null, 2)}
                        </pre>
                      </div>
                      <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3">
                        <p className="mb-1 text-xs font-semibold">Market At Entry</p>
                        <pre className="overflow-auto whitespace-pre-wrap break-all text-[10px] text-[var(--muted)]">
                          {JSON.stringify(it.MarketSnapshot ?? {}, null, 2)}
                        </pre>
                      </div>
                      <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3">
                        <p className="mb-1 text-xs font-semibold">Entry Params</p>
                        <pre className="overflow-auto whitespace-pre-wrap break-all text-[10px] text-[var(--muted)]">
                          {JSON.stringify(it.EntryParams ?? {}, null, 2)}
                        </pre>
                      </div>
                      <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3">
                        <p className="mb-1 text-xs font-semibold">Outcome / Execution</p>
                        <pre className="overflow-auto whitespace-pre-wrap break-all text-[10px] text-[var(--muted)]">
                          {JSON.stringify(it.OutcomeSnapshot ?? {}, null, 2)}
                        </pre>
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
