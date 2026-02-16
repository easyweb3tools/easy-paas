"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";

import { apiGet, apiPost } from "@/lib/api";
import { isSafePathSegment, toSafePathSegment } from "@/lib/path";

type ExecutionPlan = {
  ID: number;
  OpportunityID: number;
  Status: string;
  StrategyName: string;
  PlannedSizeUSD: string;
  Params: Record<string, unknown>;
  PreflightResult: Record<string, unknown>;
  Legs: Array<Record<string, unknown>>;
  ExecutedAt?: string | null;
  CreatedAt?: string;
  UpdatedAt?: string;
};

type Fill = {
  ID: number;
  PlanID: number;
  TokenID: string;
  Direction: string;
  FilledSize: string;
  AvgPrice: string;
  Fee: string;
  Slippage?: string | null;
  FilledAt: string;
};

type PnL = {
  PlanID: number;
  StrategyName: string;
  ExpectedEdge: string;
  RealizedPnL?: string | null;
  RealizedROI?: string | null;
  SlippageLoss?: string | null;
  Outcome: string;
  FailureReason?: string | null;
  SettledAt?: string | null;
  Notes?: string | null;
};

function pretty(v: unknown) {
  try {
    return JSON.stringify(v ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

export default function ExecutionDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id ?? "";
  const safeID = useMemo(() => {
    if (!isSafePathSegment(id)) return "";
    return toSafePathSegment(id);
  }, [id]);

  const [plan, setPlan] = useState<ExecutionPlan | null>(null);
  const [pnl, setPnL] = useState<PnL | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [fillTokenId, setFillTokenId] = useState("");
  const [fillDirection, setFillDirection] = useState("BUY_YES");
  const [fillSize, setFillSize] = useState("10");
  const [fillPrice, setFillPrice] = useState("0.50");
  const [fillFee, setFillFee] = useState("0");

  const [settleOutcomes, setSettleOutcomes] = useState("{}");
  const loadingRef = useRef(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    if (!safeID) {
      setError("Invalid execution ID");
      return;
    }
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(null);
    try {
      const body = await apiGet<ExecutionPlan>(`/api/v2/executions/${safeID}`, { cache: "no-store", signal });
      setPlan(body.data);
      try {
        const pnlBody = await apiGet<PnL>(`/api/v2/executions/${safeID}/pnl`, { cache: "no-store", signal });
        setPnL(pnlBody.data);
      } catch {
        setPnL(null);
      }
      // Pre-fill settlement JSON with market IDs if available.
      const legs = (body.data as any)?.Legs;
      if (legs) {
        const mids = new Set<string>();
        for (const leg of legs as any[]) {
          if (leg?.market_id) mids.add(String(leg.market_id));
        }
        if (mids.size > 0) {
          const next: Record<string, string> = {};
          for (const mid of mids) next[mid] = "NO";
          setSettleOutcomes(JSON.stringify(next, null, 2));
        }
      }
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "unknown error");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [safeID]);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  const canAct = useMemo(() => !!plan && !loading, [plan, loading]);

  async function preflight() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/preflight`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function markExecuting() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/mark-executing`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function markExecuted() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/mark-executed`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function submitToClob() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/submit`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function cancel() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/cancel`, {});
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function addFill() {
    if (!safeID) return;
    try {
      await apiPost(`/api/v2/executions/${safeID}/fill`, {
        token_id: fillTokenId,
        direction: fillDirection,
        filled_size: fillSize,
        avg_price: fillPrice,
        fee: fillFee,
      });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "unknown error");
    }
  }

  async function settle() {
    if (!safeID) return;
    try {
      const parsed = JSON.parse(settleOutcomes || "{}") as Record<string, string>;
      await apiPost(`/api/v2/executions/${safeID}/settle`, { market_outcomes: parsed });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "invalid json");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] px-6 py-5 shadow-[var(--shadow)]">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--muted)]">V2</p>
            <h1 className="text-2xl font-semibold tracking-tight">Execution #{id}</h1>
            <p className="mt-2 text-sm text-[var(--muted)]">
              {plan ? `${plan.StrategyName} · status=${plan.Status}` : "加载中..."}
            </p>
            {error ? <div className="mt-3 text-sm text-red-500">{error}</div> : null}
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void refresh()}
              disabled={loading}
            >
              Refresh
            </button>
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void preflight()}
              disabled={!canAct}
            >
              Preflight
            </button>
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void markExecuting()}
              disabled={!canAct}
            >
              Mark Executing
            </button>
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void submitToClob()}
              disabled={!canAct}
            >
              Submit
            </button>
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void markExecuted()}
              disabled={!canAct}
            >
              Mark Executed
            </button>
            <button
              className="rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void cancel()}
              disabled={!canAct}
            >
              Cancel
            </button>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Plan</p>
          <p className="text-xs text-[var(--muted)]">legs / params / preflight。</p>
        </div>
        <div className="grid gap-4 px-6 py-5 md:grid-cols-2">
          <div>
            <div className="text-xs font-semibold text-[var(--muted)]">legs</div>
            <pre className="mt-2 max-h-[320px] overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
              {pretty(plan?.Legs)}
            </pre>
          </div>
          <div className="flex flex-col gap-4">
            <div>
              <div className="text-xs font-semibold text-[var(--muted)]">params</div>
              <pre className="mt-2 max-h-[140px] overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
                {pretty(plan?.Params)}
              </pre>
            </div>
            <div>
              <div className="text-xs font-semibold text-[var(--muted)]">preflight_result</div>
              <pre className="mt-2 max-h-[140px] overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
                {pretty(plan?.PreflightResult)}
              </pre>
            </div>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Add Fill</p>
          <p className="text-xs text-[var(--muted)]">录入成交后会把 plan 状态推进到 partial（MVP）。</p>
        </div>
        <div className="grid gap-3 px-6 py-5 text-xs sm:grid-cols-5">
          <label className="flex flex-col gap-1 sm:col-span-2">
            <span className="text-[var(--muted)]">token_id</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2 font-mono"
              value={fillTokenId}
              onChange={(e) => setFillTokenId(e.target.value)}
              placeholder="token id"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">direction</span>
            <select
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={fillDirection}
              onChange={(e) => setFillDirection(e.target.value)}
            >
              <option value="BUY_YES">BUY_YES</option>
              <option value="BUY_NO">BUY_NO</option>
            </select>
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">filled_size</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={fillSize}
              onChange={(e) => setFillSize(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">avg_price</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={fillPrice}
              onChange={(e) => setFillPrice(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[var(--muted)]">fee</span>
            <input
              className="rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] px-3 py-2"
              value={fillFee}
              onChange={(e) => setFillFee(e.target.value)}
            />
          </label>
          <div className="flex items-end">
            <button
              className="w-full rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void addFill()}
              disabled={!canAct}
            >
              Add Fill
            </button>
          </div>
        </div>
      </section>

      <section className="glass-panel rounded-xl border border-[color:var(--border)] bg-[var(--surface)] shadow-[var(--shadow)]">
        <div className="border-b border-[color:var(--border)] px-6 py-4">
          <p className="text-sm font-semibold">Settle PnL</p>
          <p className="text-xs text-[var(--muted)]">
            需要 market_id {"->"} YES/NO。若已通过 `POST /api/v2/settlements` 写入市场结算，可不传覆盖。
          </p>
        </div>
        <div className="grid gap-4 px-6 py-5 md:grid-cols-2">
          <div>
            <div className="text-xs font-semibold text-[var(--muted)]">market_outcomes</div>
            <textarea
              className="mt-2 h-56 w-full rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 font-mono text-xs"
              value={settleOutcomes}
              onChange={(e) => setSettleOutcomes(e.target.value)}
            />
            <button
              className="mt-3 rounded-full border border-[color:var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-medium hover:bg-[var(--surface-strong)]"
              onClick={() => void settle()}
              disabled={!canAct}
            >
              Settle
            </button>
          </div>
          <div>
            <div className="text-xs font-semibold text-[var(--muted)]">pnl_record</div>
            <pre className="mt-2 h-56 overflow-auto rounded-xl border border-[color:var(--border)] bg-[var(--surface-strong)] p-3 text-xs">
              {pretty(pnl)}
            </pre>
          </div>
        </div>
      </section>
    </div>
  );
}
