# Plomvix — Sprint 14 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | TypeScript | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–13 are complete. Sprint 11 added the UI foundation. Sprint 12 added the
Theme Engine and Developer Design Panel. Sprint 13 added cookie auth, login/logout UI,
protected app routes, an auth provider, and a centralized frontend API client.

Sprint 14 adds the **Admin UI**. This sprint does not add new backend business
capabilities. It builds a browser interface on top of the admin/auth/system APIs
that already exist from Sprints 2, 9, 12, and 13.

**What Sprint 14 delivers:**
- `/app/admin` route in the authenticated React app
- Admin UI layout using the existing app shell and route registry
- Admin route hidden from non-admin users in the sidebar
- Admin overview page with three tabs/sections:
  - Users
  - API Keys
  - System Stats
- User Management UI:
  - Users table with username, role, created at, updated at, and actions
  - Create user modal
  - Edit user modal
  - Delete user confirmation modal
  - Calls existing Sprint 2 admin user APIs
- API Key Management UI:
  - Per-user key status
  - Generate key
  - Revoke key
  - Show/hide generated key
  - Copy generated key to clipboard
  - Mask generated key by default after initial display
  - Static agent setup examples for Telegraf, Vector, Fluent Bit, and curl
  - Clear warning that API keys currently grant full API access
- System Stats UI:
  - Stat cards for WAL, hot tier, cold tier, runtime, build, and uptime
  - Data from `GET /admin/stats` and `GET /admin/info`
  - Auto-refresh every 30 seconds
  - Manual refresh button
  - No charts yet — clean cards only
- Loading states, empty states, error states, and retry buttons
- Toasts for create/update/delete/generate/revoke/copy/refresh outcomes
- Frontend tests for API client helpers and pure UI utility functions where the test runner exists
- `docs/api/admin-ui.md` documenting the Admin UI
- README updated with Admin UI notes

**What Sprint 14 does NOT do:**
- No new backend admin endpoints unless a tiny compatibility wrapper is required by existing code
- No RBAC expansion — current v1 users are still admin only
- No role editor beyond preserving the current `admin` role
- No organization/team model
- No audit log UI
- No charts or historical trend graphs
- No log explorer — Sprint 15
- No trace storage or trace UI — Sprint 16
- No OTLP or Prometheus remote write — Sprint 17
- No API key scopes — API keys remain full-access in Sprint 14
- No storage of plaintext API keys after the generate response leaves the UI state

---

## EXISTING API CONTRACTS — READ BEFORE WRITING ANY CODE

Sprint 14 must consume the APIs that already exist. Do not invent new route names.
Use the centralized frontend API client from Sprint 13 so cookie auth, refresh retry,
Plomvix response-envelope parsing, and toast/session-expired handling stay consistent.

**User APIs from Sprint 2:**

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/admin/users` | List users |
| `POST` | `/admin/users` | Create user |
| `GET` | `/admin/users/{id}` | Get one user |
| `PATCH` | `/admin/users/{id}` | Update user |
| `DELETE` | `/admin/users/{id}` | Delete user |
| `POST` | `/admin/users/{id}/apikey` | Generate or rotate an API key |
| `DELETE` | `/admin/users/{id}/apikey` | Revoke API key |
| `GET` | `/admin/users/{id}/apikey/status` | Check whether user has an API key |

**System APIs from Sprint 9:**

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/admin/stats` | Consolidated system stats |
| `GET` | `/admin/info` | Version, build time, git commit, Go version, uptime |
| `GET` | `/admin/wal/stats` | WAL stats |
| `GET` | `/admin/cold/stats` | Cold tier stats |

**Auth/API client expectations from Sprint 13:**
- Browser auth uses the httpOnly `plomvix_token` cookie.
- Frontend requests must use `credentials: "include"` or the Sprint 13 API client.
- Do not store JWTs in localStorage or sessionStorage.
- API client unwraps Plomvix envelopes: `{status:"ok", data:{...}, request_id:"..."}`.
- API client emits `session-expired`; `AuthProvider` handles redirect/toast.

**Role rule for Sprint 14:**
All current users are admin in the v1 flow, but the UI must still check
`auth.user.role === "admin"` before rendering the Admin page and sidebar link.
If a future non-admin role appears, the route should show a forbidden state instead
of rendering user/API key controls.

---

## ROUTING DESIGN — READ BEFORE WRITING ANY CODE

Sprint 13 uses `BrowserRouter basename="/app"` for authenticated app routes.
Therefore:

| Browser URL | React route registry path |
|---|---|
| `/app/admin` | `/admin` |

Do **not** put `/app/admin` in the route registry path. Use `/admin` there.
The `/app` prefix is provided by the router basename.

The Admin UI is route-registry driven, just like earlier placeholder pages. The
sidebar must not hardcode `/admin`; it should read route metadata.

---

## FRONTEND DESIGN — READ BEFORE WRITING ANY CODE

The Admin UI must use existing primitives and infrastructure from earlier UI sprints:
- app shell/sidebar/header from Sprint 11
- app event/toast system from Sprint 11
- theme CSS variables from Sprint 12
- `ThemeModeToggle` from Sprint 12
- auth provider and protected routes from Sprint 13
- centralized API client from Sprint 13

**State-management rule:**
Use local component state plus small custom hooks. Do not add Redux, Zustand, React
Query, TanStack Query, or another new state-management dependency in Sprint 14.

**Modal rule:**
If Sprint 11/12 already created a modal primitive, reuse it. If no modal primitive
exists, create a minimal local modal component under `ui/src/components/Modal.tsx`.
Do not add a third-party modal dependency.

**Date formatting rule:**
Use the browser `Intl.DateTimeFormat` APIs. Do not add a date library.

**Copy-to-clipboard rule:**
Use `navigator.clipboard.writeText` when available. If unavailable, show an error
toast. Do not add a clipboard dependency.

**Plaintext API key safety rule:**
The plaintext key returned by `POST /admin/users/{id}/apikey` may be stored only in
React component state for the current page/modal display. It must never be persisted
to localStorage, sessionStorage, IndexedDB, or backend storage by the UI.

---

## TASK 01 — Confirm Sprint 13 UI auth/client files exist

**Action:** Inspect the UI files from Sprint 13 and identify the exact names/exports
for:
- `useAuth()`
- auth user type
- centralized API request helper
- app event/toast hook
- existing Button primitive
- existing route registry
- existing sidebar component

If names differ from this plan, adapt imports to the existing codebase names while
preserving Sprint 14 behaviour.

**Verify:** Write down the exact file paths in a temporary note or terminal output;
no code changes required.

---

## TASK 02 — Create ui/src/admin directory

**Action:** Create the Admin UI directory:
```bash
mkdir -p ui/src/admin
mkdir -p ui/src/admin/components
mkdir -p ui/src/admin/hooks
```

**Verify:** `ls ui/src/admin ui/src/admin/components ui/src/admin/hooks` shows the directories.

---

## TASK 03 — Create ui/src/admin/types.ts

**Action:** Create `ui/src/admin/types.ts`.

**Requirements:**
Define TypeScript types that match the existing backend/OpenAPI JSON shapes.
Use snake_case property names where the backend sends snake_case.

**Exports required:**
```ts
export type UserRole = 'admin'

export interface AdminUser {
  id: string
  username: string
  role: UserRole
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  username: string
  password: string
}

export interface UpdateUserRequest {
  username?: string
  password?: string
}

export interface APIKeyStatus {
  user_id?: string
  has_api_key: boolean
}

export interface GeneratedAPIKey {
  api_key: string
}

export interface AdminStats {
  [key: string]: unknown
}

export interface AdminInfo {
  version?: string
  build_time?: string
  git_commit?: string
  go_version?: string
  uptime?: string
  uptime_seconds?: number
  [key: string]: unknown
}
```

**Note:** Keep `AdminStats` flexible because Sprint 9 may expose nested WAL/hot/cold/runtime
stats with implementation-specific field names.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 04 — Create ui/src/admin/adminApi.ts

**Action:** Create `ui/src/admin/adminApi.ts`.

**Requirements:**
Use the Sprint 13 centralized API helper. Do not use raw `fetch` directly except if
the existing API helper has no method support; in that case, add the missing method
to the central helper first instead of bypassing it here.

**Exports required:**
```ts
export async function listUsers(): Promise<AdminUser[]>
export async function createUser(input: CreateUserRequest): Promise<AdminUser>
export async function getUser(id: string): Promise<AdminUser>
export async function updateUser(id: string, input: UpdateUserRequest): Promise<AdminUser>
export async function deleteUser(id: string): Promise<void>
export async function getAPIKeyStatus(userId: string): Promise<APIKeyStatus>
export async function generateAPIKey(userId: string): Promise<GeneratedAPIKey>
export async function revokeAPIKey(userId: string): Promise<void>
export async function getAdminStats(): Promise<AdminStats>
export async function getAdminInfo(): Promise<AdminInfo>
```

**Response-shape rule:**
If the backend returns `{user: {...}}`, `{users: [...]}`, `{api_key: "..."}`, or a
plain object inside the envelope `data`, normalize it inside `adminApi.ts` so UI
components receive the stable types above.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 05 — Add missing central API methods only if required

**Action:** If the Sprint 13 API client does not support all required HTTP methods
or `void` responses, update that central API client.

**Requirements:**
- Must support `GET`, `POST`, `PATCH`, and `DELETE`
- Must support JSON request bodies
- Must support successful empty responses
- Must keep `credentials: "include"`
- Must keep best-effort refresh-on-401 behaviour from Sprint 13
- Must not store JWTs in browser storage

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 06 — Create ui/src/admin/format.ts

**Action:** Create `ui/src/admin/format.ts`.

**Exports required:**
```ts
export function formatDateTime(value: string | undefined | null): string
export function formatDuration(value: unknown): string
export function formatNumber(value: unknown): string
export function titleCase(value: string): string
```

**Behaviour:**
- `formatDateTime` uses `Intl.DateTimeFormat`
- Invalid or empty values return `—`
- No external date/number dependency

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 07 — Create ui/src/admin/statsFlatten.ts

**Action:** Create `ui/src/admin/statsFlatten.ts`.

**Purpose:** Convert flexible nested admin stats into card-friendly rows without
hardcoding exact Sprint 9 internal struct names.

**Exports required:**
```ts
export interface StatCardItem {
  key: string
  label: string
  value: string
  group: string
}

export function flattenStats(input: Record<string, unknown>, group?: string): StatCardItem[]
```

**Behaviour:**
- Recursively flatten nested plain objects
- Skip arrays by rendering their length as a value
- Render booleans/numbers/strings as values
- Convert snake_case/camelCase keys to readable labels
- Limit recursion depth to 3 to avoid runaway rendering

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 08 — Create ui/src/components/Modal.tsx if missing

**Action:** If no modal primitive exists, create `ui/src/components/Modal.tsx`.
If a modal already exists, skip this task and reuse the existing component in later tasks.

**Minimal Modal requirements:**
```tsx
export interface ModalProps {
  open: boolean
  title: string
  children: React.ReactNode
  footer?: React.ReactNode
  onClose: () => void
}

export function Modal(props: ModalProps): React.ReactElement | null
```

**Behaviour:**
- Return `null` when `open === false`
- Close on backdrop click
- Close on Escape key
- Do not close when clicking inside the dialog content
- Use theme CSS variables
- No third-party dependency

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 09 — Create ui/src/admin/components/AdminSection.tsx

**Action:** Create `ui/src/admin/components/AdminSection.tsx`.

**Purpose:** Shared section wrapper for Admin UI pages.

**Props:**
```ts
interface AdminSectionProps {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 10 — Create ui/src/admin/components/EmptyState.tsx

**Action:** Create `ui/src/admin/components/EmptyState.tsx`.

**Props:**
```ts
interface EmptyStateProps {
  title: string
  description?: string
  action?: React.ReactNode
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 11 — Create ui/src/admin/components/ErrorState.tsx

**Action:** Create `ui/src/admin/components/ErrorState.tsx`.

**Props:**
```ts
interface ErrorStateProps {
  title?: string
  message: string
  onRetry?: () => void
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 12 — Create ui/src/admin/components/LoadingState.tsx

**Action:** Create `ui/src/admin/components/LoadingState.tsx`.

**Props:**
```ts
interface LoadingStateProps {
  label?: string
}
```

**Behaviour:** Simple skeleton/spinner-free loading block using CSS variables.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 13 — Create ui/src/admin/hooks/useUsers.ts

**Action:** Create `ui/src/admin/hooks/useUsers.ts`.

**Exports required:**
```ts
export function useUsers(): {
  users: AdminUser[]
  loading: boolean
  error: string | null
  reload: () => Promise<void>
  create: (input: CreateUserRequest) => Promise<void>
  update: (id: string, input: UpdateUserRequest) => Promise<void>
  remove: (id: string) => Promise<void>
}
```

**Behaviour:**
- Load users on first mount
- Sort users by `created_at` ascending for stable table output
- Emit success/error toasts using Sprint 11 event system
- Reload users after create/update/delete
- Never include password fields in local state after request completes

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 14 — Create ui/src/admin/hooks/useAPIKeys.ts

**Action:** Create `ui/src/admin/hooks/useAPIKeys.ts`.

**Exports required:**
```ts
export function useAPIKeys(users: AdminUser[]): {
  statusByUserId: Record<string, APIKeyStatus>
  generatedKeyByUserId: Record<string, string>
  loadingByUserId: Record<string, boolean>
  error: string | null
  loadStatus: (userId: string) => Promise<void>
  loadAllStatuses: () => Promise<void>
  generate: (userId: string) => Promise<void>
  revoke: (userId: string) => Promise<void>
  clearGeneratedKey: (userId: string) => void
}
```

**Behaviour:**
- Fetch key status per user
- Store plaintext generated keys only in `generatedKeyByUserId` React state
- Clear generated key state when revoked
- Emit toasts on generate/revoke failures and successes
- Do not auto-generate keys

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 15 — Create ui/src/admin/hooks/useAdminStats.ts

**Action:** Create `ui/src/admin/hooks/useAdminStats.ts`.

**Exports required:**
```ts
export function useAdminStats(autoRefreshMs?: number): {
  stats: AdminStats | null
  info: AdminInfo | null
  loading: boolean
  refreshing: boolean
  error: string | null
  lastLoadedAt: Date | null
  reload: () => Promise<void>
}
```

**Behaviour:**
- Load `GET /admin/stats` and `GET /admin/info`
- Default `autoRefreshMs` is `30000`
- Use `window.setInterval` and cleanup on unmount
- Manual `reload()` sets `refreshing` without blanking existing data
- Stop auto-refresh when the component unmounts

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 16 — Create ui/src/admin/components/UserFormModal.tsx

**Action:** Create `ui/src/admin/components/UserFormModal.tsx`.

**Purpose:** Reusable create/edit user modal.

**Props:**
```ts
interface UserFormModalProps {
  open: boolean
  mode: 'create' | 'edit'
  user?: AdminUser
  submitting: boolean
  onSubmit: (input: CreateUserRequest | UpdateUserRequest) => Promise<void>
  onClose: () => void
}
```

**Behaviour:**
- Create mode requires username and password
- Edit mode allows username update and optional password update
- Role is displayed as read-only `admin`
- Inline validation errors before submit
- Clear password field after submit attempt finishes
- Does not store password outside component state

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 17 — Create ui/src/admin/components/DeleteUserModal.tsx

**Action:** Create `ui/src/admin/components/DeleteUserModal.tsx`.

**Props:**
```ts
interface DeleteUserModalProps {
  open: boolean
  user: AdminUser | null
  submitting: boolean
  onConfirm: () => Promise<void>
  onClose: () => void
}
```

**Behaviour:**
- Requires typing the username to confirm deletion
- Shows warning that deletion is permanent
- Disable confirm until typed username matches exactly

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 18 — Create ui/src/admin/components/UsersTable.tsx

**Action:** Create `ui/src/admin/components/UsersTable.tsx`.

**Props:**
```ts
interface UsersTableProps {
  users: AdminUser[]
  currentUserId?: string
  onEdit: (user: AdminUser) => void
  onDelete: (user: AdminUser) => void
}
```

**Columns:**
- Username
- Role
- Created At
- Updated At
- Actions

**Behaviour:**
- Show current user badge when `user.id === currentUserId`
- Disable delete action for current user to reduce accidental lockout
- Empty state when no users exist
- Use theme CSS variables

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 19 — Create ui/src/admin/components/UserManagementPanel.tsx

**Action:** Create `ui/src/admin/components/UserManagementPanel.tsx`.

**Behaviour:**
- Uses `useUsers()`
- Uses `useAuth()` to get current user id when available
- Renders `UsersTable`
- Renders create/edit/delete modals
- Add `Create User` button
- Loading, error, empty, and retry states included

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 20 — Create ui/src/admin/components/APIKeyReveal.tsx

**Action:** Create `ui/src/admin/components/APIKeyReveal.tsx`.

**Props:**
```ts
interface APIKeyRevealProps {
  apiKey: string
  onClear: () => void
}
```

**Behaviour:**
- Mask key by default
- Toggle show/hide
- Copy to clipboard button
- Emits success/error toast on copy
- Warning text: `This is the only time the plaintext API key will be shown.`
- `Clear from screen` button calls `onClear`

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 21 — Create ui/src/admin/components/AgentConfigExamples.tsx

**Action:** Create `ui/src/admin/components/AgentConfigExamples.tsx`.

**Content required:**
Static examples for:
- curl
- Telegraf
- Vector
- Fluent Bit

**Rules:**
- Use placeholder `YOUR_PLOMVIX_API_KEY`
- Do not interpolate a real generated key into static config examples
- Include warning: `Sprint 14 API keys grant full API access. Scopes are not available yet.`

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 22 — Create ui/src/admin/components/APIKeyManagementPanel.tsx

**Action:** Create `ui/src/admin/components/APIKeyManagementPanel.tsx`.

**Behaviour:**
- Uses `useUsers()` or receives users from parent — choose the simplest implementation that avoids duplicate network calls in the final Admin page
- Uses `useAPIKeys(users)`
- Shows one row/card per user
- Shows whether user currently has an API key
- Generate button calls `POST /admin/users/{id}/apikey`
- Revoke button calls `DELETE /admin/users/{id}/apikey`
- Confirm before revoke
- Generated plaintext key appears in `APIKeyReveal`
- Includes `AgentConfigExamples`
- Loading/error/retry states included

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 23 — Create ui/src/admin/components/StatCard.tsx

**Action:** Create `ui/src/admin/components/StatCard.tsx`.

**Props:**
```ts
interface StatCardProps {
  label: string
  value: string
  group?: string
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 24 — Create ui/src/admin/components/SystemStatsPanel.tsx

**Action:** Create `ui/src/admin/components/SystemStatsPanel.tsx`.

**Behaviour:**
- Uses `useAdminStats(30000)`
- Shows manual refresh button
- Shows last refreshed timestamp
- Renders build/runtime info from `GET /admin/info`
- Renders stat cards from flattened `GET /admin/stats`
- Shows loading state on first load
- Shows non-destructive refreshing state on later reloads
- Shows error with retry button
- No charts or graph dependencies

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 25 — Create ui/src/admin/AdminPage.tsx

**Action:** Create `ui/src/admin/AdminPage.tsx`.

**Behaviour:**
- Uses `useAuth()`
- If auth is loading, show loading state
- If no user, let Sprint 13 protected route redirect; do not duplicate redirect logic here
- If `user.role !== "admin"`, show forbidden state with message `Admin role required`
- Renders Admin title and description
- Renders local tabs or segmented controls:
  - Users
  - API Keys
  - System Stats
- Default tab is Users
- Preserve active tab in URL query param `?tab=users|api-keys|stats` if simple with existing router; otherwise local state is acceptable

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 26 — Add Admin route to route registry

**Action:** Update the Sprint 11 route registry, likely `ui/src/app/routes.tsx`.

**Requirements:**
- Import `AdminPage`
- Add route metadata for Admin
- External browser URL must be `/app/admin`
- Route registry path must be `/admin` because `BrowserRouter` uses `basename="/app"`
- Mark route as requiring auth/admin using whatever metadata exists from Sprint 13

**Example shape — adapt to existing route type:**
```tsx
{
  path: '/admin',
  label: 'Admin',
  element: <AdminPage />,
  nav: true,
  authRequired: true,
  adminOnly: true,
  group: 'Admin',
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 27 — Update route type for adminOnly if needed

**Action:** If the existing `AppRoute` type does not already support admin-only route
metadata, add:
```ts
adminOnly?: boolean
```

Do not remove existing fields from Sprints 11–13 such as `devOnly`, `group`, or
auth-related metadata.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 28 — Update sidebar filtering for admin routes

**Action:** Update the sidebar component.

**Requirements:**
- Read current auth user from `useAuth()`
- Hide routes with `adminOnly === true` unless `user?.role === "admin"`
- Preserve Sprint 12 `devOnly` filtering for `/dev/design`
- Preserve route ordering and grouping
- Do not hardcode `/admin` in the sidebar

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 29 — Update protected route handling for adminOnly if needed

**Action:** If Sprint 13 protected-route wrapper already understands admin-only route
metadata, skip this task. Otherwise update it.

**Requirements:**
- Authenticated non-admin users navigating directly to `/app/admin` see a forbidden page
- Unauthenticated users still redirect to `/login`
- Do not redirect authenticated non-admin users to `/login`
- Preserve existing Sprint 13 protected route behaviour

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 30 — Add Admin UI styles

**Action:** Add Admin UI CSS using the existing styling approach from Sprint 11/12.

**Requirements:**
- Use CSS variables from Sprint 12
- Do not hardcode colors except as CSS variable fallbacks
- Provide responsive layouts for narrow screens
- Tables should become horizontally scrollable on small screens
- Modals should fit small screens

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 31 — Update docs/api/admin-ui.md

**Action:** Create `docs/api/admin-ui.md`.

**Document:**
- What the Admin UI is
- Browser route: `/app/admin`
- Requires authenticated admin user
- User Management capabilities
- API Key Management capabilities
- Full-access API key warning
- Plaintext API key shown once
- System Stats cards and 30-second auto-refresh
- Agent config examples are static placeholders
- Current limitations: no RBAC expansion, no API key scopes, no audit logs, no charts

**Verify:** `cat docs/api/admin-ui.md` shows all sections above.

---

## TASK 32 — Update README.md with Admin UI section

**Action:** Update `README.md`.

**Add:**
- Admin UI overview
- Route `/app/admin`
- Login requirement
- User/API key/system stats summary
- Link/reference to `docs/api/admin-ui.md`

**Verify:**
```bash
grep -n "Admin UI" README.md
grep -n "/app/admin" README.md
grep -n "docs/api/admin-ui.md" README.md
```

---

## TASK 33 — Update OpenAPI descriptions only if needed

**Action:** Review `api/openapi.json` from Sprint 9/12/13.

**Requirement:**
Do not add fake Admin UI API endpoints. The Admin UI is a frontend route, not a JSON API.
Only update descriptions of existing admin/user/API-key endpoints if they are inaccurate.

**Verify:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json
```

---

## TASK 34 — Add UI tests for admin format utilities if test runner exists

**Action:** If the UI test runner from Sprint 13 exists, add tests for:
- `formatDateTime()` returns `—` for empty/invalid values
- `titleCase()` handles snake_case and camelCase labels
- `flattenStats()` flattens nested stats
- `flattenStats()` handles arrays as counts
- `flattenStats()` stops at depth limit

Use explicit Vitest imports:
```ts
import { describe, it, expect } from 'vitest'
```

If no UI test runner exists, document deferral in `docs/api/admin-ui.md` and do not
add a new dependency in Sprint 14.

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

## TASK 35 — Add UI tests for admin API normalization if test runner exists

**Action:** If the UI test runner exists, add tests for pure normalization helpers
inside `adminApi.ts` or move normalization into exported helper functions and test them.

**Cases:**
- list users normalizes `{users:[...]}`
- create user normalizes `{user:{...}}` or plain user object
- generated API key normalizes `{api_key:"..."}`
- API key status normalizes missing `user_id`

Use explicit Vitest imports.

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

## TASK 36 — Add UI smoke route check

**Action:** Add or update an existing UI smoke test/documented smoke command to check
that the built app includes the Admin route.

**Minimum manual check command after build:**
```bash
cd ui
npm run build
grep -R "Admin" dist/ | head
```

If Vite output is minified and grep is unreliable, rely on typecheck/build and the
full Sprint 14 smoke test instead.

**Verify:** `cd ui && npm run build` exits with code 0.

---

## TASK 37 — Backend compatibility check for admin endpoints

**Action:** Run backend tests covering existing admin endpoints.

**Verify:**
```bash
CGO_ENABLED=1 go test ./internal/auth/...
CGO_ENABLED=1 go test ./internal/admin/...
CGO_ENABLED=1 go test ./internal/server/...
```

If an `internal/admin` package does not exist as a separate testable package in the
current codebase, skip only that command and run `CGO_ENABLED=1 go test ./...` in
TASK 40.

---

## TASK 38 — Add integration smoke test for Admin UI if integration scaffold exists

**Action:** If Sprint 10 integration tests exist, add `tests/integration/admin_ui_test.go`.

**Test flow:**
1. Start test server
2. `GET /app/admin` without browser login returns the SPA shell, not JSON 404
3. Login as admin through existing helper
4. `GET /admin/users` with admin auth returns 200
5. `POST /admin/users` creates a test user
6. `GET /admin/users` includes the test user
7. `POST /admin/users/{id}/apikey` returns plaintext API key once
8. `GET /admin/users/{id}/apikey/status` returns `has_api_key: true`
9. `DELETE /admin/users/{id}/apikey` revokes the key
10. `GET /admin/stats` returns 200
11. `GET /admin/info` returns 200

**Note:** Integration tests validate backend APIs and SPA serving. They do not need
to run a browser automation stack in Sprint 14.

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/...
```

If integration test scaffolding is absent, document this deferral in `docs/api/admin-ui.md`.

---

## TASK 39 — Run UI verification

**Action:**
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

## TASK 40 — Run Go formatting and backend tests

**Action:**
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 go test ./...
```

**Verify:** All commands exit with code 0.

---

## TASK 41 — Full build and smoke test

**Action:** Run this verification sequence from project root:

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

export PLOMVIX_STORAGE_DATA_DIR="$TMP_DIR/data"
export PLOMVIX_THEME_PATH="$TMP_DIR/theme.json"
export PLOMVIX_AUTH_DEFAULT_ADMIN_PASSWORD="adminpass123"
export PLOMVIX_AUTH_JWT_SECRET="sprint14-smoke-secret"

echo "=== Step 1: Full build ==="
CGO_ENABLED=1 make build

echo "=== Step 2: Tests ==="
CGO_ENABLED=1 make test

echo "=== Step 3: Boot server ==="
./plomvix > "$TMP_DIR/plomvix_s14.log" 2>&1 &
SERVER_PID=$!
sleep 3

echo "=== Step 4: Login ==="
LOGIN_JSON=$(curl -sf -c "$TMP_DIR/cookies.txt" \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"adminpass123"}' \
    http://localhost:8080/auth/login)
echo "$LOGIN_JSON" | jq -e '.data.token' > /dev/null

echo "=== Step 5: Admin users API works with cookie ==="
curl -sf -b "$TMP_DIR/cookies.txt" http://localhost:8080/admin/users | jq -e '.status == "ok"' > /dev/null

echo "=== Step 6: Admin stats API works with cookie ==="
curl -sf -b "$TMP_DIR/cookies.txt" http://localhost:8080/admin/stats | jq -e '.status == "ok"' > /dev/null

echo "=== Step 7: Admin info API works with cookie ==="
curl -sf -b "$TMP_DIR/cookies.txt" http://localhost:8080/admin/info | jq -e '.status == "ok"' > /dev/null

echo "=== Step 8: Admin SPA route serves UI ==="
curl -sf http://localhost:8080/app/admin | grep -qi "plomvix" \
    && echo "PASS: /app/admin serves SPA" \
    || { echo "FAIL: /app/admin did not serve SPA"; exit 1; }

echo "Sprint 14 smoke test PASSED"
```

**Verify:** Script completes with `Sprint 14 smoke test PASSED`.

---

## TASK 42 — Final lint and repository check

**Action:**
```bash
CGO_ENABLED=1 make lint
CGO_ENABLED=1 make build
git status --short
```

**Verify:**
- `make lint` exits with code 0
- `make build` exits with code 0
- `git status --short` shows only intentional Sprint 14 files changed

---

## FINAL SPRINT 14 ACCEPTANCE CHECKLIST

- `/app/admin` route exists and serves the React app
- React route registry uses `/admin`, not `/app/admin`
- Admin sidebar link is route-registry driven
- Admin sidebar link is hidden unless `user.role === "admin"`
- Direct navigation to `/app/admin` requires authentication
- Non-admin authenticated users see a forbidden state
- User Management table exists with username, role, created at, updated at, and actions
- Create user modal works and clears password state after submit
- Edit user modal works and treats password as optional
- Delete user confirmation requires typing username
- Current user delete action is disabled in the UI
- API Key Management shows status per user
- Generate API key shows plaintext key once in UI state only
- Generated key can be shown/hidden and copied
- Generated key can be cleared from screen
- Revoke API key requires confirmation
- Static agent examples exist for curl, Telegraf, Vector, and Fluent Bit
- Full-access API key warning is visible
- System Stats panel loads `/admin/stats` and `/admin/info`
- System Stats auto-refreshes every 30 seconds
- System Stats has manual refresh
- Loading, error, empty, and retry states exist
- Toasts are emitted for admin actions
- No new backend endpoint is invented for Admin UI
- No new state-management or chart dependency is added
- `docs/api/admin-ui.md` exists
- `README.md` mentions Admin UI
- OpenAPI remains valid JSON with no placeholders
- `find ... -exec gofmt -w {} +` has been used
- `cd ui && npm run typecheck` passes
- `cd ui && npm run build` passes
- `CGO_ENABLED=1 go test ./...` passes
- `CGO_ENABLED=1 make build` passes
- `CGO_ENABLED=1 make lint` passes
