"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { apiGet, apiPost } from "@/lib/api";
import { isSafePathSegment } from "@/lib/path";

type LabelRateRow = {
  Label: string;
  Total: number;
  NoCount: number;
  NoRate: number;
};

export default function SettlementsPage() {
  const [rates, setRates] = useState<LabelRateRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [marketId, setMarketId] = useState("");
  const [outcome, setOutcome] = useState("NO");
  const [settledAt, setSettledAt] = useState("");
  const [finalYesPrice, setFinalYesPrice] = useState("");
  const loadingRef = useRef(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<LabelRateRow[]>("/api/v2/settlements/label-rates", { cache: "no-store", signal });
      setRates(body.data ?? []);
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

  async function upsert() {
    try {
      if (!isSafePathSegment(marketId)) {
        setError("Invalid market ID");
        return;
      }
      await apiPost("/api/v2/settlements", {
        market_id: marketId.trim(),
        outcome,
        settled_at: settledAt || undefined,
        final_yes_price: finalYesPrice || undefined,
      });
      setMarketId("");
      setFinalYesPrice("");
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
            <h1 className="text-2xl font-semibold tracking-tight">Settlements</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              手工写入市场结算结果（用于 Systematic NO 的历史统计，以及 execution settle 计算 PnL）。
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

      <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Upsert Settlement</p>
          <p className="text-xs text-[var(--muted)]">按 market_id 唯一键 upsert。</p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-xs sm:grid-cols-6">
          <label className="flex flex-col gap-1 sm:col-span-3">
            <span className="text-[var(--muted)]">market_id</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 font-mono"
              value={marketId}
              onChange={(e) => setMarketId(e.target.value)}
              placeholder="market id"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">outcome</span>
            <select
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={outcome}
              onChange={(e) => setOutcome(e.target.value)}
            >
              <option value="YES">YES</option>
              <option value="NO">NO</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">settled_at (RFC3339)</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 font-mono"
              value={settledAt}
              onChange={(e) => setSettledAt(e.target.value)}
              placeholder="2026-02-14T00:00:00Z"
            />
          </label>
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">final_yes_price</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={finalYesPrice}
              onChange={(e) => setFinalYesPrice(e.target.value)}
              placeholder="1.0"
            />
          </label>
          <div className="flex items-end sm:col-span-2">
            <button
              className="w-full rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void upsert()}
              disabled={loading || !marketId}
            >
              Upsert
            </button>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Label NO Rates</p>
          <p className="text-xs text-[var(--muted)]">来自 `market_settlement_history` + `market_labels` join。</p>
        </div>
        <div className="hidden overflow-x-auto md:block">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">label</th>
                <th className="px-4 py-3">total</th>
                <th className="px-4 py-3">no_count</th>
                <th className="px-4 py-3">no_rate</th>
              </tr>
            </thead>
            <tbody>
              {rates.map((r) => (
                <tr key={r.Label} className="border-t border-[color:var(--border)]">
                  <td className="px-6 py-4">{r.Label}</td>
                  <td className="px-4 py-4">{r.Total}</td>
                  <td className="px-4 py-4">{r.NoCount}</td>
                  <td className="px-4 py-4">{(r.NoRate * 100).toFixed(2)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="space-y-3 p-4 md:hidden">
          {rates.map((r) => (
            <div key={`m-${r.Label}`} className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-sm">
              <div className="font-medium">{r.Label}</div>
              <div className="mt-1 text-xs text-[var(--muted)]">total {r.Total} · no {r.NoCount}</div>
              <div className="mt-1 text-xs">NO rate {(r.NoRate * 100).toFixed(2)}%</div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
