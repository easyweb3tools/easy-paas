"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export default function NavLink({
  href,
  label,
}: {
  href: string;
  label: string;
}) {
  const pathname = usePathname();
  const active = pathname === href || pathname.startsWith(`${href}/`);
  return (
    <Link
      href={href}
      className={[
        "rounded-full border px-3 py-1 text-xs font-medium transition",
        "border-[color:var(--border)]",
        active ? "bg-[var(--surface-strong)] text-[var(--text)]" : "bg-[var(--surface)] text-[var(--muted)] hover:bg-[var(--surface-strong)]",
      ].join(" ")}
    >
      {label}
    </Link>
  );
}

