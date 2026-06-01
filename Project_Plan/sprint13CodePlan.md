# Plomvix — Sprint 13 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–12 are complete. Sprint 11 added the UI foundation. Sprint 12 added the
Theme Engine and Developer Design Panel. Sprint 13 adds **Cookie Auth + Login/Logout UI + Protected Routes**.

Sprint 13 is the first sprint where the browser UI becomes a real authenticated
application. Existing API clients must keep working exactly as before: `POST /auth/login`
still returns a JWT response body, and `Authorization: Bearer <token>` still works.
Browser clients additionally receive and use an httpOnly cookie named `plomvix_token`.

**What Sprint 13 delivers:**
- `POST /auth/login` sets httpOnly cookie `plomvix_token`
- `POST /auth/logout` clears `plomvix_token`
- `POST /auth/refresh` accepts cookie auth as well as bearer auth and refreshes the cookie
- Auth middleware accepts API key, bearer JWT, or `plomvix_token` cookie
- Existing JWT/API-key API behaviour remains backward compatible
- `/login` route in the React app
- `/logout` route in the React app
- Protected route wrapper for `/app/*`
- Unauthenticated users redirect to `/login`
- Auth provider and `useAuth()` hook
- API client wrapper with centralized error handling, credentials included, and one refresh retry on 401
- Inline login error states
- Toast events for auth errors and session expiry
- Go static UI serving supports `/login` and `/logout` SPA routes in addition to `/app/*`
- Vite dev proxy supports `/login` and `/logout` SPA routes
- Backend tests for cookie set, cookie clear, cookie middleware auth, and refresh
- Frontend tests for protected route behaviour and login UI where possible
- OpenAPI and docs updated

**What Sprint 13 does NOT do:**
- No admin dashboard — Sprint 14
- No log explorer — Sprint 15
- No trace storage or trace UI — Sprint 16
- No OTLP or Prometheus remote write — Sprint 17
- No RBAC expansion — all authenticated users are still admin in current v1 flow
- No localStorage JWT persistence — browser auth uses httpOnly cookies only
- No OAuth, SSO, password reset, or user profile UI
- No CSRF token framework — SameSite=Lax cookie is sufficient for Sprint 13 local UI scope

---

## AUTH DESIGN — READ BEFORE WRITING ANY CODE

Sprint 2 already supports JWT login, logout, refresh, API keys, and admin routes.
Sprint 13 extends that system for browser UI sessions without breaking API clients.

**Cookie name:**
```
plomvix_token
```

**Cookie properties:**
- `HttpOnly: true`
- `Path: /`
- `SameSite: http.SameSiteLaxMode`
- `Secure: cfg.IsProduction()`
- `MaxAge: cfg.Auth.JWTExpirySeconds`
- `Expires: token expiry time`

**Cookie clearing properties:**
- same name and path
- empty value
- `MaxAge: -1`
- expired `Expires` time
- same `HttpOnly`, `SameSite`, and `Secure` rules

**Auth precedence in middleware:**
1. `X-API-Key` header
2. `Authorization: Bearer <jwt>` header
3. `plomvix_token` cookie

If an API key is present but invalid, return 401 immediately. Do not fall back to
bearer or cookie auth. This preserves the Sprint 2 middleware contract.

**Browser token storage rule:**
The React app must not store JWTs in localStorage or sessionStorage. The JWT body
returned by login may be kept in memory for the current page lifetime if needed,
but the durable browser session is the httpOnly cookie.

**Refresh rule:**
The API client wrapper retries a failed request once after receiving 401:
1. call `POST /auth/refresh` with `credentials: "include"`
2. if refresh succeeds, retry the original request once
3. if refresh fails, clear local auth state, emit a toast, redirect to `/login`

---

## ROUTING DESIGN — READ BEFORE WRITING ANY CODE

Sprint 11 serves the SPA from `/app/*`. Sprint 13 adds top-level SPA routes:

| Route | Purpose | Protected |
|---|---|---|
| `/login` | Login page | No |
| `/logout` | Logout action page | No, but calls logout endpoint |
| `/app/*` | Main application shell | Yes |
| `/dev/design` | Developer Design Panel from Sprint 12 | Yes when visible |

The Go server must serve the built SPA for `/login`, `/logout`, `/app/*`, and
`/dev/design` in production UI mode. In UI dev mode, these routes must proxy to
Vite so browser refresh works during development.

---

## FRONTEND DESIGN — READ BEFORE WRITING ANY CODE

Sprint 11 already created the app shell, route registry, sidebar, app event provider,
toast system, and button primitive. Sprint 12 added theme provider and design panel.
Sprint 13 must build on those instead of replacing them.

**Frontend auth files to add:**
```
ui/src/auth/types.ts
ui/src/auth/AuthContext.tsx
ui/src/auth/useAuth.ts
ui/src/auth/ProtectedRoute.tsx
ui/src/api/client.ts
ui/src/pages/LoginPage.tsx
ui/src/pages/LogoutPage.tsx
```

**UI behaviour:**
- `/login` renders outside the app shell
- Login form fields: username, password
- Submit button is disabled while logging in
- Invalid credentials show inline error and toast
- Successful login redirects to `/app/explore`
- `/logout` calls backend logout then redirects to `/login`
- Visiting `/app/*` while unauthenticated redirects to `/login?next=<path>`
- After login, if `next` is safe and starts with `/app/` or `/dev/design`, redirect there
- Unsafe `next` values are ignored and redirect to `/app/explore`
- App shell shows only after auth state is resolved
- Loading state is shown while session refresh/check is in progress

---

## TASK 01 — Add backend cookie constants to internal/auth

**Action:** Create `internal/auth/cookie.go`.

**Full file content:**
```go
package auth

import (
    "net/http"
    "time"

    "github.com/plomvix/plomvix/internal/config"
)

// TokenCookieName is the browser session cookie used by the Plomvix UI.
const TokenCookieName = "plomvix_token"

// NewTokenCookie returns an httpOnly cookie containing a JWT.
func NewTokenCookie(token string, expires time.Time, cfg *config.Config) *http.Cookie {
    return &http.Cookie{
        Name:     TokenCookieName,
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        Secure:   cfg.IsProduction(),
        SameSite: http.SameSiteLaxMode,
        Expires:  expires,
        MaxAge:   cfg.Auth.JWTExpirySeconds,
    }
}

// NewClearTokenCookie returns an expired cookie that clears the browser session.
func NewClearTokenCookie(cfg *config.Config) *http.Cookie {
    return &http.Cookie{
        Name:     TokenCookieName,
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   cfg.IsProduction(),
        SameSite: http.SameSiteLaxMode,
        Expires:  time.Unix(0, 0),
        MaxAge:   -1,
    }
}
```

**Verify:**
```bash
go build ./internal/auth/
```

---

## TASK 02 — Add helper to extract token from request

**Action:** Create `internal/auth/request_token.go`.

**Full file content:**
```go
package auth

import (
    "net/http"
    "strings"
)

// TokenSource identifies where an incoming JWT was found.
type TokenSource string

const (
    TokenSourceNone   TokenSource = "none"
    TokenSourceBearer TokenSource = "bearer"
    TokenSourceCookie TokenSource = "cookie"
)

// TokenFromRequest returns a JWT from Authorization: Bearer or the UI cookie.
// Bearer token wins over cookie when both are present.
func TokenFromRequest(r *http.Request) (string, TokenSource) {
    authHeader := r.Header.Get("Authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
        if token != "" {
            return token, TokenSourceBearer
        }
    }

    cookie, err := r.Cookie(TokenCookieName)
    if err == nil && strings.TrimSpace(cookie.Value) != "" {
        return cookie.Value, TokenSourceCookie
    }

    return "", TokenSourceNone
}
```

**Verify:**
```bash
go build ./internal/auth/
```

---

## TASK 03 — Update JWT generation to expose expiry to handlers

**Action:** Add `GenerateTokenWithClaims` to `internal/auth/jwt.go` without removing
existing `GenerateToken`.

**Add this function:**
```go
// GenerateTokenWithClaims creates a signed JWT and returns both the token string
// and the claims used to build it. Handlers use ExpiresAt for cookie expiry.
func GenerateTokenWithClaims(user *User, cfg *config.Config) (string, *Claims, error) {
    now := time.Now()
    claims := &Claims{
        UserID:   user.ID,
        Username: user.Username,
        Role:     user.Role,
        JTI:      uuid.New().String(),
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt: jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(
                now.Add(time.Duration(cfg.Auth.JWTExpirySeconds) * time.Second)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(cfg.Auth.JWTSecret))
    if err != nil {
        return "", nil, err
    }
    return signed, claims, nil
}
```

**Then update `GenerateToken` to call it:**
```go
func GenerateToken(user *User, cfg *config.Config) (string, error) {
    token, _, err := GenerateTokenWithClaims(user, cfg)
    return token, err
}
```

**Verify:**
```bash
go build ./internal/auth/
go test ./internal/auth/...
```

---

## TASK 04 — Update auth middleware to accept cookie JWTs

**Action:** Modify `internal/auth/middleware.go`.

**Required behaviour:**
- Keep API key check first and exclusive.
- Replace direct bearer parsing with `TokenFromRequest(r)`.
- If token source is none, return 401.
- Parse token, check blacklist, load user, attach to context.
- Error messages remain compatible with Sprint 2.

**Implementation shape:**
```go
// Step 2: JWT check — Authorization Bearer first, then plomvix_token cookie.
tokenString, source := TokenFromRequest(r)
if source != TokenSourceNone {
    claims, err := ParseToken(tokenString, cfg)
    if err != nil {
        utils.Unauthorized(w, r, "invalid or expired token")
        return
    }
    if blacklist.IsBlacklisted(claims.JTI) {
        utils.Unauthorized(w, r, "token has been revoked")
        return
    }
    user, err := store.GetUserByID(claims.UserID)
    if err != nil {
        utils.Unauthorized(w, r, "user not found")
        return
    }
    next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
    return
}
```

**Verify:**
```bash
go build ./internal/auth/
go test ./internal/auth/...
```

---

## TASK 05 — Update login handler to set cookie

**Action:** Modify the existing login handler in `internal/auth/handler.go`.

**Required changes:**
- Replace `GenerateToken(user, cfg)` with `GenerateTokenWithClaims(user, cfg)`.
- Call `http.SetCookie(w, NewTokenCookie(token, claims.ExpiresAt.Time, cfg))` before writing response.
- Keep the existing response body unchanged for API clients.
- Do not log token or cookie value.

**Verify:**
```bash
go build ./internal/auth/
go test ./internal/auth/...
```

---

## TASK 06 — Update logout handler to clear cookie

**Action:** Modify the existing logout handler in `internal/auth/handler.go`.

**Required changes:**
- Use `TokenFromRequest(r)` so logout works with bearer token or cookie token.
- If a token is present and valid, add its JTI to the blacklist until expiry.
- Always call `http.SetCookie(w, NewClearTokenCookie(cfg))` before returning success.
- If no token exists, still clear cookie and return success to make logout idempotent.

**Verify:**
```bash
go build ./internal/auth/
go test ./internal/auth/...
```

---

## TASK 07 — Update refresh handler to support cookie sessions

**Action:** Modify the existing refresh handler in `internal/auth/handler.go`.

**Required changes:**
- Use `TokenFromRequest(r)` so refresh accepts bearer or cookie token.
- Validate old token and blacklist status.
- Load the user from the store.
- Generate a new token with `GenerateTokenWithClaims`.
- Set a new `plomvix_token` cookie with the new token.
- Return the existing refresh response body shape used by Sprint 2.

**Verify:**
```bash
go build ./internal/auth/
go test ./internal/auth/...
```

---

## TASK 08 — Add auth cookie unit tests

**Action:** Create `internal/auth/cookie_test.go`.

**Tests required:**
- `NewTokenCookie` sets name `plomvix_token`
- `HttpOnly` is true
- `Path` is `/`
- `SameSite` is Lax
- `Secure` is false in development
- `Secure` is true in production
- clear cookie has `MaxAge == -1`

**Verify:**
```bash
go test ./internal/auth/...
```

---

## TASK 09 — Add request token extraction tests

**Action:** Create `internal/auth/request_token_test.go`.

**Tests required:**
- bearer token is extracted when present
- cookie token is extracted when no bearer token exists
- bearer token wins when both bearer and cookie exist
- empty cookie returns `TokenSourceNone`
- missing auth returns `TokenSourceNone`

**Verify:**
```bash
go test ./internal/auth/...
```

---

## TASK 10 — Add login/logout/refresh integration tests for cookies

**Action:** Add tests to the existing auth handler test file, or create
`internal/auth/handler_cookie_test.go` if handler tests are already split.

**Tests required:**
- `POST /auth/login` returns a `Set-Cookie` header for `plomvix_token`
- cookie is `HttpOnly`
- cookie `MaxAge` matches config expiry
- `POST /auth/logout` returns a clear-cookie `Set-Cookie`
- `POST /auth/refresh` accepts a valid cookie and returns a new `Set-Cookie`
- cookie-authenticated request to a protected route succeeds

**Verify:**
```bash
go test ./internal/auth/... ./internal/server/...
```

---

## TASK 11 — Update Go UI static route matching for login/logout

**Action:** Modify `internal/server/server.go` or the existing UI serving file from Sprint 11.

**Required behaviour in production UI mode:**
Serve the SPA `index.html` for:
```text
/login
/logout
/app/*
/dev/design
```

**Path traversal rule:**
Preserve the Sprint 11 path traversal guard. Do not weaken it when adding new routes.

**Verify:**
```bash
go build ./internal/server/
go test ./internal/server/...
```

---

## TASK 12 — Update Vite dev proxy route handling

**Action:** Modify the existing Sprint 11 UI dev proxy route registration in Go.

**Required behaviour in UI dev mode:**
Proxy these SPA routes to Vite:
```text
/login
/logout
/app/*
/dev/design
```

API routes must still be handled by Go and must not be proxied to Vite.

**Verify:**
```bash
go build ./internal/server/
go test ./internal/server/...
```

---

## TASK 13 — Update OpenAPI auth cookie documentation

**Action:** Modify `api/openapi.json`.

**Required changes:**
- Add cookie auth scheme:
```json
"CookieAuth": {
  "type": "apiKey",
  "in": "cookie",
  "name": "plomvix_token",
  "description": "httpOnly browser session cookie set by POST /auth/login"
}
```
- Keep `BearerAuth` and `APIKeyAuth` unchanged.
- Protected endpoints may list `CookieAuth` as an alternative where browser UI uses it.
- `POST /auth/login` response description must mention `Set-Cookie`.
- `POST /auth/logout` response description must mention cookie clearing.
- `POST /auth/refresh` response description must mention cookie refresh.

**Verify:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json
```

---

## TASK 14 — Update auth API docs

**Action:** Update the existing auth documentation file under `docs/api/`.

**Required content:**
- Browser login sets `plomvix_token`
- API clients can still use bearer JWT response body
- Logout clears the cookie
- Refresh supports bearer or cookie auth
- Cookie properties: HttpOnly, SameSite=Lax, Secure in production
- Browser clients should use `credentials: include`
- Do not store JWTs in localStorage

**Verify:**
```bash
grep -R "plomvix_token" docs/api/
grep -R "credentials" docs/api/
```

---

## TASK 15 — Add frontend test dependencies if missing

**Action:** Inspect `ui/package.json`. If Vitest and Testing Library are not already
present, add these pinned dev dependencies:

```json
"@testing-library/jest-dom": "6.6.3",
"@testing-library/react": "16.2.0",
"@testing-library/user-event": "14.6.1",
"jsdom": "26.0.0",
"vitest": "3.0.8"
```

Add scripts if missing:
```json
"test": "vitest run",
"test:watch": "vitest"
```

Do not use `"latest"` for any dependency.

**Verify:**
```bash
cd ui
npm install
node -e "const p=require('./package.json'); const all={...p.dependencies,...p.devDependencies}; for (const [k,v] of Object.entries(all)) if (v==='latest') throw new Error(k+' uses latest')"
npm run typecheck
```

---

## TASK 16 — Add frontend test setup file

**Action:** Create `ui/src/test/setup.ts`.

**Full file content:**
```ts
import '@testing-library/jest-dom/vitest';
```

**Action:** Update Vite/Vitest config to use jsdom and the setup file.

**Required config shape:**
```ts
test: {
  environment: 'jsdom',
  setupFiles: './src/test/setup.ts',
}
```

**Verify:**
```bash
cd ui
npm run test -- --passWithNoTests
npm run typecheck
```

---

## TASK 17 — Create frontend auth types

**Action:** Create `ui/src/auth/types.ts`.

**Full file content:**
```ts
export type UserRole = 'admin';

export interface AuthUser {
  id: string;
  username: string;
  role: UserRole;
  created_at: string;
  updated_at: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponseData {
  token?: string;
  user: AuthUser;
  expires_at?: string;
}

export interface AuthState {
  user: AuthUser | null;
  loading: boolean;
  authenticated: boolean;
}
```

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 18 — Create API client wrapper

**Action:** Create `ui/src/api/client.ts`.

**Requirements:**
- All requests use `credentials: 'include'`
- JSON requests set `Content-Type: application/json`
- Parse JSON only when response has a JSON content type
- On 401, attempt one refresh by calling `POST /auth/refresh`
- Retry original request once after successful refresh
- If refresh fails, emit an app event/toast for session expiry
- Throw a typed `ApiError` containing status, code, and message
- Do not store JWT in localStorage or sessionStorage

**Public API:**
```ts
export class ApiError extends Error {
  status: number;
  code?: string;
}

export async function apiGet<T>(path: string): Promise<T>;
export async function apiPost<T>(path: string, body?: unknown): Promise<T>;
export async function apiPut<T>(path: string, body?: unknown): Promise<T>;
export async function apiDelete<T>(path: string): Promise<T>;
export async function apiRequest<T>(path: string, init?: RequestInit, retry?: boolean): Promise<T>;
```

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 19 — Create AuthContext provider

**Action:** Create `ui/src/auth/AuthContext.tsx`.

**Requirements:**
- On mount, call `POST /auth/refresh` with `credentials: 'include'`
- If refresh succeeds, set `user` from response data
- If refresh fails, set user to null without showing an error toast on initial load
- Expose `login(username, password)`
- Expose `logout()`
- Expose `refresh()`
- Do not store token in localStorage/sessionStorage
- Login redirects are handled by pages, not the provider

**Context value shape:**
```ts
interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  authenticated: boolean;
  login: (username: string, password: string) => Promise<AuthUser>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthUser | null>;
}
```

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 20 — Create useAuth hook

**Action:** Create `ui/src/auth/useAuth.ts`.

**Full file content:**
```ts
import { useContext } from 'react';
import { AuthContext } from './AuthContext';

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used inside AuthProvider');
  }
  return ctx;
}
```

If `AuthContext` was not exported from `AuthContext.tsx`, export it.

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 21 — Create ProtectedRoute component

**Action:** Create `ui/src/auth/ProtectedRoute.tsx`.

**Requirements:**
- While auth is loading, show a centered loading state
- If unauthenticated, redirect to `/login?next=<current path>`
- If authenticated, render child route content
- Use React Router primitives already present from Sprint 11
- Preserve query string when building `next`

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 22 — Create safe redirect helper

**Action:** Create `ui/src/auth/redirect.ts`.

**Full file content:**
```ts
export function safeNextPath(raw: string | null): string {
  if (!raw) return '/app/explore';

  try {
    const decoded = decodeURIComponent(raw);
    if (decoded.startsWith('/app/') || decoded === '/app' || decoded === '/dev/design') {
      return decoded;
    }
    return '/app/explore';
  } catch {
    return '/app/explore';
  }
}
```

**Verify:**
```bash
cd ui
npm run typecheck
```

---

## TASK 23 — Create LoginPage

**Action:** Create `ui/src/pages/LoginPage.tsx`.

**Requirements:**
- Render outside app shell
- Use current theme CSS variables from Sprint 12
- Fields: username and password
- Client validation: both fields required
- Submit calls `auth.login(username, password)`
- While submitting, disable submit button and show loading text
- On success, redirect to `safeNextPath(searchParams.get('next'))`
- On failure, show inline error and emit toast event
- Do not display raw backend stack traces

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 24 — Create LogoutPage

**Action:** Create `ui/src/pages/LogoutPage.tsx`.

**Requirements:**
- On mount, call `auth.logout()`
- Redirect to `/login` after logout completes
- If logout request fails, still clear local auth state and redirect to `/login`
- Show a small “Signing out…” loading state while request is in flight

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 25 — Wrap app with AuthProvider

**Action:** Modify `ui/src/main.tsx` or the existing provider composition file.

**Required provider order:**
The app must be wrapped so auth can emit events and use theme safely.

Recommended order:
```tsx
<AppEventProvider>
  <ThemeProvider>
    <AuthProvider>
      <App />
    </AuthProvider>
  </ThemeProvider>
</AppEventProvider>
```

If Sprint 11/12 uses a different composition file, preserve the existing structure
and insert `AuthProvider` without removing existing providers.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 26 — Register login and logout routes

**Action:** Modify the existing route registry from Sprint 11.

**Required changes:**
- Add `/login` route mapped to `LoginPage`
- Add `/logout` route mapped to `LogoutPage`
- Mark both as hidden from sidebar/navigation
- Ensure `/login` and `/logout` do not render inside the app shell

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 27 — Protect app routes

**Action:** Modify the app routing component.

**Required changes:**
- Wrap `/app/*` routes with `ProtectedRoute`
- Wrap `/dev/design` with `ProtectedRoute`
- Keep `/login` and `/logout` unprotected
- Unknown routes should redirect to `/app/explore` if authenticated, otherwise `/login`

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 28 — Add logout control to app shell

**Action:** Modify the existing app shell/sidebar/navbar from Sprint 11.

**Requirements:**
- Show current username when authenticated
- Add a Logout button or sidebar item linking to `/logout`
- Do not show logout on `/login`
- Use existing button primitive and theme variables

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 29 — Update theme design panel API calls to use api client

**Action:** Modify Sprint 12 design panel frontend code.

**Required changes:**
- Replace raw `fetch` calls for save/reset/export with the centralized API client
- Ensure credentials are included
- On 401 after refresh failure, user is redirected to `/login`
- Keep clear error toasts for failed save/reset/export

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 30 — Add login page tests

**Action:** Create `ui/src/pages/LoginPage.test.tsx`.

**Tests required:**
- renders username and password fields
- shows inline error when submitting empty form
- calls login when form is valid
- disables submit button while login is pending
- redirects to safe `next` path after success
- unsafe `next` redirects to `/app/explore`

Mock network/auth context as needed. Do not require a real Go server.

**Verify:**
```bash
cd ui
npm run test -- LoginPage
npm run typecheck
```

---

## TASK 31 — Add protected route tests

**Action:** Create `ui/src/auth/ProtectedRoute.test.tsx`.

**Tests required:**
- shows loading state while auth is loading
- renders children when authenticated
- redirects unauthenticated user to `/login?next=...`
- preserves current query string in next path

**Verify:**
```bash
cd ui
npm run test -- ProtectedRoute
npm run typecheck
```

---

## TASK 32 — Add redirect helper tests

**Action:** Create `ui/src/auth/redirect.test.ts`.

**Tests required:**
- accepts `/app/explore`
- accepts `/dev/design`
- rejects absolute external URLs
- rejects protocol-relative URLs
- rejects malformed URI input
- defaults to `/app/explore`

**Verify:**
```bash
cd ui
npm run test -- redirect
npm run typecheck
```

---

## TASK 33 — Add API client tests

**Action:** Create `ui/src/api/client.test.ts`.

**Tests required:**
- sends requests with `credentials: include`
- parses JSON success response
- throws `ApiError` on non-2xx response
- attempts refresh once on 401
- retries original request after successful refresh
- does not infinite-loop when refresh fails

Mock `global.fetch` directly. Do not use a real server.

**Verify:**
```bash
cd ui
npm run test -- client
npm run typecheck
```

---

## TASK 34 — Update Makefile UI test target

**Action:** Modify `Makefile`.

**Required changes:**
Add target:
```makefile
ui-test:
	cd ui && npm run test
```

Update `check` target if it exists so it includes `ui-test` after `ui-build`.
If `check` currently only runs Go checks, add UI checks without removing existing Go checks.

**Verify:**
```bash
make ui-test
make check
```

---

## TASK 35 — Update README UI auth section

**Action:** Modify `README.md`.

**Required content:**
- Browser UI is available at `/login` and `/app/explore`
- Default admin credentials come from `config.yaml`
- Login sets an httpOnly cookie
- API clients may still use JWT bearer tokens or API keys
- Do not store JWTs in browser localStorage
- `make dev` starts Go + Vite dev flow if Sprint 11 target exists

**Verify:**
```bash
grep -n "/login" README.md
grep -n "plomvix_token\|httpOnly" README.md
```

---

## TASK 36 — Update docs for UI authentication

**Action:** Create or update `docs/ui/auth.md`.

**Required content:**
- Login flow
- Logout flow
- Refresh-on-401 flow
- Cookie security properties
- Protected route behaviour
- How `/login?next=` works
- Why JWT is not stored in localStorage

**Verify:**
```bash
grep -n "Refresh" docs/ui/auth.md
grep -n "localStorage" docs/ui/auth.md
grep -n "next" docs/ui/auth.md
```

---

## TASK 37 — Update integration test helpers for cookie auth

**Action:** Modify `tests/integration/helpers_test.go`.

**Required changes:**
- Add helper `adminCookieJar(t, baseURL)` or equivalent
- Login helper should preserve cookies from `Set-Cookie`
- Existing bearer token helpers must continue to work
- Add helper for browser-style requests using cookie jar and no Authorization header

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/... -run TestNonExistent -count=1
```

The command may report no tests matched, but the package must compile.

---

## TASK 38 — Add integration test for browser cookie login flow

**Action:** Create `tests/integration/auth_cookie_test.go`.

**Test required:** `TestBrowserCookieLoginFlow`

**Flow:**
1. Start test server
2. POST `/auth/login` with admin credentials using an HTTP client with cookie jar
3. Assert response is 200
4. Assert jar contains `plomvix_token`
5. Call a protected endpoint without Authorization header
6. Assert protected endpoint succeeds using only cookie auth
7. POST `/auth/logout`
8. Assert subsequent protected endpoint request returns 401

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/... -run TestBrowserCookieLoginFlow -count=1
```

---

## TASK 39 — Add integration test for refresh cookie flow

**Action:** Add test `TestBrowserCookieRefreshFlow` to `tests/integration/auth_cookie_test.go`.

**Flow:**
1. Start test server
2. Login with cookie jar
3. Call `POST /auth/refresh` with no Authorization header
4. Assert response is 200
5. Assert a new `Set-Cookie` for `plomvix_token` is returned
6. Assert protected endpoint still succeeds with cookie only

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/... -run TestBrowserCookieRefreshFlow -count=1
```

---

## TASK 40 — Add backend route smoke test for SPA auth routes

**Action:** Add or update server tests under `internal/server/`.

**Tests required:**
- production UI mode serves SPA for `/login`
- production UI mode serves SPA for `/logout`
- production UI mode serves SPA for `/app/explore`
- production UI mode serves SPA for `/dev/design`
- API route `/auth/login` is not swallowed by SPA fallback

Use temporary `ui/dist/index.html` fixture as Sprint 11 server tests do.

**Verify:**
```bash
go test ./internal/server/...
```

---

## TASK 41 — Run frontend test suite

**Action:** Run the full frontend checks.

**Verify:**
```bash
cd ui
npm run typecheck
npm run test
npm run build
```

All commands must pass with zero errors.

---

## TASK 42 — Run backend test suite

**Action:** Run backend tests.

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./...
```

All packages must pass.

---

## TASK 43 — Run lint and formatting

**Action:** Format all Go files and run lint.

```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 make lint
```

**Verify:**
```bash
CGO_ENABLED=1 make lint
```

Must exit with code 0.

---

## TASK 44 — Final Sprint 13 verification

**Action:** Run the full project verification.

**Verify:**
```bash
make ui-build
make ui-test
CGO_ENABLED=1 make test
CGO_ENABLED=1 make integration-test
CGO_ENABLED=1 make build
CGO_ENABLED=1 make check
```

All commands must pass.

---

## FINAL ACCEPTANCE CHECKLIST

Sprint 13 is complete only when all of these are true:

- `POST /auth/login` still returns JWT response body for API clients
- `POST /auth/login` sets httpOnly `plomvix_token` cookie for browsers
- `POST /auth/logout` clears `plomvix_token`
- `POST /auth/refresh` works with bearer token and cookie token
- Auth middleware accepts API key, bearer JWT, and cookie JWT in the documented order
- Invalid API key does not fall back to bearer or cookie auth
- Browser UI does not store JWTs in localStorage or sessionStorage
- `/login` route exists and renders outside app shell
- `/logout` route signs out and redirects to `/login`
- `/app/*` routes require authentication
- `/dev/design` requires authentication when present
- Unauthenticated app visits redirect to `/login?next=...`
- Safe `next` redirects work; unsafe external redirects are rejected
- Central API client sends `credentials: include`
- Central API client retries once after refresh on 401
- Go static UI serving supports `/login`, `/logout`, `/app/*`, and `/dev/design`
- Vite dev proxy supports `/login`, `/logout`, `/app/*`, and `/dev/design`
- OpenAPI documents cookie auth
- README and UI auth docs are updated
- Frontend tests pass
- Backend tests pass
- Integration cookie auth tests pass
- `make test`, `make lint`, `make build`, and `make check` pass
