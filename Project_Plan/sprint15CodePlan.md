# Plomvix — Sprint 15 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | TypeScript | React 18 | Vite | Tailwind CSS v4 | shadcn/ui | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–14 are complete. Sprint 11 added the UI foundation. Sprint 12 added the
Theme Engine and Developer Design Panel. Sprint 13 added cookie auth, login/logout UI,
protected app routes, an auth provider, and a centralized frontend API client.
The Sprint 13 patch migrated the existing Sprint 11–13 UI to **Tailwind CSS v4 + shadcn/ui**.
Sprint 14 added the Admin UI using that Tailwind/shadcn baseline.

Sprint 15 adds the **Log Explorer UI**. This sprint does not add new backend storage
or query capabilities. It builds a browser interface on top of the existing Sprint 6
query APIs and the authenticated Tailwind/shadcn UI shell from Sprints 11–14.

**What Sprint 15 delivers:**
- `/app/explore` authenticated React route
- Log Explorer page using the existing app shell, route registry, Tailwind utilities, shadcn/ui components, theme variables, toasts, and API client
- Time range picker:
  - Last 15m
  - Last 1h
  - Last 6h
  - Last 24h
  - Last 7d
  - Custom from/to
- Search bar mapped to the Sprint 6 `filter` query parameter
- Filter chips parsed from the search bar and removable
- Results table with:
  - timestamp
  - level
  - message
  - dynamic additional fields from schema/records
- Expandable/detail dialog showing full JSON
- Tail mode using 5-second polling
- Pagination:
  - previous/next
  - total count visible
  - current range summary
- Data from `GET /query/logs`
- Optional schema data from `GET /query/schema/logs` when available
- Empty state
- Loading skeleton
- Error state with retry button
- Toasts through the existing Sprint 11 app event system
- Frontend pure utility tests when the UI test runner exists
- `docs/api/log-explorer.md` documenting the UI behaviour
- README update for Log Explorer

**What Sprint 15 does NOT do:**
- No new backend query endpoints
- No WebSocket or server-sent events — tail mode is polling only
- No saved searches
- No advanced query language beyond Sprint 6 filter syntax
- No OR filters
- No aggregation charts
- No metrics explorer
- No JSON explorer
- No trace UI — Sprint 16
- No OTLP or Prometheus remote write — Sprint 17
- No new third-party table/grid/date/state dependencies
- No localStorage persistence for filters or results in Sprint 15
- No second toast system such as Sonner
- No custom large CSS file for Log Explorer layout

---

## TAILWIND + SHADCN BASELINE — READ BEFORE WRITING ANY CODE

Sprint 15 assumes `sprint13PatchCodePlan.md` has been applied and Sprint 14 has
completed successfully. Do not reinstall or reinitialize Tailwind or shadcn in this
sprint unless a required component file is missing.

The Log Explorer UI must use:
- Tailwind utility classes
- shadcn/ui components under `ui/src/components/ui/`
- the existing `cn()` helper from `@/lib/utils`
- Plomvix theme variables exposed through shadcn-compatible CSS variables
- the existing app event/toast system from Sprint 11
- the existing API client from Sprint 13

**Allowed shadcn/ui primitives for Sprint 15:**
- `Button`
- `Input`
- `Label`
- `Card`
- `Table`
- `Badge`
- `Dialog`
- `Skeleton`
- `Alert`
- `Separator`
- `Textarea`

**Important styling rule:**
Use classes such as `bg-background`, `text-foreground`, `text-muted-foreground`,
`border-border`, `bg-card`, `text-card-foreground`, `bg-primary`, and
`bg-muted`. These must resolve through the Sprint 12 theme variable chain. Do not
hardcode a parallel Plomvix color palette in React components.

**No custom CSS rule:**
Do not create a large Log Explorer CSS file. Normal Log Explorer styling must be
Tailwind classes. Small global fixes are allowed only if needed for an existing
app-shell issue.

---

## EXISTING API CONTRACTS — READ BEFORE WRITING ANY CODE

Sprint 15 must consume APIs that already exist. Do not invent new route names.
Use the centralized frontend API client from Sprint 13 so cookie auth, best-effort
refresh-on-401, Plomvix response-envelope parsing, and session-expired handling stay consistent.

**Query APIs from Sprint 6:**

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `GET` | `/query/logs` | JWT cookie or API key | Query log records |
| `GET` | `/query/schema/logs` | JWT cookie or API key | Return inferred log schema |

**`GET /query/logs` query params:**

| Param | Type | Notes |
|---|---|---|
| `from` | Unix nanoseconds string | Optional; `0` means beginning of time |
| `to` | Unix nanoseconds string | Optional; defaults to now if omitted |
| `filter` | string | Sprint 6 filter expression, e.g. `level=error AND service=api` |
| `limit` | positive integer | Default 100, max 10000 on backend |
| `offset` | non-negative integer | Number of matching records to skip |

**Query result shape after API-client unwrapping:**

```ts
interface QueryResult {
  records: Array<Record<string, unknown>>
  count: number
  total: number
  limit: number
  offset: number
  query_ms: number
  data_type: string
}
```

**Important JavaScript timestamp rule:**
Unix nanoseconds exceed JavaScript's safe integer range. All outgoing `from` and
`to` query params must be built as decimal strings using `BigInt`, not floating-point
numbers. Display helpers must accept string, number, or bigint-like values defensively.
Do not perform nanosecond arithmetic with `number`.

---

## FILTER SYNTAX — READ BEFORE WRITING ANY CODE

Sprint 15 does not create a new query language. The search bar maps directly to the
Sprint 6 filter syntax.

**Supported operators:**

| Operator | Example |
|---|---|
| `=` | `level=error` |
| `!=` | `level!=debug` |
| `>` | `duration_ms>100` |
| `<` | `duration_ms<500` |
| `>=` | `status_code>=500` |
| `<=` | `latency_ms<=250` |

Multiple conditions are combined with literal ` AND `:

```text
level=error AND service=api
status_code>=500 AND service=gateway
```

**UI filter-chip rule:**
Filter chips are a UI convenience only. They parse and remove ` AND `-separated
conditions. The final source of truth remains the search bar string. Removing a chip
rebuilds the search bar by joining remaining conditions with ` AND `.

**Duplicate condition rule:**
Filter chip IDs must include the condition index, not just the condition text. This
allows repeated conditions to be removed one at a time without removing all duplicates.

---

## ROUTING DESIGN — READ BEFORE WRITING ANY CODE

Sprint 13 uses the authenticated app router with an `/app` browser prefix.
Therefore:

| Browser URL | React route registry path |
|---|---|
| `/app/explore` | `/explore` |

Do **not** put `/app/explore` in the route registry path. Use `/explore` there.
The `/app` prefix is provided by the router setup from Sprint 13.

If Sprint 11 already created a temporary Explore route, replace its temporary
component with the real Log Explorer page. If no route exists, add it through the
same route registry used by the sidebar. The sidebar must not hardcode `/explore`.

---

## FRONTEND DESIGN — READ BEFORE WRITING ANY CODE

The Log Explorer must use existing infrastructure from earlier UI sprints:
- app shell/sidebar/header from Sprint 11
- app event/toast system from Sprint 11
- theme CSS variables from Sprint 12
- auth provider and protected routes from Sprint 13
- centralized API client from Sprint 13
- Tailwind/shadcn primitives from Sprint 13 Patch and Sprint 14

**State-management rule:**
Use local component state plus small custom hooks. Do not add Redux, Zustand, React
Query, TanStack Query, or another state-management dependency in Sprint 15.

**Table rule:**
Use shadcn `Table` components, which render semantic HTML table markup. Do not add
a third-party virtualized table/grid dependency in Sprint 15.

**Date formatting rule:**
Use browser `Intl.DateTimeFormat`. Do not add a date library.

**Tail mode rule:**
Tail mode is client-side polling every 5 seconds. It must clean up timers on unmount
and when tail mode is disabled. Tail mode re-runs the current query with a rolling
relative time range when the active preset is relative. For custom ranges, tail mode
keeps the custom `from` fixed and advances only `to` to current time on each poll.
Tail mode must update visible range state before querying so the UI matches the
request actually sent.

---

## TASK 01 — Verify Tailwind + shadcn baseline from earlier UI sprints

**Action:** In `ui/`, verify that the Tailwind/shadcn baseline exists and is pinned.
Do not use `latest`, `^`, or `~` while checking or adding anything.

**Verify:**
```bash
cd ui
node -e "const p=require('./package.json'); const all={...p.dependencies,...p.devDependencies}; for (const k of ['tailwindcss','@tailwindcss/vite','shadcn','class-variance-authority','clsx','tailwind-merge','lucide-react']) { if (!all[k]) throw new Error(k+' missing'); if (/^[~^]|latest/.test(all[k])) throw new Error(k+' is not pinned: '+all[k]); }"
test -f components.json
test -f src/lib/utils.ts
grep -R "@import \"tailwindcss\"" src
npm run typecheck
```

---

## TASK 02 — Ensure required shadcn/ui primitives exist

**Action:** In `ui/`, ensure these shadcn/ui component files exist:

```text
src/components/ui/button.tsx
src/components/ui/input.tsx
src/components/ui/label.tsx
src/components/ui/card.tsx
src/components/ui/table.tsx
src/components/ui/badge.tsx
src/components/ui/dialog.tsx
src/components/ui/skeleton.tsx
src/components/ui/alert.tsx
src/components/ui/separator.tsx
src/components/ui/textarea.tsx
```

If any are missing, add only the missing components using the exact installed shadcn
CLI version from `package.json`. Do not use `latest`. Do not overwrite existing shadcn
files that were customized by Sprint 13 Patch or Sprint 14.

Example if the installed version is `4.8.3`:

```bash
npx --yes shadcn@4.8.3 add button input label card table badge dialog skeleton alert separator textarea
```

**Verify:**
```bash
cd ui
for f in \
  src/components/ui/button.tsx \
  src/components/ui/input.tsx \
  src/components/ui/label.tsx \
  src/components/ui/card.tsx \
  src/components/ui/table.tsx \
  src/components/ui/badge.tsx \
  src/components/ui/dialog.tsx \
  src/components/ui/skeleton.tsx \
  src/components/ui/alert.tsx \
  src/components/ui/separator.tsx \
  src/components/ui/textarea.tsx; do test -f "$f" || exit 1; done
npm run typecheck
```

---

## TASK 03 — Confirm existing UI auth/client/route exports

**Action:** Inspect the current UI files from Sprints 11–14 and identify exact names
and exports for:
- route registry
- existing temporary Explore page, if any
- sidebar component
- centralized API client
- app event/toast hook
- auth provider/protected route behaviour
- shadcn Button, Card, Table, Dialog, Badge, Skeleton, Alert imports

If names differ from this plan, adapt imports to the existing codebase names while
preserving Sprint 15 behaviour.

**Verify:** Write down the exact paths in terminal output or a temporary note; no
code changes required.

---

## TASK 04 — Create ui/src/logs directory

**Action:** Create the Log Explorer directory structure:

```bash
mkdir -p ui/src/logs
mkdir -p ui/src/logs/components
mkdir -p ui/src/logs/hooks
mkdir -p ui/src/logs/utils
```

**Verify:** `ls ui/src/logs ui/src/logs/components ui/src/logs/hooks ui/src/logs/utils` shows the directories.

---

## TASK 05 — Create ui/src/logs/types.ts

**Action:** Create `ui/src/logs/types.ts`.

**Exports required:**

```ts
export type TimePreset = '15m' | '1h' | '6h' | '24h' | '7d' | 'custom'

export interface TimeRangeState {
  preset: TimePreset
  fromNs: string
  toNs: string
}

export interface LogQueryParams {
  from?: string
  to?: string
  filter?: string
  limit: number
  offset: number
}

export interface LogRecord {
  [key: string]: unknown
}

export interface LogQueryResult {
  records: LogRecord[]
  count: number
  total: number
  limit: number
  offset: number
  query_ms: number
  data_type: string
}

export interface LogSchema {
  data_type: string
  fields: Record<string, string>
  updated_at?: string
  record_count?: number
}

export interface FilterChip {
  id: string
  index: number
  expression: string
  field: string
  operator: string
  value: string
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 06 — Create ui/src/logs/utils/timeRange.ts

**Action:** Create `ui/src/logs/utils/timeRange.ts`.

**Exports required:**

```ts
export const NS_PER_MS = 1_000_000n
export function nowNs(): string
export function presetToRange(preset: Exclude<TimePreset, 'custom'>): TimeRangeState
export function updateRollingRange(range: TimeRangeState): TimeRangeState
export function dateInputToNs(value: string): string
export function nsToDateInput(ns: string): string
export function formatTimestamp(value: unknown): string
export function isValidCustomRange(fromNs: string, toNs: string): boolean
```

**Behaviour:**
- Use `BigInt(Date.now()) * NS_PER_MS` for `nowNs()`
- Presets subtract nanosecond durations using `BigInt`
- `updateRollingRange()` keeps `fromNs` and `toNs` rolling for relative presets
- `updateRollingRange()` keeps custom `fromNs` fixed and advances only custom `toNs` to `nowNs()`
- `dateInputToNs()` accepts the value from `<input type="datetime-local">`
- `dateInputToNs()` must return `""` for invalid or empty input; it must not throw during typing
- `nsToDateInput()` returns a local datetime string suitable for the input
- `nsToDateInput()` must return `""` for invalid input; it must not throw during rendering
- `formatTimestamp()` handles string, number, bigint, `Date`, missing, and invalid values
- When `formatTimestamp()` receives a JavaScript `number`, treat it as already lossy and use it only for best-effort display; never convert it back into query params
- `isValidCustomRange()` compares nanosecond strings with `BigInt` and returns false for invalid/empty inputs
- Never do nanosecond arithmetic with JavaScript `number`

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 07 — Create ui/src/logs/utils/filterChips.ts

**Action:** Create `ui/src/logs/utils/filterChips.ts`.

**Exports required:**

```ts
export function parseFilterChips(filter: string): FilterChip[]
export function removeFilterChip(filter: string, chipID: string): string
export function normalizeFilter(filter: string): string
```

**Behaviour:**
- Split conditions on literal ` AND ` with surrounding spaces
- Trim whitespace around each condition
- Parse operators longest-first: `>=`, `<=`, `!=`, `=`, `>`, `<`
- Preserve unparseable conditions as chips with field `""`, operator `""`, value equal to the condition
- Chip ID must include the original condition index, for example `${index}:${condition}`
- Removing a chip removes only the chip matching that exact ID and rejoins remaining conditions with ` AND `
- `normalizeFilter()` collapses empty conditions and returns a clean filter string

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 08 — Create ui/src/logs/utils/logFields.ts

**Action:** Create `ui/src/logs/utils/logFields.ts`.

**Exports required:**

```ts
export const CORE_LOG_FIELDS = ['timestamp', 'level', 'message'] as const
export function getRecordValue(record: LogRecord, key: string): unknown
export function stringifyCellValue(value: unknown): string
export function deriveDynamicColumns(records: LogRecord[], schema?: LogSchema): string[]
export function getLogLevel(record: LogRecord): string
export function getLogMessage(record: LogRecord): string
export function getLogTimestamp(record: LogRecord): unknown
```

**Behaviour:**
- `deriveDynamicColumns()` returns schema fields first when schema exists, then fields found in records
- Exclude core fields from dynamic columns
- De-duplicate fields while preserving the schema-first group order
- Sort schema-derived fields alphabetically and record-discovered fields alphabetically for stable rendering
- Limit dynamic columns to a maximum of 8 in Sprint 15 to avoid unusable tables
- `stringifyCellValue()` JSON-stringifies objects/arrays and handles null/undefined safely

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 09 — Create ui/src/logs/api.ts

**Action:** Create `ui/src/logs/api.ts`.

**Exports required:**

```ts
export async function queryLogs(params: LogQueryParams): Promise<LogQueryResult>
export async function fetchLogSchema(): Promise<LogSchema | null>
```

**Behaviour:**
- Use the centralized Sprint 13 API client, not raw `fetch`, for JSON API calls
- Build query strings with `URLSearchParams`
- Pass `from`, `to`, `filter`, `limit`, and `offset` only when set
- Encode filter safely through `URLSearchParams`, not manual string concatenation
- `queryLogs()` calls `GET /query/logs`
- `fetchLogSchema()` calls `GET /query/schema/logs`
- If `fetchLogSchema()` receives 404 or validation-style failure, return `null` and let the UI continue without schema
- Do not emit session-expired toasts here; the centralized API client handles session-expired events

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 10 — Create ui/src/logs/hooks/useLogQuery.ts

**Action:** Create `ui/src/logs/hooks/useLogQuery.ts`.

**Hook API:**

```ts
export function useLogQuery(): {
  result: LogQueryResult | null
  schema: LogSchema | null
  loading: boolean
  error: string | null
  params: LogQueryParams
  setParams: React.Dispatch<React.SetStateAction<LogQueryParams>>
  runQuery: (nextParams?: Partial<LogQueryParams>) => Promise<void>
  retry: () => Promise<void>
}
```

**Behaviour:**
- Holds current query params with defaults: `limit=100`, `offset=0`
- Loads schema once on first use; schema failure is non-fatal
- `runQuery()` merges `nextParams` into existing params and immediately queries with the synchronously merged object
- Do not query using stale React state after calling `setParams()`
- On a new search/filter/time range, caller must reset `offset` to `0`
- Uses a request sequence number or abort guard so older responses do not overwrite newer ones
- Converts thrown API errors to readable error strings

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 11 — Create ui/src/logs/hooks/useTailMode.ts

**Action:** Create `ui/src/logs/hooks/useTailMode.ts`.

**Hook API:**

```ts
export function useTailMode(options: {
  enabled: boolean
  range: TimeRangeState
  onTick: (range: TimeRangeState) => void | Promise<void>
}): void
```

**Behaviour:**
- Starts a `setInterval` at 5 seconds only when `enabled === true`
- Calls `updateRollingRange(range)` before invoking `onTick`
- Cleans up interval on unmount and when dependencies change
- Does not run overlapping ticks; skip a tick if the previous tick is still in flight
- The in-flight guard must reset in `finally`, even when `onTick` rejects
- Does not add a WebSocket dependency

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 12 — Create ui/src/logs/components/TimeRangePicker.tsx

**Action:** Create `ui/src/logs/components/TimeRangePicker.tsx`.

**Props:**

```ts
interface TimeRangePickerProps {
  value: TimeRangeState
  onChange: (value: TimeRangeState) => void
}
```

**Behaviour:**
- Use shadcn `Button`, `Input`, `Label`, `Card`, and `Badge` where useful
- Use Tailwind classes only for layout/styling
- Renders preset buttons for 15m, 1h, 6h, 24h, 7d, custom
- For custom, shows `datetime-local` from/to inputs
- On preset change, calculate range using `presetToRange()`
- On custom change, convert inputs to nanosecond strings
- Validate custom `from < to` in the UI; show inline error with `text-destructive` if invalid
- Does not execute the query itself; parent owns query submission

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 13 — Create ui/src/logs/components/FilterSearchBar.tsx

**Action:** Create `ui/src/logs/components/FilterSearchBar.tsx`.

**Props:**

```ts
interface FilterSearchBarProps {
  value: string
  onChange: (value: string) => void
  onSubmit: (normalizedFilter: string) => void
}
```

**Behaviour:**
- Use shadcn `Input`, `Button`, and `Label`
- Text input hint text: `level=error AND service=api`
- Submit on Enter and on Search button click
- Show a small helper line listing supported operators with `text-muted-foreground`
- Does not validate the full backend filter grammar; backend remains source of truth
- Calls `normalizeFilter()` before submit and passes the normalized string to `onSubmit(normalizedFilter)`
- Do not rely on `onChange()` state propagation before submit; parent must receive the normalized string directly

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 14 — Create ui/src/logs/components/FilterChips.tsx

**Action:** Create `ui/src/logs/components/FilterChips.tsx`.

**Props:**

```ts
interface FilterChipsProps {
  filter: string
  onChange: (nextFilter: string) => void
}
```

**Behaviour:**
- Uses `parseFilterChips()`
- Renders one removable shadcn `Badge` per condition
- Use a small shadcn `Button` or inline button for removal with accessible `aria-label`
- If no chips exist, renders nothing
- Removing a chip calls `removeFilterChip()` and `onChange()`
- Does not execute the query itself; parent decides when to run

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 15 — Create ui/src/logs/components/LogLevelBadge.tsx

**Action:** Create `ui/src/logs/components/LogLevelBadge.tsx`.

**Behaviour:**
- Accepts `level: string`
- Uses shadcn `Badge`
- Normalizes level to lowercase for display styling
- Supports at least: `trace`, `debug`, `info`, `warn`, `warning`, `error`, `fatal`
- Use theme-aware Tailwind classes only; unknown levels render as neutral badges
- Do not hardcode a parallel palette outside Tailwind/shadcn theme tokens

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 16 — Create ui/src/logs/components/JsonDialog.tsx

**Action:** Create `ui/src/logs/components/JsonDialog.tsx`.

**Props:**

```ts
interface JsonDialogProps {
  record: LogRecord | null
  open: boolean
  onOpenChange: (open: boolean) => void
}
```

**Behaviour:**
- Uses shadcn `Dialog`, `Button`, and `Textarea` or a themed `<pre>` block
- Shows formatted JSON with two-space indentation
- Includes Copy JSON button using `navigator.clipboard.writeText`
- Emits success/error toast via existing Sprint 11 app event system
- Does not add a JSON viewer dependency
- Handles `record === null` gracefully

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 17 — Create ui/src/logs/components/LogTable.tsx

**Action:** Create `ui/src/logs/components/LogTable.tsx`.

**Props:**

```ts
interface LogTableProps {
  records: LogRecord[]
  schema: LogSchema | null
}
```

**Behaviour:**
- Uses shadcn `Table`, `Button`, `Card`, and `Separator` where useful
- Renders columns: timestamp, level, message, dynamic additional fields, actions
- Uses `formatTimestamp()`, `getLogLevel()`, `getLogMessage()`, and `deriveDynamicColumns()`
- Each row has a View JSON action that opens `JsonDialog`
- Handles missing timestamp/level/message gracefully
- Uses a responsive wrapper with `overflow-x-auto`
- Does not virtualize rows in Sprint 15

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 18 — Create ui/src/logs/components/PaginationControls.tsx

**Action:** Create `ui/src/logs/components/PaginationControls.tsx`.

**Props:**

```ts
interface PaginationControlsProps {
  limit: number
  offset: number
  total: number
  count: number
  onPageChange: (nextOffset: number) => void
}
```

**Behaviour:**
- Uses shadcn `Button`
- Previous button disabled when `offset === 0`
- Next button disabled when `offset + count >= total`
- Shows `Showing X–Y of Z`
- Computes next/previous offsets using `limit`
- Does not allow negative offsets

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 19 — Create ui/src/logs/components/LogExplorerToolbar.tsx

**Action:** Create `ui/src/logs/components/LogExplorerToolbar.tsx`.

**Props:**

```ts
interface LogExplorerToolbarProps {
  range: TimeRangeState
  filter: string
  tailEnabled: boolean
  loading: boolean
  onRangeChange: (range: TimeRangeState) => void
  onFilterChange: (filter: string) => void
  onSearch: (normalizedFilter?: string) => void
  onTailChange: (enabled: boolean) => void
  onRefresh: () => void
}
```

**Behaviour:**
- Uses shadcn `Card`, `Button`, `Badge`, and `Separator`
- Composes `TimeRangePicker`, `FilterSearchBar`, `FilterChips`, Tail toggle, and Refresh button
- `FilterSearchBar` submit passes the normalized filter to `onSearch(normalizedFilter)`
- Search and Refresh buttons are disabled while loading
- Tail toggle clearly states polling interval: `5s`
- Tail mode does not hide manual refresh

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 20 — Create ui/src/logs/components/LogExplorerStates.tsx

**Action:** Create `ui/src/logs/components/LogExplorerStates.tsx`.

**Exports required:**

```tsx
export function LogLoadingSkeleton(): React.ReactElement
export function LogEmptyState(): React.ReactElement
export function LogErrorState({ message, onRetry }: { message: string; onRetry: () => void }): React.ReactElement
```

**Behaviour:**
- Loading skeleton uses shadcn `Skeleton`, `Card`, and themed rows
- Empty state uses shadcn `Card` and explains how to ingest logs and adjust time range
- Error state uses shadcn `Alert` and `Button` for retry
- Use Tailwind classes only for layout/styling

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 21 — Create ui/src/pages/ExplorePage.tsx

**Action:** Create or replace `ui/src/pages/ExplorePage.tsx`.

**Behaviour:**
- Page title: `Log Explorer`
- Short description
- Use shadcn `Card` layout and Tailwind spacing utilities
- Initial range: last 15 minutes
- Initial filter: empty
- Initial limit: 100
- Initial offset: 0
- Uses `useLogQuery()`
- Runs an initial query on mount
- Changing time range or filter resets offset to 0
- Search button runs query using the normalized filter string received from `FilterSearchBar`; do not rely on an async state update before querying
- Filter chip removal updates filter state and immediately refreshes results with `offset=0`
- Pagination controls run query with updated offset
- Tail mode uses `useTailMode()` to poll every 5 seconds
- Tail mode updates visible range state before querying
- Invalid custom ranges must not trigger queries
- Shows loading skeleton, error state, empty state, or `LogTable`
- Emits toast on successful manual refresh and on query errors

**Important:** Avoid infinite query loops. Do not include non-stable object literals in
`useEffect` dependency arrays. Use callbacks or explicit submit handlers.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 22 — Update route registry for Log Explorer

**Action:** Update the Sprint 11 route registry, usually `ui/src/app/routes.tsx`.

**Requirements:**
- Import `ExplorePage`
- If a temporary Explore route already exists, replace its element with `<ExplorePage />`
- If no route exists, add:

```tsx
{
  path: '/explore',
  label: 'Explore',
  element: <ExplorePage />,
  nav: true,
  group: 'Data',
}
```

**Important:** The route path is `/explore`, not `/app/explore`, because the router
basename/app route prefix is already handled by Sprint 13.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 23 — Update sidebar metadata handling if needed

**Action:** Update the sidebar component only if needed.

**Requirements:**
- Ensure the Explore route appears in the authenticated sidebar
- Preserve existing admin/dev route filtering from Sprints 12–14
- Do not hardcode `/explore`; use route registry metadata
- Preserve route ordering and grouping
- Use existing shadcn/Tailwind sidebar styles from Sprint 13 Patch/Sprint 14

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 24 — Polish Log Explorer Tailwind layout

**Action:** Review Log Explorer components and make Tailwind-only layout/styling fixes.

**Requirements:**
- Use `bg-background`, `bg-card`, `text-foreground`, `text-muted-foreground`, `border-border`, and `shadow-sm`
- Keep mobile/narrow viewport usable with `overflow-x-auto` around the table
- Do not add a new `logExplorer.css` file unless absolutely necessary for a tiny app-shell bug
- Do not use inline styles for the whole page except for tiny dynamic values when unavoidable
- Do not introduce a second theme token system

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 25 — Add log query utility tests if UI test runner exists

**Action:** If the UI test runner exists from Sprint 13 or 14, add tests for
`ui/src/logs/utils/timeRange.ts` and `ui/src/logs/utils/filterChips.ts`.

**Tests required:**
- `presetToRange('15m')` returns `fromNs < toNs`
- custom range validation rejects `fromNs >= toNs`
- invalid datetime input returns `""` and does not throw
- time range outputs are decimal strings
- no outgoing range value uses unsafe JS number arithmetic
- `parseFilterChips('level=error AND service=api')` returns two chips
- duplicate filters get distinct chip IDs
- operator parsing checks `>=` before `>`
- removing one duplicate chip leaves the other duplicate chip intact
- removing one chip rejoins remaining filters with ` AND `
- empty filter returns no chips

**Important:** Test files must import Vitest globals explicitly:

```ts
import { describe, expect, it } from 'vitest'
// Import `vi` only in files that actually use mocks/timers.
```

If no UI test runner exists, do not add a new dependency in Sprint 15. Document UI
tests as deferred in `docs/api/log-explorer.md`.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

If tests exist:

```bash
cd ui
npm test -- --run
```

---

## TASK 26 — Add log field utility tests if UI test runner exists

**Action:** If the UI test runner exists, add tests for `ui/src/logs/utils/logFields.ts`.

**Tests required:**
- Core fields are excluded from dynamic columns
- Schema fields are preferred over record-discovered fields
- Dynamic columns are sorted stably
- Dynamic columns are limited to 8
- `stringifyCellValue()` handles objects, arrays, null, undefined, and strings
- `getLogLevel()` returns a safe fallback for missing level
- Timestamp formatting handles string nanoseconds and invalid values in `timeRange.ts` tests

If no UI test runner exists, do not add a new dependency in Sprint 15.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

If tests exist:

```bash
cd ui
npm test -- --run
```

---

## TASK 27 — Update docs/api/log-explorer.md

**Action:** Create `docs/api/log-explorer.md`.

**Document:**
- What the Log Explorer does
- Browser URL `/app/explore`
- Auth requirement
- Tailwind/shadcn UI baseline
- Time range presets
- Custom time range behaviour
- BigInt nanosecond handling note
- Filter syntax and examples
- Filter chips behaviour
- Tail mode polling every 5 seconds
- Pagination behaviour
- Dynamic columns from schema/records
- Expanded row JSON dialog
- Empty/loading/error states
- Known Sprint 15 limitations: no OR filters, no saved searches, no WebSockets, no charts
- If UI tests are deferred because no runner exists, note that here

**Verify:**
```bash
grep -n "Log Explorer" docs/api/log-explorer.md
grep -n "/app/explore" docs/api/log-explorer.md
grep -n "Tailwind" docs/api/log-explorer.md
grep -n "Tail mode" docs/api/log-explorer.md
grep -n "level=error" docs/api/log-explorer.md
```

---

## TASK 28 — Update README.md with Log Explorer section

**Action:** Update `README.md`.

**Add:**
- Short Log Explorer overview
- Link/reference to `/app/explore`
- Mention filter syntax examples
- Mention tail mode polling
- Mention that the UI uses Tailwind/shadcn components after Sprint 13 Patch
- Link/reference to `docs/api/log-explorer.md`

**Verify:**
```bash
grep -n "Log Explorer" README.md
grep -n "/app/explore" README.md
grep -n "docs/api/log-explorer.md" README.md
```

---

## TASK 29 — Add Log Explorer to any UI navigation docs

**Action:** If Sprint 11–14 created UI navigation documentation, update it to include
Log Explorer. If no such documentation exists, no change is required.

**Verify:** Search docs for stale temporary-route language:

```bash
if grep -R "temporary Explore\|Explore temporary" docs README.md ui/src; then
  echo "stale temporary Explore wording found"
  exit 1
fi
```

There must be no stale user-facing text claiming Explore is only temporary or unfinished.

---

## TASK 30 — Add optional integration smoke test for Log Explorer backend flow

**Action:** If Sprint 10 integration scaffolding exists and is compatible with the
current server constructor, add or update `tests/integration/log_explorer_test.go`.

**Test flow:**
1. Start test server with isolated temp storage
2. Login as admin
3. Ingest two log records through `POST /ingest/logs`
4. Query `GET /query/logs?from=0&limit=100&offset=0`
5. Assert the response contains at least two records
6. Query `GET /query/logs?filter=level=error&limit=100&offset=0`
7. Assert only error-level matching records are returned
8. Query `GET /query/schema/logs`
9. Assert schema endpoint returns 200 or acceptable existing schema shape

**Important:** This is a backend/API smoke test for the flow used by the UI. It does
not require a browser.

If integration scaffolding is not present or is stale, document this as deferred in
`docs/api/log-explorer.md` and rely on existing query API tests plus UI build checks.

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/...
```

If integration tests are deferred, `docs/api/log-explorer.md` must state why.

---

## TASK 31 — Run UI typecheck and build

**Action:** Run:

```bash
cd ui
npm install
npm run typecheck
npm run build
```

**Verify:**
- TypeScript exits with zero errors
- Vite build exits with zero errors
- `ui/dist/` exists

---

## TASK 32 — Run Go tests

**Action:** Run from project root:

```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 go test ./...
```

**Verify:** All commands exit with code 0.

---

## TASK 33 — Run lint and full build

**Action:** Run from project root:

```bash
CGO_ENABLED=1 make lint
CGO_ENABLED=1 make build
```

**Verify:** Both commands exit with code 0.

---

## TASK 34 — Manual browser check

**Action:** Boot Plomvix locally and verify the UI manually:

```bash
CGO_ENABLED=1 make build
./plomvix
```

In the browser:
1. Open `http://localhost:8080/login`
2. Login as admin
3. Navigate to `http://localhost:8080/app/explore`
4. Confirm Log Explorer page renders with Tailwind/shadcn styling
5. Confirm time presets update the query
6. Confirm filter search runs
7. Confirm tail mode starts and stops without errors
8. Confirm row View JSON action opens the JSON dialog
9. Confirm the page still follows the active Sprint 12 theme variables

**Verify:** Browser console has no runtime errors related to Log Explorer.

---

## TASK 35 — Full Sprint 15 smoke test

**Action:** Run the following verification sequence from project root:

```bash
#!/bin/bash
set -euo pipefail

TMP_DIR="$(mktemp -d)"
SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

export PLOMVIX_ENV="development"
export PLOMVIX_STORAGE_DATA_DIR="$TMP_DIR/data"
export PLOMVIX_THEME_PATH="$TMP_DIR/theme.json"
export PLOMVIX_AUTH_DEFAULT_ADMIN_USERNAME="admin"
export PLOMVIX_AUTH_DEFAULT_ADMIN_PASSWORD="testpass"
export PLOMVIX_AUTH_JWT_SECRET="sprint15-smoke-secret"

echo "=== Step 1: Build ==="
CGO_ENABLED=1 make build

echo "=== Step 2: Run tests ==="
CGO_ENABLED=1 make test

echo "=== Step 3: Boot server ==="
./plomvix > "$TMP_DIR/plomvix_s15.log" 2>&1 &
SERVER_PID=$!
sleep 3

echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"testpass"}' | jq -r '.data.token')
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { echo "FAIL: login token missing"; exit 1; }

echo "=== Step 5: Ingest sample logs ==="
curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"records":[{"level":"info","message":"sprint15 info","service":"ui"},{"level":"error","message":"sprint15 error","service":"ui"}]}' \
    | jq '.data.ingested' | grep -q '^2$'

echo "=== Step 6: Query logs ==="
curl -sf "http://localhost:8080/query/logs?from=0&limit=100&offset=0" \
    -H "Authorization: Bearer $TOKEN" \
    | jq '.data.total >= 2' | grep -q true

echo "=== Step 7: Query filter ==="
curl -sf "http://localhost:8080/query/logs?from=0&filter=level%3Derror&limit=100&offset=0" \
    -H "Authorization: Bearer $TOKEN" \
    | jq '.data.records | length >= 1' | grep -q true

echo "=== Step 8: UI route loads ==="
curl -sf http://localhost:8080/app/explore | grep -qi "plomvix" \
    && echo "PASS: /app/explore served" \
    || { echo "FAIL: /app/explore did not serve UI"; exit 1; }

echo "Sprint 15 smoke test PASSED"
```

**Verify:** Script completes with `Sprint 15 smoke test PASSED`.

---

## TASK 36 — Final repository check

**Action:** Run:

```bash
git status --short
```

**Verify:** `git status --short` shows only intentional Sprint 15 files changed.

---

## FINAL SPRINT 15 ACCEPTANCE CHECKLIST

- `/app/explore` renders the Log Explorer UI after login
- React route registry path is `/explore`, not `/app/explore`
- No new backend query endpoints were added
- Query client calls `GET /query/logs`
- Query params use `from`, `to`, `filter`, `limit`, and `offset`
- Outgoing nanosecond params are built as decimal strings with `BigInt`
- Time presets exist for 15m, 1h, 6h, 24h, and 7d
- Custom from/to works and invalid ranges do not query
- Search bar maps to Sprint 6 filter syntax
- Filter chips parse and remove `AND`-combined conditions
- Duplicate filter chips are removable one at a time
- Results table uses shadcn `Table` and shows timestamp, level, message, and dynamic fields
- Rows can open a JSON detail dialog
- Tail mode polls every 5 seconds, updates visible range state, and cleans up timers
- Pagination previous/next works and shows total count
- Empty state exists using shadcn/Tailwind
- Loading skeleton exists using shadcn `Skeleton`
- Error state with retry exists using shadcn `Alert`
- Toasts are emitted through the existing app event system
- Tailwind/shadcn theme classes are used for styling
- No new state-management/table/date dependencies were added
- No second toast system was added
- `docs/api/log-explorer.md` exists
- README mentions Log Explorer
- UI typecheck passes
- UI build passes
- Go tests pass
- `CGO_ENABLED=1 make lint` passes
- `CGO_ENABLED=1 make build` passes
