# Frontend Changes Mapping (From `docs/FRONTEND_REVIEW.md`)

This document maps review findings to implemented fixes in `services/polymarket/frontend`.

## 1. UI/UX

### 1.1 Navigation overcrowded + mobile no navigation
- Status: `DONE`
- Changes:
  - Grouped top navigation into Market/Trading/Strategy/Insights
  - Added mobile menu + bottom tab navigation
- Files:
  - `services/polymarket/frontend/components/AppNavigation.tsx`
  - `services/polymarket/frontend/app/layout.tsx`

### 1.2 Homepage no focus
- Status: `DONE`
- Changes:
  - Home page converted to dashboard style: KPI cards + latest opportunities + pending executions
  - Full catalog tables retained below overview blocks
- Files:
  - `services/polymarket/frontend/app/page.tsx`

### 1.3 KPI labels are developer jargon
- Status: `DONE`
- Changes:
  - Renamed labels like `total_plans/total_pnl_usd/avg_roi` to user-facing titles
- Files:
  - `services/polymarket/frontend/app/v2/analytics/page.tsx`
  - `services/polymarket/frontend/app/v2/review/page.tsx`

### 1.4 Table interactivity / empty state
- Status: `DONE`
- Changes:
  - Added row hover styles globally
  - Added stronger empty-state copy across main pages
  - Added lightweight sort hints (`↑↓`) on key table headers
- Files:
  - `services/polymarket/frontend/app/globals.css`
  - `services/polymarket/frontend/app/v2/opportunities/page.tsx`
  - `services/polymarket/frontend/app/v2/orders/page.tsx`

### 1.5 Color system too monotone
- Status: `DONE`
- Changes:
  - Added semantic tokens (`--color-profit`, `--color-loss`, `--color-warn`, `--color-info`)
  - Applied PnL semantic color usage to key pages
- Files:
  - `services/polymarket/frontend/app/globals.css`
  - `services/polymarket/frontend/app/v2/portfolio/page.tsx`
  - `services/polymarket/frontend/app/v2/review/page.tsx`
  - `services/polymarket/frontend/app/v2/analytics/page.tsx`

### 1.6 Spacing / border radius
- Status: `DONE`
- Changes:
  - Replaced oversized `rounded-[28px]` usage with `rounded-xl`
  - Normalized card/input/button radius usage
- Files:
  - `services/polymarket/frontend/app/**/*.tsx` (multi-page update)
  - `services/polymarket/frontend/app/globals.css`

### 1.7 Mobile broken (table overflow, no touch size)
- Status: `DONE`
- Changes:
  - Added mobile card layouts for major table views
  - Set global minimum touch target (`44px`) for interactive controls
- Files:
  - `services/polymarket/frontend/app/globals.css`
  - `services/polymarket/frontend/app/v2/opportunities/page.tsx`
  - `services/polymarket/frontend/app/v2/orders/page.tsx`
  - `services/polymarket/frontend/app/v2/review/page.tsx`
  - `services/polymarket/frontend/app/v2/signals/page.tsx`
  - `services/polymarket/frontend/app/v2/executions/page.tsx`
  - `services/polymarket/frontend/app/v2/portfolio/page.tsx`
  - `services/polymarket/frontend/app/v2/labels/page.tsx`
  - `services/polymarket/frontend/app/v2/settlements/page.tsx`
  - `services/polymarket/frontend/app/v2/automation/page.tsx`
  - `services/polymarket/frontend/app/v2/analytics/page.tsx`

## 2. Security

### 2.1 Token in localStorage
- Status: `PARTIAL (hardening done, full backend cookie auth depends on backend/login flow)`
- Changes:
  - Frontend switched to cookie-first requests (`credentials: include`)
  - Legacy localStorage token mode gated behind `NEXT_PUBLIC_LEGACY_TOKEN_AUTH=1`
  - Settings page now defaults to cookie auth guidance
- Files:
  - `services/polymarket/frontend/lib/api.ts`
  - `services/polymarket/frontend/app/v2/settings/page.tsx`

### 2.2 Raw backend errors exposed
- Status: `DONE`
- Changes:
  - Added unified API response parser and sanitized user-facing errors
  - Raw body only logged to console for debugging
- Files:
  - `services/polymarket/frontend/lib/api.ts`

### 2.3 Path injection in API calls
- Status: `DONE`
- Changes:
  - Added shared safe path helpers + validation
  - Applied to user-controlled path segments (labels/settlements/executions)
  - Applied URL encoding for strategy/rule names in strategy & automation APIs
- Files:
  - `services/polymarket/frontend/lib/path.ts`
  - `services/polymarket/frontend/app/v2/labels/page.tsx`
  - `services/polymarket/frontend/app/v2/settlements/page.tsx`
  - `services/polymarket/frontend/app/v2/executions/[id]/page.tsx`
  - `services/polymarket/frontend/app/v2/strategies/page.tsx`
  - `services/polymarket/frontend/app/v2/automation/page.tsx`

### Extra headers/CSP
- Status: `DONE`
- Changes:
  - Added CSP + standard hardening headers
- Files:
  - `services/polymarket/frontend/next.config.ts`

## 3. Performance

### 3.1 No request cancellation
- Status: `DONE` (major pages)
- Changes:
  - Added `AbortController` cancellation on unmount/filter changes
- Files:
  - `services/polymarket/frontend/app/v2/**/*.tsx` (major pages)

### 3.2 No request dedup
- Status: `DONE` (major pages)
- Changes:
  - Added loading guards (`loadingRef`) and centralized `useApi` helper
- Files:
  - `services/polymarket/frontend/lib/useApi.ts`
  - `services/polymarket/frontend/app/v2/**/*.tsx`

### 3.4 No skeleton loading
- Status: `DONE`
- Changes:
  - Added `.skeleton` utility and applied to key data pages
- Files:
  - `services/polymarket/frontend/app/globals.css`
  - `services/polymarket/frontend/app/v2/orders/page.tsx`
  - `services/polymarket/frontend/app/v2/opportunities/page.tsx`
  - `services/polymarket/frontend/app/v2/executions/page.tsx`
  - `services/polymarket/frontend/app/v2/automation/page.tsx`
  - `services/polymarket/frontend/app/v2/strategies/page.tsx`

## 4. Code Quality

### 4.1 No Error Boundary
- Status: `DONE`
- Files:
  - `services/polymarket/frontend/components/AppErrorBoundary.tsx`
  - `services/polymarket/frontend/app/layout.tsx`

### 4.2 Weak type safety (`unknown`)
- Status: `DONE` (high-impact pages)
- Changes:
  - Replaced key `unknown` fields with structured types in opportunities/journal/signals/strategies/executions detail
- Files:
  - `services/polymarket/frontend/app/v2/opportunities/page.tsx`
  - `services/polymarket/frontend/app/v2/journal/page.tsx`
  - `services/polymarket/frontend/app/v2/signals/page.tsx`
  - `services/polymarket/frontend/app/v2/strategies/page.tsx`
  - `services/polymarket/frontend/app/v2/executions/[id]/page.tsx`

### 4.3 Magic numbers
- Status: `DONE` (major defaults)
- Changes:
  - Added shared `DEFAULTS` constants and migrated key pages
- Files:
  - `services/polymarket/frontend/lib/constants.ts`
  - `services/polymarket/frontend/app/v2/opportunities/page.tsx`
  - `services/polymarket/frontend/app/v2/automation/page.tsx`
  - `services/polymarket/frontend/app/v2/journal/page.tsx`
  - `services/polymarket/frontend/app/v2/review/page.tsx`
  - `services/polymarket/frontend/app/v2/signals/page.tsx`
  - `services/polymarket/frontend/app/v2/executions/page.tsx`
  - `services/polymarket/frontend/app/v2/labels/page.tsx`

### 4.4 Inconsistent error handling
- Status: `DONE` (major pages)
- Changes:
  - Unified API error normalization in `lib/api.ts`
  - Introduced reusable `useApi` helper for consistent loading/error lifecycle
- Files:
  - `services/polymarket/frontend/lib/api.ts`
  - `services/polymarket/frontend/lib/useApi.ts`
  - `services/polymarket/frontend/app/v2/orders/page.tsx` (adopted)

## Verification

Run:

```bash
cd services/polymarket/frontend
npm run build
```

Latest result in this dev cycle: `PASS`.

