# Plomvix UI Authentication

## Login Flow

1. User navigates to `/login` and enters credentials.
2. Browser posts `{username, password}` to `POST /auth/login`.
3. Server responds with a JWT token in the response body AND a `Set-Cookie: plomvix_token` header.
4. The app redirects to the page specified by the `?next=` query parameter (if safe), otherwise `/app/explore`.

## Logout Flow

1. User clicks "Logout" in the shell navigation.
2. Browser navigates to `/logout`.
3. The page calls `POST /auth/logout`, which invalidates the server-side token and clears the cookie.
4. The user is redirected to `/login`.

## Refresh-on-401 Flow

The centralized API client automatically refreshes expired sessions:

1. A request to a protected API endpoint returns HTTP 401.
2. The client calls `POST /auth/refresh` with `credentials: "include"` (using the existing cookie).
3. If refresh succeeds, the server sets a new `plomvix_token` cookie, and the original request is retried once.
4. If refresh fails, the user is redirected to `/login`.

## Cookie Security Properties

| Property | Value | Reason |
|---|---|---|
| **Name** | `plomvix_token` | |
| **HttpOnly** | `true` | Not accessible to JavaScript — prevents XSS token theft |
| **Path** | `/` | Available for all API and UI routes |
| **SameSite** | `Lax` | Prevents CSRF from external sites while allowing same-site navigation |
| **Secure** | `true` in production | Only transmitted over HTTPS |
| **MaxAge** | Matches JWT expiry | Cookie expires when the JWT expires |

## Protected Route Behaviour

- Routes under `/app/*` and `/dev/design` require authentication.
- Unauthenticated visitors are redirected to `/login?next=<original_path>`.
- After login, the `next` parameter is validated — only paths starting with `/app/` or `/dev/design` are accepted.
- External URLs, protocol-relative URLs, and malformed inputs are rejected and default to `/app/explore`.
- The app shell (sidebar, navigation) only renders after the auth state is resolved.

## How `/login?next=` Works

1. User visits `/app/admin` without being authenticated.
2. `ProtectedRoute` redirects to `/login?next=%2Fapp%2Fadmin`.
3. After successful login, `safeNextPath()` validates the `next` parameter:
   - `/app/admin` ✓ (starts with `/app/`)
   - `/app` ✓
   - `/dev/design` ✓
   - `https://evil.com` ✗ (external, rejected)
   - `//evil.com` ✗ (protocol-relative, rejected)
   - `null` ✗ (defaults to `/app/explore`)

## Why JWT is Not Stored in localStorage

Plomvix uses httpOnly cookies for browser session management instead of storing JWTs in `localStorage` or `sessionStorage`. This is a deliberate security decision:

- **httpOnly cookies** cannot be read by JavaScript, which means an XSS vulnerability cannot steal the token.
- **localStorage** is accessible to ALL JavaScript running on the domain, including third-party scripts.
- **SameSite=Lax** cookies provide built-in CSRF protection without requiring custom CSRF tokens.

API clients (non-browser) continue to use the JWT bearer token response body for authentication.
