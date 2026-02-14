"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { apiGet, ApiResponse } from "@/lib/api";

type Signal = {
  ID: number;
  SignalType: string;
  Source: string;
  MarketID?: string | null;
  EventID?: string | null;
  TokenID?: string | null;
  Strength: number;
  Direction: string;
  Payload: unknown;
  ExpiresAt?: string | null;
  CreatedAt?: string;
};

type SignalSource = {
  Name: string;
  SourceType: string;
  Endpoint: string;
  PollInterval: string;
  Enabled: boolean;
  LastPollAt?: string | null;
  LastError?: string | null;
  HealthStatus: string;
  UpdatedAt?: string;
};

function pretty(v: unknown) {
  try {
    return JSON.stringify(v ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

export default function SignalsPage() {
  const [signals, setSignals] = useState<Signal[]>([]);
  const [sources, setSources] = useState<SignalSource[]>([]);
  const [meta, setMeta] = useState<ApiResponse<Signal[]>["meta"]>();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [type, setType] = useState("");
  const [source, setSource] = useState("");

  const query = useMemo(() => {
    const sp = new URLSearchParams();
    if (type) sp.set("type", type);
    if (source) sp.set("source", source);
    sp.set("limit", "100");
    sp.set("offset", "0");
    return sp.toString();
  }, [type, source]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [sigBody, srcBody] = await Promise.all([
        apiGet<Signal[]>(`/api/v2/signals?${query}`, { cache: "no-store" }),
        apiGet<SignalSource[]>(`/api/v2/signals/sources`, { cache: "no-store" }),
      ]);
      setSignals(sigBody.data ?? []);
      setMeta(sigBody.meta);
      setSources(srcBody.data ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Signals</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">查看信号与信号源健康状态。</p>
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
        <div className="mt-4 grid grid-cols-2 gap-3 text-xs sm:grid-cols-6">
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">type</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={type}
              onChange={(e) => setType(e.target.value)}
              placeholder="arb_sum_deviation"
            />
          </label>
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">source</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={source}
              onChange={(e) => setSource(e.target.value)}
              placeholder="internal_scan"
            />
          </label>
          <div className="flex flex-col justify-end">
            <div className="text-[var(--muted)]">total: {meta?.total ?? signals.length}</div>
          </div>
          {error ? <div className="flex items-end text-red-500">{error}</div> : null}
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Signal Sources</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">name</th>
                <th className="px-4 py-3">type</th>
                <th className="px-4 py-3">status</th>
                <th className="px-4 py-3">endpoint</th>
                <th className="px-4 py-3">last_poll</th>
                <th className="px-4 py-3">last_error</th>
              </tr>
            </thead>
            <tbody>
              {sources.map((s) => (
                <tr key={s.Name} className="border-t border-[color:var(--border)]">
                  <td className="px-6 py-4 font-mono text-xs">{s.Name}</td>
                  <td className="px-4 py-4">{s.SourceType}</td>
                  <td className="px-4 py-4">{s.HealthStatus}</td>
                  <td className="px-4 py-4 max-w-[420px] truncate">{s.Endpoint}</td>
                  <td className="px-4 py-4">{s.LastPollAt ?? "--"}</td>
                  <td className="px-4 py-4 max-w-[420px] truncate">{s.LastError ?? ""}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Signals</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-[color:var(--glass)] text-xs uppercase tracking-[0.15em] text-[var(--muted)]">
              <tr>
                <th className="px-6 py-3">id</th>
                <th className="px-4 py-3">type</th>
                <th className="px-4 py-3">source</th>
                <th className="px-4 py-3">strength</th>
                <th className="px-4 py-3">direction</th>
                <th className="px-4 py-3">market</th>
                <th className="px-4 py-3">token</th>
                <th className="px-4 py-3">payload</th>
              </tr>
            </thead>
            <tbody>
              {signals.map((it) => (
                <tr key={it.ID} className="border-t border-[color:var(--border)] align-top">
                  <td className="px-6 py-4 font-mono text-xs">{it.ID}</td>
                  <td className="px-4 py-4">{it.SignalType}</td>
                  <td className="px-4 py-4">{it.Source}</td>
                  <td className="px-4 py-4">{it.Strength.toFixed(2)}</td>
                  <td className="px-4 py-4">{it.Direction}</td>
                  <td className="px-4 py-4 font-mono text-xs">{it.MarketID ?? ""}</td>
                  <td className="px-4 py-4 font-mono text-xs">{it.TokenID ?? ""}</td>
                  <td className="px-4 py-4">
                    <details>
                      <summary className="cursor-pointer text-xs text-[var(--text)]">view</summary>
                      <pre className="mt-2 max-h-[220px] overflow-auto rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
                        {pretty(it.Payload)}
                      </pre>
                    </details>
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

