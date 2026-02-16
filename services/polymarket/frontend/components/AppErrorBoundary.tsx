"use client";

import { Component, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export default class AppErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: unknown) {
    console.error("UI crashed", error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-6 text-center">
          <h2 className="text-lg font-semibold">Something went wrong</h2>
          <p className="mt-2 text-sm text-[var(--muted)]">{this.state.error.message}</p>
          <button
            type="button"
            className="mt-4 rounded-lg border border-[color:var(--border)] px-4 py-2 text-sm hover:bg-[var(--surface-strong)]"
            onClick={() => this.setState({ error: null })}
          >
            Retry
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

