"use client";

import { useCallback, useEffect, useState } from "react";

import { apiDelete, apiGet, apiPost } from "@/lib/api";

type LabelRow = {
  ID: number;
  MarketID: string;
  Label: string;
  SubLabel?: string | null;
  AutoLabeled: boolean;
  Confidence: number;
  CreatedAt?: string;
};

export default function LabelsPage() {
  const [items, setItems] = useState<LabelRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [marketId, setMarketId] = useState("");
  const [label, setLabel] = useState("");
  const [subLabel, setSubLabel] = useState("");

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<LabelRow[]>(`/api/v2/markets/labels?limit=200&order_by=created_at&order=desc`, {
        cache: "no-store",
      });
      setItems(body.data ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function autoLabel() {
    try {
      await apiPost(`/api/v2/markets/auto-label`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function add() {
    try {
      await apiPost(`/api/v2/markets/${marketId}/labels`, {
        label,
        sub_label: subLabel ? subLabel : null,
        auto_labeled: false,
        confidence: 1.0,
      });
      setLabel("");
      setSubLabel("");
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function del(marketID: string, labelName: string) {
    try {
      await apiDelete(`/api/v2/markets/${marketID}/labels/${labelName}`);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Labels</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">市场标注与自动标注触发。</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void refresh()}
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium text-[var(--text)] transition hover:bg-[var(--surface-strong)]"
              disabled={loading}
            >
              {loading ? "刷新中..." : "刷新"}
            </button>
            <button
              type="button"
              onClick={() => void autoLabel()}
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium text-[var(--text)] transition hover:bg-[var(--surface-strong)]"
              disabled={loading}
            >
              Auto Label
            </button>
          </div>
        </div>
        {error ? <div className="mt-4 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Add Label</p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-xs sm:grid-cols-4">
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">market_id</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 font-mono"
              value={marketId}
              onChange={(e) => setMarketId(e.target.value)}
              placeholder="market id"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">label</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="weather"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">sub_label</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={subLabel}
              onChange={(e) => setSubLabel(e.target.value)}
              placeholder="new-york"
            />
          </label>
          <div className="flex items-end">
            <button
              className="w-full rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void add()}
              disabled={loading || !marketId || !label}
            >
              Add
            </button>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">List</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">market</th>
                <th className="px-4 py-3">label</th>
                <th className="px-4 py-3">sub</th>
                <th className="px-4 py-3">auto</th>
                <th className="px-4 py-3">conf</th>
                <th className="px-4 py-3">actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.ID} className="border-t border-[color:var(--border)]">
                  <td className="px-6 py-4 font-mono text-xs">{it.MarketID}</td>
                  <td className="px-4 py-4">{it.Label}</td>
                  <td className="px-4 py-4">{it.SubLabel ?? ""}</td>
                  <td className="px-4 py-4">{it.AutoLabeled ? "true" : "false"}</td>
                  <td className="px-4 py-4">{it.Confidence.toFixed(2)}</td>
                  <td className="px-4 py-4">
                    <button
                      className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                      onClick={() => void del(it.MarketID, it.Label)}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
