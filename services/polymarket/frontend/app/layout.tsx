import type { Metadata } from "next";

import "./globals.css";
import AppErrorBoundary from "@/components/AppErrorBoundary";
import AppNavigation from "@/components/AppNavigation";

export const metadata: Metadata = {
  title: "Polymarket Dashboard",
  description: "Modern prediction markets, designed with clarity and depth.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className="h-full scroll-smooth">
      <body className="min-h-dvh bg-[var(--bg)] text-[var(--text)] antialiased selection:bg-black/10 selection:text-black dark:selection:bg-white/10 dark:selection:text-white">
        <div className="relative">
          <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(0,0,0,0.04),transparent_55%)] dark:bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.06),transparent_55%)]" />
          <header className="glass-panel sticky top-0 z-40 border-b border-[color:var(--border)] bg-[var(--glass)] backdrop-blur-md shadow-[var(--shadow)]">
            <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3 sm:px-6">
              <a href="/" className="flex items-center gap-3">
                <div className="h-9 w-9 rounded-xl bg-black/90 text-white shadow-[0_8px_24px_rgba(0,0,0,0.12)] dark:bg-white/90 dark:text-black">
                  <div className="flex h-full w-full items-center justify-center text-sm font-semibold">PM</div>
                </div>
                <div>
                  <p className="text-sm font-semibold tracking-tight">Polymarket</p>
                  <p className="text-xs text-[var(--muted)]">Prediction Studio</p>
                </div>
              </a>
              <AppNavigation />
            </div>
          </header>

          <main className="mx-auto flex min-h-[calc(100vh-64px)] max-w-6xl flex-col gap-6 px-4 py-6 pb-20 sm:px-6 md:pb-6">
            <AppErrorBoundary>{children}</AppErrorBoundary>
          </main>
        </div>
      </body>
    </html>
  );
}
