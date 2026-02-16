# Frontend Review: UI/UX & Code Quality

Based on a full review of the live site (https://www.easyweb3.tools/) on desktop and mobile, plus a source code audit of `services/polymarket/frontend`.

---

## Part 1: UI/UX Issues (Apple Design Perspective)

### 1.1 Navigation: Overcrowded

The top nav bar crams 13 items in a single row. On mobile, the nav disappears entirely — users cannot switch pages.

**Current:**
```
Catalog | Opportunities | Strategies | Executions | Orders | Portfolio | Automation | Journal | Review | Analytics | Signals | Labels | Settlements
```

**Recommended:** Group into 3-4 primary tabs with dropdowns:

| Tab | Children |
|-----|----------|
| Market | Catalog, Signals, Labels, Settlements |
| Trading | Opportunities, Executions, Orders, Portfolio |
| Strategy | Strategies, Automation |
| Insights | Analytics, Journal, Review |

On mobile, use a bottom tab bar (4 icons) or a hamburger menu. Follow iOS HIG thumb-zone guidelines.

### 1.2 Homepage: No Focus

The homepage dumps a raw table of 6,000+ events. There is no summary, no highlights, no sense of "what matters right now".

**Recommended:** Replace with a Dashboard layout:
- Top row: 3-4 KPI cards (Active Opportunities, Today's P&L, Win Rate, Portfolio Value)
- Below: "Latest Opportunities" card list + "Pending Executions" card list
- Push the full catalog table to the dedicated Catalog page

### 1.3 KPI Labels: Developer Jargon

Cards display raw variable names: `total_plans`, `total_pnl_usd`, `avg_roi`.

**Recommended mapping:**

| Current | Recommended |
|---------|-------------|
| total_plans | Total Plans |
| total_pnl_usd | Total P&L |
| avg_roi | Avg ROI |
| win | Wins |
| loss | Losses |
| pending | Pending |

### 1.4 Tables: Lack Interactivity

- No hover highlight on rows
- No sort indicators (arrows) on column headers
- Empty state is just plain text "暂无数据" with no illustration or guidance
- No column resize, long text truncation is inconsistent

**Recommended:**
- Add hover background on table rows + cursor pointer for clickable rows
- Add sort arrow icons on sortable headers
- Use empty-state illustrations with helpful text (e.g., "No active opportunities. The strategy engine will scan automatically.")

### 1.5 Color System: Too Monotone

The entire UI is grayscale + one green badge. Critically, P&L numbers have no color coding — this is a fundamental expectation in any trading interface.

**Recommended semantic palette:**

| Token | Usage | Color |
|-------|-------|-------|
| `--color-profit` | Positive P&L, wins, active | Green `#34C759` |
| `--color-loss` | Negative P&L, losses, errors | Red `#FF3B30` |
| `--color-warn` | Warnings, pending | Amber `#FF9500` |
| `--color-info` | Informational, in-progress | Blue `#007AFF` |

Apply to:
- All P&L numbers (green if positive, red if negative)
- Strategy cards: left color bar by category (arbitrage=blue, systematic=purple, weather=teal)
- Status badges: active=green, closed=gray, error=red, pending=amber

### 1.6 Spacing & Border Radius

- `border-radius: 28px` is too large for a data-intensive tool — feels toy-like, not professional
- Card gaps are too wide (24px+), wasting screen real estate
- Font hierarchy is flat — titles and body text have similar visual weight

**Recommended:**
- Border radius: `12px` for cards, `8px` for inputs/buttons, `20px` for pills/badges
- Card gap: `12-16px`
- Font scale: Page Title `28px/700`, Section Title `17px/600`, Body `14px/400`, Caption `12px/400`

### 1.7 Mobile: Broken

- Navigation completely gone — no way to switch pages
- Tables overflow horizontally, require side-scrolling
- Buttons and inputs do not meet minimum touch target (44x44pt per iOS HIG)

**Recommended:**
- Mobile nav: bottom tab bar or hamburger menu
- Tables: switch to card-list layout on screens < 768px
- All tappable elements: minimum `44x44` points

---

## Part 2: Security Issues

### 2.1 [HIGH] Token Stored in localStorage

**File:** `lib/api.ts:17-37`

```typescript
const TOKEN_STORAGE_KEY = "easyweb3.auth_token";
export function setAuthToken(token: string) {
  window.localStorage.setItem(TOKEN_STORAGE_KEY, v);
}
```

localStorage is accessible to any JavaScript on the page. If an XSS vulnerability exists anywhere, all tokens are compromised.

**Fix:** Use httpOnly cookies set by the backend (not accessible to JS). If localStorage must be used, add a strict Content-Security-Policy header in `next.config.ts` to block inline scripts and limit script sources.

### 2.2 [HIGH] Raw Backend Errors Exposed to Users

**File:** `lib/api.ts:60`

```typescript
if (!res.ok) {
  const text = await res.text().catch(() => "");
  throw new Error(`HTTP ${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
}
```

Backend error bodies (SQL errors, stack traces, internal paths) are passed directly to the UI via `setError(e.message)`.

**Fix:** Show generic user-facing messages. Log the original error to `console.error` for debugging:

```typescript
if (!res.ok) {
  const text = await res.text().catch(() => "");
  console.error(`API error: ${res.status} ${res.statusText}`, text);
  throw new Error(res.status === 401 ? "Unauthorized" : "Request failed, please try again");
}
```

### 2.3 [HIGH] Path Injection in API Calls

**File:** `app/v2/labels/page.tsx:56`

```typescript
await apiPost(`/api/v2/markets/${marketId}/labels`, { ... });
```

`marketId` comes from user input (`useState`) and is interpolated directly into the URL path without validation or encoding.

**Fix:** Validate with regex before use, and always encode:

```typescript
if (!/^[a-zA-Z0-9_-]+$/.test(marketId)) {
  setError("Invalid market ID");
  return;
}
await apiPost(`/api/v2/markets/${encodeURIComponent(marketId)}/labels`, { ... });
```

Same pattern applies to other user-supplied path segments in `app/v2/settlements/page.tsx` and `app/v2/executions/[id]/page.tsx`.

---

## Part 3: Performance Issues

### 3.1 [MED] No Request Cancellation on Unmount

**Files:** All pages with `useEffect` + `apiGet`

When a user navigates away while a request is in-flight, the response still calls `setState` on an unmounted component.

**Fix:** Use `AbortController` in `useEffect`:

```typescript
useEffect(() => {
  const controller = new AbortController();
  apiGet("/api/v2/...", { signal: controller.signal })
    .then(setItems)
    .catch((e) => { if (e.name !== "AbortError") setError(e.message); });
  return () => controller.abort();
}, [query]);
```

### 3.2 [MED] No Request Deduplication

Rapid clicks on "刷新" fire multiple identical requests simultaneously.

**Fix:** Guard with a loading flag:

```typescript
const refresh = useCallback(async () => {
  if (loading) return;          // ← guard
  setLoading(true);
  // ...
}, [query, loading]);
```

### 3.3 [MED] All Pages Are Client Components

Every page is `"use client"`. This disables SSR/RSC, increases the JS bundle sent to the browser, and slows initial page load.

**Fix:** Refactor each page into:
- A Server Component that fetches initial data
- A Client Component child that handles interactivity (filters, buttons)

Example:
```
app/v2/opportunities/page.tsx        ← Server Component (fetches data)
app/v2/opportunities/OppsClient.tsx  ← "use client" (renders filters + table)
```

### 3.4 [LOW] No Loading Skeletons

All pages show "加载中..." plain text during data fetching.

**Fix:** Add skeleton placeholder divs that match the card/table layout with pulsing animation:

```css
.skeleton { background: var(--border); border-radius: 8px; animation: pulse 1.5s infinite; }
```

---

## Part 4: Code Quality Issues

### 4.1 [MED] No Error Boundary

The entire app has no React Error Boundary. Any component crash results in a white screen.

**Fix:** Create `components/ErrorBoundary.tsx` and wrap the app in `app/layout.tsx`:

```tsx
"use client";
import { Component, ReactNode } from "react";

export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };
  static getDerivedStateFromError(error: Error) { return { error }; }
  render() {
    if (this.state.error) {
      return <div className="p-8 text-center">
        <h2>Something went wrong</h2>
        <p className="text-sm text-[var(--muted)]">{this.state.error.message}</p>
        <button onClick={() => this.setState({ error: null })}>Retry</button>
      </div>;
    }
    return this.props.children;
  }
}
```

### 4.2 [MED] Weak Type Safety

Multiple types use `unknown` for complex fields:

```typescript
// app/v2/opportunities/page.tsx
type Opportunity = { Legs: unknown; ... };

// app/v2/signals/page.tsx
type Signal = { Payload: unknown; ... };

// app/v2/journal/page.tsx
type JournalEntry = { SignalSnapshot: unknown; MarketSnapshot: unknown; ... };
```

**Fix:** Define proper interfaces:

```typescript
interface Leg {
  token_id: string;
  direction: "BUY_YES" | "BUY_NO";
  target_price: string;
  size_usd: string;
}

type Opportunity = { Legs: Leg[]; ... };
```

### 4.3 [LOW] Hardcoded Magic Numbers

Default values scattered across pages:

| File | Value | Usage |
|------|-------|-------|
| `app/v2/opportunities/page.tsx` | `"0.05"` | min edge default |
| `app/v2/journal/page.tsx` | `"100"` | page limit |
| `app/v2/labels/page.tsx` | `200` | page limit |
| `app/v2/automation/page.tsx` | `0.8` | min confidence |
| `app/v2/automation/page.tsx` | `72` | max hold hours |

**Fix:** Extract to `lib/constants.ts`:

```typescript
export const DEFAULTS = {
  PAGE_LIMIT: 50,
  MIN_EDGE: "0.05",
  MIN_CONFIDENCE: 0.8,
  MAX_HOLD_HOURS: 72,
  STOP_LOSS_PCT: "0.10",
  TAKE_PROFIT_PCT: "0.20",
} as const;
```

### 4.4 [LOW] Inconsistent Error Handling

Some pages display errors inline, some swallow them, some throw without catching:

- `app/v2/opportunities/page.tsx` — catches and shows via `setError`
- `app/v2/journal/page.tsx` — catches and shows via `setError`
- `app/v2/settlements/page.tsx` — catches but error display may be missed
- `app/page.tsx` — error shown inline

**Fix:** Create a shared `useApi` hook that standardizes loading/error/data state:

```typescript
function useApi<T>(fetcher: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  // ... standardized fetch with abort, dedup, error mapping
  return { data, error, loading, refresh };
}
```

---

## Summary

| Category | High | Medium | Low |
|----------|------|--------|-----|
| Security | 3 | — | — |
| Performance | — | 3 | 1 |
| Code Quality | — | 2 | 2 |
| UI/UX | — | — | 7 |

### Recommended Priority Order

1. **Token storage** → switch to httpOnly cookie or add CSP (security)
2. **Error message sanitization** → stop leaking backend errors (security)
3. **Input validation** → encode/validate path params (security)
4. **Error Boundary** → prevent white-screen crashes (stability)
5. **Navigation redesign** → group into 4 tabs (UX)
6. **Color system** → add semantic colors for P&L (UX)
7. **Request lifecycle** → add abort + dedup (performance)
8. **Type safety + constants** → improve maintainability (code quality)
