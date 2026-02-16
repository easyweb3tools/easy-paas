"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { apiGet, apiPost, apiPut, ApiResponse } from "@/lib/api";

type Strategy = {
  ID: number;
  Name: string;
  DisplayName: string;
  Description: string;
  Category: string;
  Enabled: boolean;
  Priority: number;
  Params: Record<string, unknown>;
  RequiredSignals: Array<string> | Record<string, unknown>;
  Stats: Record<string, unknown>;
  UpdatedAt?: string;
};

function pretty(v: unknown) {
  try {
    return JSON.stringify(v ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

export default function StrategiesPage() {
  const [items, setItems] = useState<Strategy[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Record<string, string>>({});
  const loadingRef = useRef(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<Strategy[]>("/api/v2/strategies", { cache: "no-store", signal });
      setItems(body.data ?? []);
      const next: Record<string, string> = {};
      for (const it of body.data ?? []) {
        next[it.Name] = pretty(it.Params);
      }
      setEditing(next);
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

  async function setEnabled(name: string, enabled: boolean) {
    try {
      const safeName = encodeURIComponent(name);
      await apiPost(`/api/v2/strategies/${safeName}/${enabled ? "enable" : "disable"}`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function saveParams(name: string) {
    try {
      const raw = editing[name] ?? "{}";
      JSON.parse(raw);
      const safeName = encodeURIComponent(name);
      await apiPut(`/api/v2/strategies/${safeName}/params`, JSON.parse(raw));
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "invalid json");
    }
  }

  const byPriority = useMemo(() => {
    return [...items].sort((a, b) => (a.Priority ?? 0) - (b.Priority ?? 0) || a.Name.localeCompare(b.Name));
  }, [items]);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Strategies</h1>
            <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
              启用/禁用策略，并在线调整 JSON 参数（DB 覆盖 config 默认值）。
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

      <section className="glass-panel relative overflow-hidden rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="relative z-10 border-b border-[color:var(--border)] px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">列表</p>
              <p className="text-xs text-[var(--muted)]">Priority 0=P0，1=P1，2=P2。</p>
            </div>
            <div className="text-xs text-[var(--muted)]">total: {byPriority.length}</div>
          </div>
        </div>

        {loading ? (
          <div className="space-y-2 px-6 py-6">
            <div className="skeleton h-6 w-1/2" />
            <div className="skeleton h-16 w-full" />
            <div className="skeleton h-16 w-full" />
          </div>
        ) : byPriority.length === 0 ? (
          <div className="px-6 py-10 text-sm text-[var(--muted)]">暂无数据（需要先启动策略引擎写入 strategies 表）</div>
        ) : (
          <div className="divide-y divide-[color:var(--border)]">
            {byPriority.map((it) => (
              <div key={it.Name} className="px-6 py-5">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <div className="flex items-center gap-3">
                      <div className="rounded-full border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-1 text-xs">
                        P{it.Priority}
                      </div>
                      <div className="text-sm font-semibold">{it.Name}</div>
                      <div className="text-xs text-[var(--muted)]">{it.Category}</div>
                    </div>
                    <div className="mt-2 text-xs text-[var(--muted)]">
                      required_signals: {pretty(it.RequiredSignals)}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                      onClick={() => void setEnabled(it.Name, !it.Enabled)}
                    >
                      {it.Enabled ? "Disable" : "Enable"}
                    </button>
                    <button
                      className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
                      onClick={() => void saveParams(it.Name)}
                    >
                      Save Params
                    </button>
                  </div>
                </div>

                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <div>
                    <div className="text-xs font-semibold text-[var(--muted)]">params</div>
                    <textarea
                      className="mt-2 h-44 w-full rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 font-mono text-xs"
                      value={editing[it.Name] ?? "{}"}
                      onChange={(e) => setEditing((prev) => ({ ...prev, [it.Name]: e.target.value }))}
                    />
                  </div>
                  <div>
                    <div className="text-xs font-semibold text-[var(--muted)]">stats</div>
                    <pre className="mt-2 h-44 overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
                      {pretty(it.Stats)}
                    </pre>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
