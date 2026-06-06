# UI Migration Plan: Replace `ui/` with `obs_theme/`

## Goal

Swap the placeholder `ui/` React app (3 stub pages, no real views) with the fully-built
`obs_theme/` React app (15 pages, MUI v9, ECharts, real layout) as the Plomvix frontend.

Wire real auth and real admin data to the backend. Leave all observability pages
(Metrics, Traces, APM, Alerts, Incidents, Synthetics) on mock data. The Logs page
exists as a stub (upgrade to full spec later).

---

## Current state summary (verified 2026-06-06)

**Already done:**
- ✅ `vite.config.ts` — `base: '/app/'`, `outDir: 'dist'`, `server.port: 3000`
- ✅ `App.tsx` — `basename="/app"`
- ✅ `logs/` page — stub exists (builds successfully)
- ✅ `npm run build` — succeeds (1.7 MB bundle)

**Still needs doing:**
- ❌ `server.go` — still serves `ui/dist`, no redirects for `/login` etc.
- ❌ `Makefile` — no `obs-build`, `obs-dev` targets
- ❌ `authStore` — still fake demo auth (email/password, hardcoded creds)
- ❌ `LoginForm` — still uses email field, hardcoded demo credentials
- ❌ `api/` directory — does not exist (client.ts, adminApi.ts needed)
- ❌ `users/` page — still mock data, not wired to `/admin/users`

**Start with:** Phase 1.3 (server.go redirects) → Phase 1.4 (Makefile) → Phase 2 (auth)

---

## Verified facts (read directly from source files)

| File | Verified value | Status |
|------|---------------|--------|
| `obs_theme/vite.config.ts` | `base: '/app/'`, `outDir: 'dist'`, `server.port: 3000` | ✅ ALREADY DONE |
| `obs_theme/src/App.tsx` | `<BrowserRouter basename="/app">` | ✅ ALREADY DONE |
| `obs_theme/src/pages/logs/` | EXISTS (stub implementation, not full phase3.md spec) | ⚠️ PARTIAL |
| `obs_theme` import style | relative paths only — **no `@/` alias anywhere** | ✅ Confirmed |
| `obs_theme/src/store/authStore.ts` | `login(email, password)` — **takes email, not username** | ❌ NEEDS FIX |
| `obs_theme/src/store/authStore.ts` | `logout: () => void` — **synchronous** | ✅ Correct |
| `obs_theme/src/store/authStore.ts` | `User = { id, name, email, role, avatar }` | ❌ NEEDS FIX (backend mismatch) |
| `obs_theme/src/layout/Topbar.tsx` | renders `user?.name`, `user?.email`, `user?.avatar` | ✅ Confirmed |
| `obs_theme/src/layout/Topbar.tsx` | `logout(); navigate('/login'); notify.success(...)` — sync, no await | ✅ Confirmed |
| `obs_theme/src/pages/auth/components/LoginForm.tsx` | `name="email"`, `z.string().email()`, `login(data.email, data.password)` | ❌ NEEDS FIX |
| `obs_theme/src/pages/auth/components/LoginForm.tsx` | `href="/forgot-password"` on an `<a>` tag — full page reload | ❌ NEEDS FIX |
| `obs_theme/src/pages/auth/components/LoginForm.tsx` | `navigate(from \|\| '/', { replace: true })` after login | ❌ NEEDS FIX (should be '/app') |
| `obs_theme/src/components/guards/AuthGuard.tsx` | `<Navigate to="/login" ...>` | ✅ Confirmed |
| `internal/server/server.go` line 238 | `newSPAHandler("ui/dist")` | ❌ NEEDS FIX |
| `internal/server/server.go` | Go registers `/login`, `/logout`, `/dev/design` as SPA routes | ❌ NEEDS FIX (should redirect) |
| `internal/server/server.go` | dev proxy target `"http://localhost:3000"` | ✅ Confirmed |
| `internal/auth/handler.go` | login body: `{ username, password }` | ✅ Confirmed |
| `internal/auth/handler.go` | login response data: `{ token, expires_in, user }` | ✅ Confirmed |
| `internal/auth/model.go` | `UserResponse = { id, username, role, created_at, updated_at }` | ✅ Confirmed |
| `ui/src/main.tsx` | old ui uses `<BrowserRouter>` with **no basename** | ✅ Confirmed |
| `obs_theme/package.json` | `@monaco-editor/react`, `@tanstack/react-virtual`, `echarts`, `echarts-for-react` all present | ✅ Confirmed |
| `obs_theme/src/api/` | **DOES NOT EXIST** — must be created | ❌ NOT STARTED |

---

## Critical routing issue (NEW — not in previous plan versions)

The old `ui/` uses `<BrowserRouter>` with **no basename**. React Router sees the raw URL
path. So Go's `/login` route → SPA → React matches `<Route path="/login">` ✓.

The obs_theme uses `<BrowserRouter basename="/obs_theme/app">`. After we change it to
`basename="/app"`, React Router **strips the basename** before matching routes. So:

- `/app/login` → React sees `/login` → matches `<Route path="/login">` ✓
- `/login` (Go route, served as index.html) → React sees `""` relative to `/app` → **no match → NotFoundPage** ✗
- `/logout` same problem ✗
- `/forgot-password` same problem ✗

**Fix:** In `internal/server/server.go`, change the `/login`, `/logout`, `/dev/design`,
and (new) `/forgot-password` handlers from serving the SPA directly to **redirecting
permanently (308) to their `/app/...` equivalents**:

```go
// Replace these direct SPA handlers:
s.router.Handle("/login", uiHandler)          // ← WRONG with basename=/app
s.router.Handle("/logout", uiHandler)         // ← WRONG
s.router.Handle("/dev/design", uiHandler)     // ← stale, obs_theme has no /dev/design

// With permanent redirects:
s.router.Handle("/login", http.RedirectHandler("/app/login", http.StatusMovedPermanently))
s.router.Handle("/logout", http.RedirectHandler("/app/logout", http.StatusMovedPermanently))
s.router.Handle("/forgot-password", http.RedirectHandler("/app/forgot-password", http.StatusMovedPermanently))
// Remove /dev/design entirely — obs_theme has no such route
```

This applies to **both** the `DevMode` and production branches. In DevMode, replace the
proxy handle with the redirect too — the SPA at `/app/login` (served via proxy or dist)
handles it correctly from there.

Also fix `LoginForm.tsx`: the `<a href="/forgot-password">` causes a full page reload
to `/forgot-password` which redirects to `/app/forgot-password` — this works but adds
a round-trip. Change to `<Link to="/forgot-password">` (React Router) which navigates
to `/app/forgot-password` directly within the SPA.

Also fix `LoginForm.tsx`: `navigate(from || '/')` → `navigate(from || '/app', { replace: true })`.
With `basename="/app"`, `'/'` is the root of the app (= `/app/`). This is actually fine —
`'/'` relative to basename `/app` resolves to `/app/`. But `from` is a `Location` object
from `useLocation()`, not a pathname string. Fix the type:
```ts
const from = (location.state as { from?: Location } | null)?.from?.pathname ?? '/app'
```

---

## Repository layout (all files to touch)

```
plomvix/
  internal/server/server.go          line 238 + route handlers — EDIT
  internal/server/ui_test.go         update one test — EDIT
  Makefile                           add obs-build, obs-dev — EDIT
  obs_theme/
    vite.config.ts                   base, outDir, server.port — EDIT
    src/
      App.tsx                        basename — EDIT
      api/                           CREATE directory
        client.ts                    CREATE
        adminApi.ts                  CREATE
      store/authStore.ts             REPLACE entirely
      lib/auth.ts                    REPLACE with stub
      pages/
        auth/components/LoginForm.tsx  REPLACE (email→username, fix href, fix from)
        logs/                        CREATE directory + 7 files
        users/index.tsx              REPLACE (real API, fix columns)
```

---

## Backend API reference

Envelope: `{ status: "ok", data: T, request_id: string }` / `{ status: "error", error: { code, message } }`.
Cookie: `plomvix_token` (httpOnly).

| Method | Path | Auth | Body / Returns |
|--------|------|------|----------------|
| POST | `/auth/login` | none | `{username, password}` → `{token, expires_in, user: UserResponse}` + sets cookie |
| POST | `/auth/logout` | cookie | → `{message}` + clears cookie |
| GET | `/admin/users` | admin | → `UserResponse[]` |
| POST | `/admin/users` | admin | `{username, password}` → `UserResponse` |
| PATCH | `/admin/users/{id}` | admin | `{username?, password?}` → `UserResponse` |
| DELETE | `/admin/users/{id}` | admin | → empty |
| POST | `/admin/users/{id}/apikey` | admin | → `{api_key: string}` (shown once) |
| DELETE | `/admin/users/{id}/apikey` | admin | → empty |
| GET | `/admin/users/{id}/apikey/status` | admin | → `{has_key: boolean}` |
| GET | `/admin/stats` | admin | → WAL+hot+cold stats |
| GET | `/admin/info` | admin | → `{version, build_time, git_commit, go_version, os_arch, uptime_seconds}` |

`UserResponse`:
```json
{ "id": "uuid", "username": "admin", "role": "admin",
  "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z" }
```

---

## Phase 0 — Upgrade the Logs page (ALREADY EXISTS AS STUB)

The `obs_theme/src/pages/logs/` directory already exists with a basic stub implementation.
However, it does NOT match the full spec in `obs_theme/design_docs/phase3.md` (361 lines).

**Current state:** Simple list showing 5 mock logs
**Required state:** Full Logs Explorer with sidebar, toolbar, Monaco editor, histogram, virtualized table

The existing stub is sufficient to unblock the build. Upgrading to the full spec can be
done later. **Skip Phase 0 for now** — proceed to Phase 1.

---

## Phase 1 — Fix Go server routes and Makefile (vite/App.tsx ALREADY DONE)

### 1.1 ~~Edit `obs_theme/vite.config.ts`~~ — ALREADY DONE ✅

No action needed. Current state:
```ts
export default defineConfig({
  plugins: [react()],
  base: '/app/',
  build: { outDir: 'dist' },
  server: { port: 3000 },
})
```

### 1.2 ~~Edit `obs_theme/src/App.tsx`~~ — ALREADY DONE ✅

No action needed. Current state: `<BrowserRouter basename="/app">`

### 1.3 Edit `internal/server/server.go` — STILL NEEDS FIX

Three changes:

**a) Change SPA dist path (line 238):**
```go
// From:
uiHandler := newSPAHandler("ui/dist")
// To:
uiHandler := newSPAHandler("obs_theme/dist")
```

**b) Replace `/login`, `/logout`, `/dev/design` handlers with redirects** in the
production `else` branch. Replace the old block entirely:

```go
// OLD (remove this):
uiHandler := newSPAHandler("obs_theme/dist")
s.router.Handle("/app", uiHandler)
s.router.Handle("/app/*", uiHandler)
s.router.Handle("/login", uiHandler)
s.router.Handle("/logout", uiHandler)
s.router.Handle("/dev/design", uiHandler)

// NEW:
uiHandler := newSPAHandler("obs_theme/dist")
s.router.Handle("/app", uiHandler)
s.router.Handle("/app/*", uiHandler)
s.router.Handle("/login", http.RedirectHandler("/app/login", http.StatusMovedPermanently))
s.router.Handle("/logout", http.RedirectHandler("/app/logout", http.StatusMovedPermanently))
s.router.Handle("/forgot-password", http.RedirectHandler("/app/forgot-password", http.StatusMovedPermanently))
// /dev/design removed — obs_theme has no such route
```

**c) Same for the DevMode branch** — replace proxy handles for `/login`, `/logout`,
`/dev/design` with the same redirects:

```go
// In the DevMode if-branch, keep:
s.router.Handle("/app", uiProxy)
s.router.Handle("/app/*", uiProxy)
// Replace /login, /logout, /dev/design with:
s.router.Handle("/login", http.RedirectHandler("/app/login", http.StatusMovedPermanently))
s.router.Handle("/logout", http.RedirectHandler("/app/logout", http.StatusMovedPermanently))
s.router.Handle("/forgot-password", http.RedirectHandler("/app/forgot-password", http.StatusMovedPermanently))
```

### 1.4 Update Makefile

Add after the existing `ui-install` target:

```makefile
## obs-dev: Start obs_theme Vite dev server on port 3000
obs-dev:
	cd obs_theme && npm run dev

## obs-build: Build obs_theme into obs_theme/dist/
obs-build:
	cd obs_theme && npm install && npm run build
```

Change the `build` target from `build: ui-build` to:
```makefile
build: obs-build
	go build $(LDFLAGS) -o $(BINARY) ./cmd/plomvix
```

Keep all `ui-*` targets — do not remove them.

### 1.5 Build and verify

```bash
cd obs_theme && npm install && npm run build
# Must produce obs_theme/dist/index.html  (will fail if logs/ page missing — do Phase 0 first)

go run ./cmd/plomvix
# http://localhost:8080/login         → 308 redirect → /app/login → obs_theme login page ✓
# http://localhost:8080/app           → obs_theme login page (not authenticated) ✓
# http://localhost:8080/forgot-password → 308 redirect → /app/forgot-password ✓
```

---

## Phase 2 — Wire real auth

### 2.1 Create `obs_theme/src/api/client.ts`

```ts
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Envelope<T> = { status: 'ok'; data: T; request_id: string }

let _onSessionExpired: (() => void) | null = null
export function setSessionExpiredHandler(fn: () => void) {
  _onSessionExpired = fn
}

async function apiRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) init.body = JSON.stringify(body)

  const res = await fetch(path, init)

  if (res.status === 401) {
    _onSessionExpired?.()
    throw new ApiError('UNAUTHORIZED', 'Session expired')
  }

  const json = await res.json()

  if (json.status === 'error') {
    throw new ApiError(
      json.error?.code ?? 'UNKNOWN',
      json.error?.message ?? 'Unknown error',
      json.request_id,
    )
  }

  return (json as Envelope<T>).data
}

export const apiGet    = <T>(path: string)                => apiRequest<T>('GET',    path)
export const apiPost   = <T>(path: string, body: unknown) => apiRequest<T>('POST',   path, body)
export const apiPut    = <T>(path: string, body: unknown) => apiRequest<T>('PUT',    path, body)
export const apiPatch  = <T>(path: string, body: unknown) => apiRequest<T>('PATCH',  path, body)
export const apiDelete = <T>(path: string)                => apiRequest<T>('DELETE', path)
```

### 2.2 Replace `obs_theme/src/store/authStore.ts`

**Shape bridge:** Topbar renders `user?.name`, `user?.email`, `user?.avatar`.
Backend returns `{ id, username, role, created_at, updated_at }` — no name, email, avatar.
We derive them: `name = username`, `email = ''` (not stored by backend, Topbar shows
empty string under the username — acceptable), `avatar = username.slice(0,2).toUpperCase()`.

**Keep `logout()` synchronous** — Topbar calls `logout(); navigate('/login');` with no await.
Fire-and-forget the `POST /auth/logout` call.

```ts
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { apiPost } from '../api/client'

export interface User {
  id: string
  username: string
  name: string      // = username — Topbar renders user?.name
  email: string     // = '' — backend has no email field; Topbar shows it (empty is fine)
  role: 'admin' | string
  avatar: string    // = first 2 chars of username uppercased — Topbar Avatar initials
}

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>
  loginDemo: () => void   // kept as no-op so any remaining call sites don't crash
  logout: () => void      // synchronous — Topbar calls without await
}

function toUser(r: { id: string; username: string; role: string }): User {
  return {
    id: r.id,
    username: r.username,
    name: r.username,
    email: '',
    role: r.role,
    avatar: r.username.slice(0, 2).toUpperCase(),
  }
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,

      login: async (username: string, password: string) => {
        try {
          const data = await apiPost<{
            token: string
            expires_in: number
            user: { id: string; username: string; role: string }
          }>('/auth/login', { username, password })
          set({ user: toUser(data.user), token: data.token, isAuthenticated: true })
          return { success: true }
        } catch (err: unknown) {
          const msg = err instanceof Error ? err.message : 'Login failed'
          return { success: false, error: msg }
        }
      },

      loginDemo: () => {
        // No-op — kept so call sites don't crash during migration
      },

      logout: () => {
        // Fire-and-forget — clear state immediately, server blacklists the token async
        apiPost('/auth/logout', {}).catch(() => { /* ignore */ })
        set({ user: null, token: null, isAuthenticated: false })
      },
    }),
    {
      name: 'obsadmin-auth',
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        // Token expiry is enforced by the backend — no client-side check needed
        // If the stored token is stale, the first API call returns 401 and
        // setSessionExpiredHandler clears state + redirects to login
        if (state && !state.token) {
          state.isAuthenticated = false
          state.user = null
        }
      },
    },
  ),
)
```

**No changes needed to `Topbar.tsx`** — `logout()` is still sync, call pattern unchanged.

### 2.3 Replace `obs_theme/src/lib/auth.ts`

```ts
// Auth is now handled by the backend via httpOnly cookie (plomvix_token).
// These stubs prevent import errors from any remaining call sites.

export function generateDemoToken(): string {
  return ''
}

export function isTokenExpired(_token: string): boolean {
  return false
}
```

### 2.4 Replace `obs_theme/src/pages/auth/components/LoginForm.tsx`

Replace the entire file. Key changes from the current version:
- `email` field → `username` field throughout
- `z.string().email(...)` → `z.string().min(3, 'Username is required')`
- `defaultValues: { email: 'demo@obsadmin.io', password: 'ObsAdmin@demo' }` → `{ username: 'admin', password: 'changeme' }`
- `login(data.email, data.password)` → `login(data.username, data.password)`
- `type="email"` → `type="text"` on the TextField
- Label "Email" → "Username"
- `<a href="/forgot-password">` → `<Link to="/forgot-password">` (add `import { Link } from 'react-router-dom'`)
- `navigate(from || '/', { replace: true })` → `navigate(from || '/app', { replace: true })`
  And fix the `from` type: `(location.state as { from?: { pathname: string } } | null)?.from?.pathname`
- Toast: `'Welcome back, Demo User!'` → `` `Welcome back, ${data.username}!` ``
- Error text: remove the hardcoded `'. Use demo@obsadmin.io / ObsAdmin@demo...'` suffix

```tsx
import { useState } from 'react'
import { Box, Typography, TextField, Button, Alert, IconButton, InputAdornment, CircularProgress } from '@mui/material'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Eye, EyeOff } from 'lucide-react'
import { useNavigate, useLocation, Link, type Location } from 'react-router-dom'
import { useAuthStore } from '../../../store/authStore'
import { notify } from '../../../lib/toast'

const schema = z.object({
  username: z.string().min(3, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})
type FormData = z.infer<typeof schema>

export default function LoginForm() {
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const location = useLocation()

  const { control, handleSubmit, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { username: 'admin', password: 'changeme' },
  })

  const onSubmit = async (data: FormData) => {
    setLoading(true)
    setError('')
    const result = await login(data.username, data.password)
    setLoading(false)
    if (result.success) {
      notify.success(`Welcome back, ${data.username}!`)
      // AuthGuard passes state={{ from: Location }} — extract pathname
      const from = (location.state as { from?: Location } | null)?.from?.pathname ?? '/app'
      navigate(from, { replace: true })
    } else {
      setError(result.error ?? 'Invalid username or password')
    }
  }

  return (
    <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {error && <Alert severity="error" sx={{ fontSize: 13 }}>{error}</Alert>}

      <Box>
        <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Username</Typography>
        <Controller name="username" control={control} render={({ field }) => (
          <TextField {...field} size="small" fullWidth disabled={loading}
            error={!!errors.username} helperText={errors.username?.message}
            type="text" slotProps={{ input: { sx: { fontSize: 13 } } }} />
        )} />
      </Box>

      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography variant="caption2" sx={{ color: 'text.secondary' }}>Password</Typography>
          <Typography variant="caption2" component={Link} to="/forgot-password"
            sx={{ color: '#06b6d4', textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}>
            Forgot password?
          </Typography>
        </Box>
        <Controller name="password" control={control} render={({ field }) => (
          <TextField {...field} size="small" fullWidth disabled={loading}
            error={!!errors.password} helperText={errors.password?.message}
            type={showPassword ? 'text' : 'password'}
            slotProps={{ input: { sx: { fontSize: 13 },
              endAdornment: (
                <InputAdornment position="end">
                  <IconButton size="small" onClick={() => setShowPassword(!showPassword)} edge="end" tabIndex={-1}>
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </IconButton>
                </InputAdornment>
              ),
            } }} />
        )} />
      </Box>

      <Button type="submit" variant="contained" fullWidth disabled={loading}
        sx={{ height: 40, fontSize: 14, mt: 1 }}>
        {loading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null}
        {loading ? 'Signing in...' : 'Sign In'}
      </Button>
    </Box>
  )
}
```

### 2.5 Add session-expired handler in `obs_theme/src/App.tsx`

**Important:** `useNavigate()` can only be called inside a component that is a child of `<BrowserRouter>`.
Since `App()` renders `<BrowserRouter>`, we need a separate inner component to use `useNavigate`.

Add a `SessionWatcher` component as the first child inside `<BrowserRouter>`:

```tsx
// Add these imports at the top of App.tsx:
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { setSessionExpiredHandler } from './api/client'
import { useAuthStore } from './store/authStore'

// Add this inner component BEFORE the App() function:
function SessionWatcher() {
  const navigate = useNavigate()
  const logout = useAuthStore((s) => s.logout)
  useEffect(() => {
    setSessionExpiredHandler(() => {
      logout()
      navigate('/login', { replace: true })
    })
  }, [logout, navigate])
  return null
}

export default function App() {
  return (
    <BrowserRouter basename="/app">   {/* already changed in Phase 1.2 */}
      <SessionWatcher />   {/* ← add as first child inside BrowserRouter */}
      <Routes>
        ...
      </Routes>
    </BrowserRouter>
  )
}
```

---

## Phase 3 — Wire real admin data

### 3.1 Create `obs_theme/src/api/adminApi.ts`

```ts
import { apiGet, apiPost, apiPatch, apiDelete } from './client'

// Exactly matches backend UserResponse
export interface User {
  id: string
  username: string
  role: string
  created_at: string
  updated_at: string
}

export interface AdminStats {
  version: string
  env: string
  uptime_seconds: number
  pid: number
  go_version: string
  os_arch: string
  wal: { segment_count: number; active_segment: number; active_size_bytes: number; total_entries: number }
  hot: { total_writes: number; total_data_writes: number; data_dir: string }
  cold: { parquet_files: number; records_moved: number; last_flush_at: string; last_flush_duration_ms: number }
}

export interface AdminInfo {
  version: string
  build_time: string
  git_commit: string
  go_version: string
  os_arch: string
  uptime_seconds: number
}

export const listUsers       = ()                                               => apiGet<User[]>('/admin/users')
export const createUser      = (body: { username: string; password: string })  => apiPost<User>('/admin/users', body)
export const updateUser      = (id: string, body: { username?: string; password?: string }) =>
                                 apiPatch<User>(`/admin/users/${id}`, body)
export const deleteUser      = (id: string)                                     => apiDelete<{ message: string }>(`/admin/users/${id}`)
export const generateAPIKey  = (id: string)                                     => apiPost<{ api_key: string }>(`/admin/users/${id}/apikey`, {})
export const revokeAPIKey    = (id: string)                                     => apiDelete<{ message: string }>(`/admin/users/${id}/apikey`)
export const getAPIKeyStatus = (id: string)                                     => apiGet<{ has_key: boolean }>(`/admin/users/${id}/apikey/status`)
export const getAdminStats   = ()                                               => apiGet<AdminStats>('/admin/stats')
export const getAdminInfo    = ()                                               => apiGet<AdminInfo>('/admin/info')
```

### 3.2 Replace `obs_theme/src/pages/users/index.tsx`

Keep all existing MUI layout, table, modals structure. Replace only:
1. Data source: remove `initialUsers` hardcoded array, fetch from `listUsers()`
2. Action handlers: wire to real API
3. Column fix: backend has no `email` — show `username` not `name`/`email`
4. Role values: backend uses lowercase `'admin'`, not `'Admin'`/`'Editor'`/`'Viewer'`

**Data fetching pattern:**
```tsx
import { useEffect, useState } from 'react'
import { listUsers, createUser, updateUser, deleteUser,
         generateAPIKey, revokeAPIKey, type User } from '../../api/adminApi'
import { notify } from '../../lib/toast'
import PageSkeleton from '../../components/common/PageSkeleton'

// Inside component:
const [users, setUsers] = useState<User[]>([])
const [loading, setLoading] = useState(true)

const fetchUsers = async () => {
  try {
    setLoading(true)
    setUsers(await listUsers())
  } catch (err) {
    notify.error(err instanceof Error ? err.message : 'Failed to load users')
  } finally {
    setLoading(false)
  }
}
useEffect(() => { void fetchUsers() }, [])

if (loading) return <PageSkeleton />
```

**Create user:** `await createUser({ username, password })` → `void fetchUsers()`
**Delete user:** `await deleteUser(id)` → `void fetchUsers()`
**Generate API key:** `const r = await generateAPIKey(id)` → show `r.api_key` in
  a `<Dialog>` with copy-to-clipboard. The key is returned only once.
**Revoke key:** `await revokeAPIKey(id)` → `void fetchUsers()`

**Table column mapping** (backend → UI):

| Backend field | Display as |
|---|---|
| `username` | "Username" column |
| `role` | "Role" chip (`admin` → cyan, others → grey) |
| `created_at` | "Created" column, format with `dayjs` |
| no `email` | remove Email column entirely |
| no `status` | remove Status column / badge |
| no `lastSeen` | remove Last Seen column |

The "Roles" tab and "Audit Log" tab can stay as mock data — they have no backend.

---

## Phase 4 — Update Go server tests

### `internal/server/ui_test.go`

Test `TestSPAHandlerServesIndexForLogin` (line 87) sends `GET /login` and expects
a 200 with SPA content. After Phase 1.3, `/login` now returns a 308 redirect — this
test must be updated.

Options (pick one):
- **Change the test** to assert `http.StatusMovedPermanently` and `Location: /app/login`
- **Add a new test** `TestLoginRedirectsToApp` that follows the redirect

Also update or remove `TestSPAHandlerServesIndexForDevDesign` — `/dev/design` no
longer exists as a route after Phase 1.3.

---

## Execution order (strict)

**Already done:** vite.config.ts, App.tsx basename, logs/ stub

```
Phase 1.3  Edit internal/server/server.go     (dist path + redirect handlers)
Phase 1.4  Edit Makefile                      (obs-build, obs-dev, build target)
Phase 1.5  RUN: cd obs_theme && npm run build && go run ./cmd/plomvix
           CHECK: http://localhost:8080/login → 308 → /app/login → login page

Phase 2.1  CREATE obs_theme/src/api/client.ts
Phase 2.2  REPLACE obs_theme/src/store/authStore.ts
Phase 2.3  REPLACE obs_theme/src/lib/auth.ts
Phase 2.4  REPLACE obs_theme/src/pages/auth/components/LoginForm.tsx
Phase 2.5  EDIT obs_theme/src/App.tsx         (add SessionWatcher component)
Phase 2.x  RUN: npm run build && go run ./cmd/plomvix
           CHECK: login with admin/changeme → Dashboard loads at /app

Phase 3.1  CREATE obs_theme/src/api/adminApi.ts
Phase 3.2  REPLACE obs_theme/src/pages/users/index.tsx
Phase 3.x  RUN: npm run build && go run ./cmd/plomvix
           CHECK: /app/users shows real users; create/delete works

Phase 4    EDIT internal/server/ui_test.go (fix /login + /dev/design tests)
           RUN: go test ./internal/server/... → all pass
```

---

## What stays on mock data (intentionally)

| Page | Mock file | Status |
|------|-----------|--------|
| Dashboard | `src/pages/dashboard/mockData.ts` | ✅ existing mock |
| Logs | `src/pages/logs/mockData.ts` | ✅ stub exists (upgrade later) |
| Metrics | `src/pages/metrics/mockData.ts` | ✅ existing mock |
| Traces | `src/pages/traces/mockData.ts` | ✅ existing mock |
| APM | `src/pages/apm/mockData.ts` | ✅ existing mock |
| Alerts | `src/pages/alerts/mockData.ts` | ✅ existing mock |
| Incidents | `src/pages/incidents/mockData.ts` | ✅ existing mock |
| Synthetics | inline mock | ✅ existing mock |

---

## Constraints for the executing agent

1. **Do not touch `ui/` source files.** Keep all `ui-*` Makefile targets.
2. **No new npm packages.** Every dep needed (MUI, Zustand, react-router-dom, sonner,
   react-hook-form, zod, @monaco-editor/react, @tanstack/react-virtual, echarts,
   echarts-for-react, dayjs) is already in `obs_theme/package.json`.
3. **No `@/` alias.** obs_theme uses relative imports exclusively. Do not add path
   aliases to vite.config.ts or tsconfig.
4. **Keep `logout()` synchronous.** Topbar calls it without await. Do not change Topbar.
5. **`obs_theme/docs/`** is the GitHub Pages build — never modify it. Only `obs_theme/dist/`
   is served by Go.
6. **Vite dev port must be 3000** (set via `server: { port: 3000 }` in vite.config.ts)
   to match the Go dev proxy target `http://localhost:3000`.
7. **Backend has no email field for users.** Do not show an email column anywhere.
8. **`SessionWatcher` must be inside `<BrowserRouter>`** to use `useNavigate()`.
9. **`loginDemo` must be kept** as a no-op in authStore — it may be called from
   `LoginPage.tsx` or `DemoCredentials.tsx` which are not being replaced.
