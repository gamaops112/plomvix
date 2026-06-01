# Plomvix Auth API Reference

## Authentication

Plomvix supports three authentication methods. Include one on every request to a protected endpoint:

- **JWT Bearer token** (human users): `Authorization: Bearer <jwt_token>`
- **API Key** (machine/service clients): `X-API-Key: <api_key>`
- **Cookie session** (browser UI): `plomvix_token` httpOnly cookie set by login

If multiple auth methods are present, the order of precedence is: `X-API-Key`, `Authorization: Bearer`, `plomvix_token` cookie.

---

## Endpoints

### Public

#### `GET /health`
Health check. Returns version, uptime, and environment info. No auth required.

#### `POST /auth/login`
Authenticate with username and password. Returns a JWT token.

When accessed from a browser, the response also sets a `plomvix_token` httpOnly cookie with properties:
- HttpOnly: true
- Path: /
- SameSite: Lax
- Secure: true in production, false in development
- MaxAge matches JWT expiry

Browser clients should use `credentials: "include"` on API requests instead of storing the JWT token. The JWT response body is still available for API clients.

**Request:**
```json
{"username": "admin", "password": "changeme"}
```

**Response (200):**
```json
{
  "status": "ok",
  "data": {
    "token": "<jwt>",
    "expires_in": 3600,
    "user": {"id": "...", "username": "admin", "role": "admin", "created_at": "...", "updated_at": "..."}
  }
}
```

**Errors:**
- `400 VALIDATION_FAILED` — missing username or password
- `401 UNAUTHORIZED` — invalid username or password

---

### Protected (auth required)

#### `POST /auth/logout`
Invalidate the current JWT token. Clears the `plomvix_token` cookie when present. Always returns 200 for idempotent logout.

**Auth:** JWT (Bearer or Cookie), API key. Cookie-based logout does not require Authorization header.

**Response (200):**
```json
{"status": "ok", "data": {"message": "logged out successfully"}}
```

#### `POST /auth/refresh`
Invalidate the current JWT and issue a new one. Accepts Authorization Bearer token or plomvix_token cookie.

For cookie-authenticated requests, sets a new plomvix_token cookie.

**Auth:** JWT (Bearer or Cookie) only

**Response (200):**
```json
{
  "status": "ok",
  "data": {"token": "<new_jwt>", "expires_in": 3600}
}
```

---

### Admin (auth + admin role required)

#### `POST /admin/users`
Create a new user.

**Request:**
```json
{"username": "newuser", "password": "password123"}
```

**Response (201):**
```json
{
  "status": "ok",
  "data": {"id": "...", "username": "newuser", "role": "admin", "created_at": "...", "updated_at": "..."}
}
```

**Errors:**
- `400 VALIDATION_FAILED` — username must be 3-64 chars (alphanumeric, underscore, hyphen); password must be 8+ chars
- `409 CONFLICT` — username already exists

#### `GET /admin/users`
List all users.

**Response (200):**
```json
{
  "status": "ok",
  "data": {"users": [...], "count": 1}
}
```

#### `GET /admin/users/{id}`
Get a user by ID.

**Response (200):**
```json
{"status": "ok", "data": {"id": "...", "username": "...", "role": "admin", ...}}
```

**Errors:** `404 NOT_FOUND`

#### `PATCH /admin/users/{id}`
Update username and/or password. Only provided fields are updated.

**Request:**
```json
{"username": "newname", "password": "newpassword123"}
```

**Response (200):** Updated `UserResponse`

**Errors:**
- `400 VALIDATION_FAILED` — validation failed
- `404 NOT_FOUND` — user not found
- `409 CONFLICT` — new username already taken

#### `DELETE /admin/users/{id}`
Delete a user. Cannot delete your own account or the last admin.

**Response (200):**
```json
{"status": "ok", "data": {"message": "user deleted"}}
```

**Errors:**
- `400 VALIDATION_FAILED` — self-deletion or last admin
- `404 NOT_FOUND`

#### `POST /admin/users/{id}/apikey`
Generate a new API key for the user. Replaces any existing key. The plaintext key is returned once and never stored.

**Response (201):**
```json
{
  "status": "ok",
  "data": {
    "api_key": "<plaintext_key>",
    "user_id": "...",
    "message": "Store this key securely. It will not be shown again."
  }
}
```

#### `DELETE /admin/users/{id}/apikey`
Revoke the user's API key. Idempotent — returns 200 if no key exists.

**Response (200):**
```json
{"status": "ok", "data": {"message": "API key revoked"}}
```

#### `GET /admin/users/{id}/apikey/status`
Check if a user has an API key configured. Never reveals the key.

**Response (200):**
```json
{"status": "ok", "data": {"has_api_key": true, "user_id": "..."}}
```

---

## Error Response Format

All errors follow this structure:
```json
{
  "status": "error",
  "error": {"code": "ERROR_CODE", "message": "Human-readable message", "details": []},
  "request_id": "..."
}
```

Standard error codes: `VALIDATION_FAILED`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`.

## Browser UI Authentication

The Plomvix web UI available at `/login` and `/app/*` uses httpOnly cookies for session management.

### Cookie Details
- **Name:** `plomvix_token`
- **HttpOnly:** `true` — not accessible to JavaScript (XSS protection)
- **Path:** `/` — available to all routes
- **SameSite:** `Lax` — prevents CSRF from external sites
- **Secure:** `true` in production — HTTPS only

### How It Works
1. Browser posts credentials to `POST /auth/login`
2. Server responds with a `Set-Cookie: plomvix_token=<jwt>; HttpOnly; Path=/; ...` header
3. All subsequent API requests from the browser include `credentials: "include"` to send the cookie
4. Server middleware checks for the cookie when no `Authorization` header is present
5. `POST /auth/logout` clears the cookie

### Security Rules
- **Never store the JWT in `localStorage` or `sessionStorage`**. The token lives only in the httpOnly cookie.
- The API client automatically retries a failed request once after receiving 401 by calling `POST /auth/refresh`.
- If refresh fails, the user is redirected to `/login`.
- The `/login?next=` parameter allows safe redirects after login (only `/app/*` and `/dev/design` paths are accepted).
