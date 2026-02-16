"use client";

import { useCallback, useEffect, useState } from "react";

import { apiGet } from "@/lib/api";

type Position = {
  ID: number;
  TokenID: string;
  MarketID: string;
  Direction: string;
  Quantity: string;
  AvgEntryPrice: string;
  CurrentPrice: string;
  UnrealizedPnL: string;
  Status: string;
  OpenedAt: string;
};

type Summary = {
  TotalOpen: number;
  TotalCostBasis: number;
  TotalMarketVal: number;
  UnrealizedPnL: number;
  RealizedPnL: number;
  NetLiquidation: number;
};

type Snapshot = {
  SnapshotAt: string;
  NetLiquidation: string;
};

function fmtNum(v: string | number, digits = 2) {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(n);
}

export default function PortfolioPage() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [history, setHistory] = useState<Snapshot[]>([]);
  const [status, setStatus] = useState("open");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [sum, pos, hist] = await Promise.all([
        apiGet<Summary>("/api/v2/positions/summary", { cache: "no-store" }),
        apiGet<Position[]>(`/api/v2/positions?limit=200&status=${status}`, { cache: "no-store" }),
        apiGet<Snapshot[]>("/api/v2/portfolio/history?limit=168", { cache: "no-store" }),
      ]);
      setSummary(sum.data);
      setPositions(pos.data ?? []);
      setHistory(hist.data ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Portfolio</h1>
          </div>
          <button
            className="rounded-full border border-[color:var(--border)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
            onClick={() => void refresh()}
            disabled={loading}
          >
            {loading ? "刷新中..." : "刷新"}
          </button>
        </div>
        {error ? <div className="mt-3 text-sm text-red-500">{error}</div> : null}
      </section>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Open: {summary?.TotalOpen ?? 0}</div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Cost: ${fmtNum(summary?.TotalCostBasis ?? 0)}</div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Mkt Val: ${fmtNum(summary?.TotalMarketVal ?? 0)}</div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Unrealized: ${fmtNum(summary?.UnrealizedPnL ?? 0)}</div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Realized: ${fmtNum(summary?.RealizedPnL ?? 0)}</div>
        <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface)] p-4">Net Liq: ${fmtNum(summary?.NetLiquidation ?? 0)}</div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center gap-3 text-xs">
            <span>Positions</span>
            <select
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-2 py-1"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            >
              <option value="open">open</option>
              <option value="closed">closed</option>
            </select>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-4 py-3">token</th>
                <th className="px-4 py-3">market</th>
                <th className="px-4 py-3">dir</th>
                <th className="px-4 py-3">qty</th>
                <th className="px-4 py-3">entry</th>
                <th className="px-4 py-3">current</th>
                <th className="px-4 py-3">uPnL</th>
                <th className="px-4 py-3">status</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((p) => (
                <tr key={p.ID} className="border-t border-[color:var(--border)]">
                  <td className="px-4 py-3 font-mono text-xs">{p.TokenID}</td>
                  <td className="px-4 py-3 font-mono text-xs">{p.MarketID}</td>
                  <td className="px-4 py-3">{p.Direction}</td>
                  <td className="px-4 py-3">{fmtNum(p.Quantity, 4)}</td>
                  <td className="px-4 py-3">{fmtNum(p.AvgEntryPrice, 4)}</td>
                  <td className="px-4 py-3">{fmtNum(p.CurrentPrice, 4)}</td>
                  <td className="px-4 py-3">${fmtNum(p.UnrealizedPnL, 2)}</td>
                  <td className="px-4 py-3">{p.Status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] p-4">
        <p className="text-xs text-[var(--muted)]">History points: {history.length}</p>
      </section>
    </div>
  );
}
