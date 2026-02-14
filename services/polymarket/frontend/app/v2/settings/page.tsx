"use client";

import { useEffect, useMemo, useState } from "react";

import { API_BASE, getAuthToken, setAuthToken } from "@/lib/api";

export default function SettingsPage() {
  const apiBase = useMemo(() => API_BASE, []);
  const [token, setToken] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setToken(getAuthToken());
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="mt-2 max-w-2xl text-sm text-[var(--muted)]">
          这里是最小化的运行配置提示页。所有参数以 backend 的 `config.yaml` 和环境变量为准。
        </p>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Frontend</p>
          <p className="text-xs text-[var(--muted)]">API base used by browser requests</p>
        </div>
        <div className="grid gap-4 px-6 py-5 text-sm">
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="text-xs font-semibold text-[var(--muted)]">NEXT_PUBLIC_API_BASE</p>
            <p className="mt-2 font-mono text-xs">{apiBase || "(empty: same-origin)"}</p>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Authorization</p>
          <p className="text-xs text-[var(--muted)]">
            UI requests go through easyweb3-platform and must include <span className="font-mono">Authorization: Bearer</span>.
          </p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-sm">
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="text-xs font-semibold text-[var(--muted)]">Bearer token (stored in localStorage)</p>
            <textarea
              className="mt-2 w-full rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 font-mono text-xs text-[var(--text)] outline-none"
              rows={4}
              value={token}
              onChange={(e) => {
                setSaved(false);
                setToken(e.target.value);
              }}
              placeholder="Paste JWT token here (without 'Bearer ')"
            />
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <button
                className="rounded-xl bg-[var(--primary)] px-3 py-2 text-xs font-semibold text-[var(--primary-ink)]"
                onClick={() => {
                  setAuthToken(token);
                  setSaved(true);
                }}
              >
                Save token
              </button>
              <button
                className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-semibold text-[var(--text)]"
                onClick={() => {
                  setAuthToken("");
                  setToken("");
                  setSaved(true);
                }}
              >
                Clear token
              </button>
              {saved ? <span className="text-xs text-[var(--muted)]">saved</span> : null}
            </div>
            <p className="mt-3 text-xs text-[var(--muted)]">
              You can obtain a token via <span className="font-mono">POST /api/v1/auth/login</span> (project_id: polymarket).
            </p>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-[28px] border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Backend Quick Checks</p>
          <p className="text-xs text-[var(--muted)]">Useful endpoints while wiring the V2 loop</p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-xs text-[var(--muted)]">
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="font-mono text-[var(--text)]">GET /healthz</p>
            <p className="mt-1">Basic liveness.</p>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="font-mono text-[var(--text)]">GET /api/v2/opportunities</p>
            <p className="mt-1">Active opportunities list.</p>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="font-mono text-[var(--text)]">GET /api/v2/strategies</p>
            <p className="mt-1">Strategy enable/params status.</p>
          </div>
          <div className="rounded-2xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-4">
            <p className="font-mono text-[var(--text)]">GET /api/v2/signals/sources</p>
            <p className="mt-1">Collector health overview.</p>
          </div>
        </div>
      </section>
    </div>
  );
}
