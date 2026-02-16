"use client";

import { useCallback } from "react";

import { apiGet, apiPost } from "@/lib/api";
import { useApi } from "@/lib/useApi";

type Order = {
  ID: number;
  PlanID: number;
  TokenID: string;
  Side: string;
  Price: string;
  SizeUSD: string;
  FilledUSD: string;
  Status: string;
  CreatedAt: string;
};

function fmt(v: string | number, digits = 2) {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) return "--";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(n);
}

export default function OrdersPage() {
  const {
    data: itemsData,
    error,
    loading,
    refresh,
    setError,
  } = useApi<Order[]>(
    useCallback(async (signal?: AbortSignal) => {
      const body = await apiGet<Order[]>("/api/v2/orders?limit=100", { cache: "no-store", signal });
      return body.data ?? [];
    }, []),
    []
  );
  const items = itemsData ?? [];

  async function cancel(id: number) {
    try {
      await apiPost(`/api/v2/orders/${id}/cancel`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Orders</h1>
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

      <section className="glass-panel overflow-hidden rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        {loading ? (
          <div className="space-y-2 px-6 py-6">
            <div className="skeleton h-6 w-1/2" />
            <div className="skeleton h-16 w-full" />
            <div className="skeleton h-16 w-full" />
          </div>
        ) : null}
        {items.length === 0 && !loading ? (
          <div className="px-6 py-8 text-sm text-[var(--muted)]">No active orders. Submit an execution plan to place orders.</div>
        ) : null}
        {!loading && items.length > 0 ? (
          <>
        <div className="overflow-x-auto hidden md:block">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-4 py-3">id ↑↓</th>
                <th className="px-4 py-3">plan</th>
                <th className="px-4 py-3">token</th>
                <th className="px-4 py-3">side</th>
                <th className="px-4 py-3">price</th>
                <th className="px-4 py-3">size_usd</th>
                <th className="px-4 py-3">filled_usd</th>
                <th className="px-4 py-3">status</th>
                <th className="px-4 py-3">action</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.ID} className="border-t border-[color:var(--border)]">
                  <td className="px-4 py-3 font-mono text-xs">{it.ID}</td>
                  <td className="px-4 py-3 font-mono text-xs">{it.PlanID}</td>
                  <td className="px-4 py-3 font-mono text-xs">{it.TokenID}</td>
                  <td className="px-4 py-3">{it.Side}</td>
                  <td className="px-4 py-3">{fmt(it.Price, 4)}</td>
                  <td className="px-4 py-3">${fmt(it.SizeUSD)}</td>
                  <td className="px-4 py-3">${fmt(it.FilledUSD)}</td>
                  <td className="px-4 py-3">{it.Status}</td>
                  <td className="px-4 py-3">
                    <button
                      className="rounded-full border border-[color:var(--border)] px-3 py-1 text-xs hover:bg-[var(--surface-strong)]"
                      onClick={() => void cancel(it.ID)}
                      disabled={it.Status !== "submitted" && it.Status !== "partial" && it.Status !== "pending"}
                    >
                      Cancel
                    </button>
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
              <div className="mt-2 text-xs text-[var(--muted)]">{it.TokenID}</div>
              <div className="mt-1 text-xs">{it.Side} · ${fmt(it.Price, 4)} · ${fmt(it.SizeUSD)}</div>
              <div className="mt-2">
                <button
                  className="rounded-lg border border-[color:var(--border)] px-3 py-2 text-xs"
                  onClick={() => void cancel(it.ID)}
                  disabled={it.Status !== "submitted" && it.Status !== "partial" && it.Status !== "pending"}
                >
                  Cancel
                </button>
              </div>
            </div>
          ))}
        </div>
        </>
        ) : null}
      </section>
    </div>
  );
}
