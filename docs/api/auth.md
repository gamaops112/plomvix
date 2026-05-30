# Plomvix Auth API Reference

## Authentication

Plomvix supports two authentication methods. Include one on every request to a protected endpoint:

- **JWT Bearer token** (human users): `Authorization: Bearer <jwt_token>`
- **API Key** (machine/service clients): `X-API-Key: <api_key>`

If both headers are present, `X-API-Key` takes priority. A failed API key check returns 401 immediately — the JWT is not checked as fallback.

---

## Endpoints

### Public

#### `GET /health`
Health check. Returns version, uptime, and environment info. No auth required.

#### `POST /auth/login`
Authenticate with username and password. Returns a JWT token.

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
Invalidate the current JWT token. Returns 200 if authenticated via API key (nothing to blacklist).

**Auth:** JWT or API key

**Response (200):**
```json
{"status": "ok", "data": {"message": "logged out successfully"}}
```

#### `POST /auth/refresh`
Invalidate the current JWT and issue a new one.

**Auth:** JWT only

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
