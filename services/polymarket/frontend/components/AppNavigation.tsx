"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const groups = [
  {
    key: "market",
    label: "Market",
    links: [
      { href: "/", label: "Catalog" },
      { href: "/v2/signals", label: "Signals" },
      { href: "/v2/labels", label: "Labels" },
      { href: "/v2/settlements", label: "Settlements" },
    ],
  },
  {
    key: "trading",
    label: "Trading",
    links: [
      { href: "/v2/opportunities", label: "Opportunities" },
      { href: "/v2/executions", label: "Executions" },
      { href: "/v2/orders", label: "Orders" },
      { href: "/v2/portfolio", label: "Portfolio" },
    ],
  },
  {
    key: "strategy",
    label: "Strategy",
    links: [
      { href: "/v2/strategies", label: "Strategies" },
      { href: "/v2/automation", label: "Automation" },
      { href: "/v2/settings", label: "Settings" },
    ],
  },
  {
    key: "insights",
    label: "Insights",
    links: [
      { href: "/v2/analytics", label: "Analytics" },
      { href: "/v2/journal", label: "Journal" },
      { href: "/v2/review", label: "Review" },
    ],
  },
];

const mobileTabs = [
  { href: "/", label: "Market" },
  { href: "/v2/opportunities", label: "Trading" },
  { href: "/v2/strategies", label: "Strategy" },
  { href: "/v2/analytics", label: "Insights" },
];

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export default function AppNavigation() {
  const pathname = usePathname();
  return (
    <>
      <div className="hidden items-center gap-2 text-sm font-medium text-[var(--muted)] md:flex">
        {groups.map((group) => {
          const active = group.links.some((it) => isActive(pathname, it.href));
          return (
            <details key={group.key} className="group relative">
              <summary
                className={[
                  "list-none cursor-pointer rounded-full border px-3 py-2 text-xs transition",
                  "border-[color:var(--border)]",
                  active ? "bg-[var(--surface-strong)] text-[var(--text)]" : "bg-[var(--surface)] hover:bg-[var(--surface-strong)]",
                ].join(" ")}
              >
                {group.label}
              </summary>
              <div className="absolute right-0 top-11 z-50 min-w-44 rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-2 shadow-[var(--shadow)]">
                {group.links.map((link) => (
                  <Link
                    key={link.href}
                    href={link.href}
                    className={[
                      "block rounded-lg px-3 py-2 text-xs",
                      isActive(pathname, link.href)
                        ? "bg-[var(--surface-strong)] text-[var(--text)]"
                        : "text-[var(--muted)] hover:bg-[var(--surface-strong)] hover:text-[var(--text)]",
                    ].join(" ")}
                  >
                    {link.label}
                  </Link>
                ))}
              </div>
            </details>
          );
        })}
      </div>

      <details className="relative md:hidden">
        <summary className="list-none rounded-lg border border-[color:var(--border)] bg-[var(--surface)] px-3 py-2 text-xs">
          Menu
        </summary>
        <div className="absolute right-0 top-11 z-50 w-56 rounded-xl border border-[color:var(--border)] bg-[var(--surface)] p-2 shadow-[var(--shadow)]">
          {groups.map((group) => (
            <div key={group.key} className="mb-2">
              <div className="px-2 py-1 text-[10px] uppercase tracking-[0.12em] text-[var(--muted)]">{group.label}</div>
              {group.links.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className={[
                    "block rounded-lg px-3 py-2 text-xs",
                    isActive(pathname, link.href)
                      ? "bg-[var(--surface-strong)] text-[var(--text)]"
                      : "text-[var(--muted)] hover:bg-[var(--surface-strong)] hover:text-[var(--text)]",
                  ].join(" ")}
                >
                  {link.label}
                </Link>
              ))}
            </div>
          ))}
        </div>
      </details>

      <nav className="fixed inset-x-0 bottom-0 z-40 border-t border-[color:var(--border)] bg-[var(--glass)] px-2 py-2 backdrop-blur md:hidden">
        <div className="mx-auto grid max-w-md grid-cols-4 gap-2">
          {mobileTabs.map((tab) => (
            <Link
              key={tab.href}
              href={tab.href}
              className={[
                "flex min-h-11 items-center justify-center rounded-lg text-xs",
                isActive(pathname, tab.href)
                  ? "bg-[var(--surface-strong)] text-[var(--text)]"
                  : "text-[var(--muted)] hover:bg-[var(--surface)]",
              ].join(" ")}
            >
              {tab.label}
            </Link>
          ))}
        </div>
      </nav>
    </>
  );
}

