# Plomvix — Sprint 2

> **Plomvix** is an Indian-built, open-source, unified observability and general-purpose database
> supporting logs, metrics, telemetry, key-value, and JSON data.
> Built in Go. Production grade. Resource friendly.

---

## Architecture Layers Overview

```
Layer 1  →  Project Skeleton + Config + Logging        ← Sprint 1 ✅
Layer 2  →  Auth System (JWT + API Key)                ← Sprint 2
Layer 3  →  Write Ahead Log (WAL)
Layer 4  →  Hot Tier (RocksDB)
Layer 5  →  Ingestion API + Schema Inference Engine
Layer 6  →  SQL Query Engine (Hot Tier)
Layer 7  →  Cold Tier (Parquet) + Tiering Policy
Layer 8  →  Multi-Format Parsers
Layer 9  →  Admin APIs + Swagger Docs
Layer 10 →  Polish + Testing + Documentation
```

---

## Sprint 2 Goal

> By the end of Sprint 2, Plomvix has a fully working authentication system.
> Every API endpoint is protected. Users can log in with a password and receive a JWT,
> or authenticate silently using an API key. The default admin user is created
> automatically on first boot. Admins can create and manage users and API keys.
> No request gets through without a valid identity attached to it.
> All endpoints are fully documented for developers.

---

## Storage Decision — Why BoltDB for Sprint 2

Sprint 4 introduces RocksDB as the full hot tier storage engine. Pulling RocksDB
into Sprint 2 just for user storage would mean:
- CGO dependency and system library requirements before the project is ready
- A half-integrated RocksDB that gets refactored in Sprint 4 anyway

**Decision: use BoltDB for user storage in Sprint 2.**

BoltDB is:
- Pure Go — zero CGO, single import, no system dependencies
- Embedded — runs in-process, no separate server
- ACID compliant — safe for user records
- Proven — used by etcd, InfluxDB, and others

In Sprint 4, when RocksDB lands as the full hot tier, user data migrates from BoltDB
into the `_system` namespace of RocksDB. The migration is a single Sprint 4 story.

BoltDB file lives at: `{data_dir}/system/auth.db`

**New directory to add to bootstrap in main.go:**
```
{data_dir}/system/
```

**`.gitignore` update required (Story 8.2):**
```gitignore
data/system/*
!data/system/.gitkeep
```

---

&nbsp;

## Feature 1 — User Storage Layer

> The foundation everything else in Sprint 2 builds on.
> Defines what a User is, how it is stored, and how it is retrieved.
> All other auth features consume this layer.

---

### Story 1.1 — Add BoltDB Dependency

**What:**
Add BoltDB to the Go module.

```bash
go get go.etcd.io/bbolt
go mod tidy
```

**Files affected:**
```
go.mod
go.sum
```

**Acceptance Criteria:**
- `go.etcd.io/bbolt` appears in `go.mod`
- `go mod tidy` exits with no errors
- No other new dependencies introduced

---

### Story 1.2 — Add bcrypt and JWT Dependencies

**What:**
Add the two remaining third-party libraries Sprint 2 needs.

```bash
go get golang.org/x/crypto
go get github.com/golang-jwt/jwt/v5
go mod tidy
```

| Library | Purpose |
|---|---|
| `golang.org/x/crypto` | `bcrypt` for password hashing |
| `github.com/golang-jwt/jwt/v5` | JWT signing and validation |

**Acceptance Criteria:**
- Both libraries appear in `go.mod`
- `go mod tidy` exits with no errors

---

### Story 1.3 — Define the User Model

**What:**
Create `internal/auth/model.go` defining the `User` struct and all associated types.

**File:** `internal/auth/model.go`

```go
// Role defines the permission level of a user.
// In Sprint 2 all created users are Admin.
// RBAC with finer roles is deferred to a future version.
type Role string

const (
    RoleAdmin Role = "admin"
)

// User represents a Plomvix user account.
// IMPORTANT: PasswordHash and APIKeyHash use json:"-" so they can NEVER
// appear in a JSON response even if a User is accidentally serialized directly.
type User struct {
    ID           string    `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"-"` // bcrypt hash — excluded from all JSON output
    Role         Role      `json:"role"`
    APIKeyHash   string    `json:"-"` // bcrypt hash of API key — excluded from all JSON output
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// UserResponse is the safe public representation of a User.
// It never contains PasswordHash or APIKeyHash.
type UserResponse struct {
    ID        string    `json:"id"`
    Username  string    `json:"username"`
    Role      Role      `json:"role"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse converts a User to its safe public representation.
// This is the ONLY way handlers should convert a User for API output.
func (u *User) ToResponse() UserResponse {
    return UserResponse{
        ID:        u.ID,
        Username:  u.Username,
        Role:      u.Role,
        CreatedAt: u.CreatedAt,
        UpdatedAt: u.UpdatedAt,
    }
}
```

**Acceptance Criteria:**
- `PasswordHash` and `APIKeyHash` use `json:"-"` — excluded from ALL JSON output
- `ToResponse()` is the only way to convert a `User` to an API-safe struct
- No handler in Sprint 2 serializes a `User` directly — always `UserResponse`
- `go build ./internal/auth/` compiles with no errors

---

### Story 1.4 — Implement the User Store

**What:**
Create `internal/auth/store.go` that implements all user persistence operations
using BoltDB. This is the only file in the project that talks to `auth.db`.

**BoltDB layout:**
```
auth.db
└── bucket: "users"
    └── key: user.ID (UUID string)
        value: JSON-encoded User struct
```

**Sentinel errors — define at package level:**
```go
var (
    ErrUserNotFound      = errors.New("user not found")
    ErrUserAlreadyExists = errors.New("username already exists")
)
```

**Public API:**
```go
// Store manages persistent user storage backed by BoltDB.
type Store struct {
    db *bbolt.DB
}

// NewStore opens (or creates) the BoltDB database at the given path.
// Creates the "users" bucket if it does not exist.
// Returns an error if the file cannot be opened.
func NewStore(path string) (*Store, error)

// Close closes the BoltDB database. Call on shutdown.
func (s *Store) Close() error

// CreateUser persists a new User. Returns ErrUserAlreadyExists if username is taken.
func (s *Store) CreateUser(u *User) error

// GetUserByID returns the User with the given ID. Returns ErrUserNotFound if missing.
func (s *Store) GetUserByID(id string) (*User, error)

// GetUserByUsername returns the User with the given username. Returns ErrUserNotFound if missing.
func (s *Store) GetUserByUsername(username string) (*User, error)

// UpdateUser replaces the stored User record. Returns ErrUserNotFound if the user does not exist.
func (s *Store) UpdateUser(u *User) error

// DeleteUser removes the user with the given ID. Returns ErrUserNotFound if missing.
func (s *Store) DeleteUser(id string) error

// ListUsers returns all users in the store, ordered by CreatedAt ascending.
func (s *Store) ListUsers() ([]*User, error)

// UserExists returns true if a user with the given username exists.
func (s *Store) UserExists(username string) (bool, error)
```

**Acceptance Criteria:**
- All operations are wrapped in BoltDB transactions — no partial writes
- `CreateUser` checks for duplicate username and returns `ErrUserAlreadyExists` if found
- `GetUserByUsername` scans all users and matches by username field — not by key
- `ListUsers` returns a stable order (sorted by `CreatedAt`)
- `Close()` is called during graceful shutdown
- `go build ./internal/auth/` compiles with no errors

---

### Story 1.5 — Bootstrap Default Admin User

**What:**
On every startup, before the HTTP server starts, check if any users exist.
If the store is empty, create the default admin user from config values.

**Logic — implement in `internal/auth/bootstrap.go`:**
```go
// BootstrapAdminUser creates the default admin user if no users exist in the store.
// Uses credentials from config: auth.default_admin_username and auth.default_admin_password.
// Safe to call on every startup — does nothing if users already exist.
func BootstrapAdminUser(store *Store, cfg *config.Config) error
```

**Bootstrap sequence:**
```
1. Call store.ListUsers()
2. If len(users) > 0 → return nil (users already exist, nothing to do)
3. Hash cfg.Auth.DefaultAdminPassword with bcrypt (cost: 12)
4. Generate new UUID v4 for user ID
5. Generate a random API key (length from cfg.Auth.APIKeyLength bytes, base64 encoded)
6. Hash the API key with bcrypt (cost: 12)
7. Create User{
       ID:           newUUID,
       Username:     cfg.Auth.DefaultAdminUsername,
       PasswordHash: hashedPassword,
       Role:         RoleAdmin,
       APIKeyHash:   hashedAPIKey,
       CreatedAt:    time.Now(),
       UpdatedAt:    time.Now(),
   }
8. Call store.CreateUser(&user)
9. Log at INFO: "default admin user created"
   Fields: zap.String("username", cfg.Auth.DefaultAdminUsername)
   NEVER log the password or API key plaintext
10. Log at WARN in ALL environments:
    "default admin credentials are set to defaults — change before exposing to network"
    This warning applies in development AND production, because even dev environments
    may be accidentally network-accessible.
    Note: production startup is already blocked by config validation if password == "changeme"
```

**Acceptance Criteria:**
- Running on a fresh install creates exactly one admin user
- Running on an existing install with users does nothing
- Password is bcrypt hashed before storage — plaintext never touches the store
- API key is generated randomly and bcrypt hashed before storage
- No secrets are logged
- Warning log fires in all environments
- `go build ./internal/auth/` compiles with no errors

---

&nbsp;

## Feature 2 — Password Authentication + JWT

> The human login flow. A user provides credentials, receives a JWT,
> and uses that JWT for all subsequent requests until it expires or is revoked.

---

### Story 2.1 — Password Hashing Utilities

**What:**
Create `internal/auth/password.go` with helpers for password hashing and verification.

```go
// HashPassword hashes a plaintext password using bcrypt with cost 12.
// Returns the hash string or an error.
func HashPassword(password string) (string, error)

// CheckPassword compares a plaintext password against a bcrypt hash.
// Returns nil if they match, non-nil error if they do not.
func CheckPassword(password, hash string) error
```

**Acceptance Criteria:**
- bcrypt cost is `12` — hardcoded, not configurable (intentional)
- `CheckPassword` returns a non-nil error on mismatch
- `go build ./internal/auth/` compiles with no errors

---

### Story 2.2 — JWT Signing and Validation

**What:**
Create `internal/auth/jwt.go` with JWT generation and parsing.

**JWT Claims struct — define with `jti` from the start:**
```go
// Claims holds the payload of a Plomvix JWT token.
// JTI (JWT ID) is a UUID v4 generated per token, used for blacklist lookup on logout.
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     Role   `json:"role"`
    JTI      string `json:"jti"` // UUID v4 — unique per token, used for blacklist
    jwt.RegisteredClaims
}
```

**Public API:**
```go
// GenerateToken creates a signed JWT for the given user.
// Expiry is set from cfg.Auth.JWTExpirySeconds.
// Uses cfg.Auth.JWTSecret as the signing key.
// Algorithm: HS256 — hardcoded, not configurable.
// Populates: IssuedAt, ExpiresAt, UserID, Username, Role, JTI (new UUID v4 per call).
func GenerateToken(user *User, cfg *config.Config) (string, error)

// ParseToken validates the token string and returns the Claims.
// Returns error if the token is expired, malformed, or has an invalid signature.
func ParseToken(tokenString string, cfg *config.Config) (*Claims, error)
```

**Acceptance Criteria:**
- Algorithm is `HS256` — hardcoded, not configurable
- `GenerateToken` sets `IssuedAt`, `ExpiresAt`, `UserID`, `Username`, `Role`, `JTI`
- `JTI` is a fresh UUID v4 on every call to `GenerateToken`
- `ParseToken` returns a typed error distinguishing expired vs invalid signature
- `go build ./internal/auth/` compiles with no errors

---

### Story 2.3 — Token Blacklist (Logout Support)

**What:**
Create `internal/auth/blacklist.go` — an in-memory token blacklist
that enables immediate logout before JWT expiry.

**Struct — include `done` channel explicitly:**
```go
// Blacklist is a thread-safe in-memory store of invalidated JWT token IDs.
// Tokens are identified by their JTI claim (UUID v4).
// Entries are automatically pruned when the token's expiry time passes.
type Blacklist struct {
    mu      sync.RWMutex
    entries map[string]time.Time // jti → expiry time
    done    chan struct{}         // signals background pruning goroutine to stop
}

// NewBlacklist creates and returns a new Blacklist.
// Starts a background goroutine that prunes expired entries every 5 minutes.
func NewBlacklist() *Blacklist

// Add adds a token's JTI to the blacklist until its expiry time.
// Safe to call from multiple goroutines.
func (b *Blacklist) Add(jti string, expiry time.Time)

// IsBlacklisted returns true if the given JTI is currently blacklisted.
// Safe to call from multiple goroutines.
func (b *Blacklist) IsBlacklisted(jti string) bool

// Stop signals the background pruning goroutine to exit.
// Call during graceful shutdown.
func (b *Blacklist) Stop()
```

**Blacklist pruning:**
- Background goroutine runs every 5 minutes
- Removes entries where `expiry.Before(time.Now())`
- `Stop()` closes the `done` channel, goroutine exits on next tick

**Acceptance Criteria:**
- `IsBlacklisted` is goroutine safe — uses `sync.RWMutex`
- `Add` is goroutine safe
- Pruning runs in background and does not block requests
- `Stop()` is called during graceful shutdown
- Memory does not grow unboundedly — pruning removes expired entries

---

&nbsp;

## Feature 3 — API Key Authentication

> The machine authentication flow. Services, scripts, and agents
> authenticate using a long-lived API key instead of a JWT.

---

### Story 3.1 — API Key Generation and Verification

**What:**
Create `internal/auth/apikey.go` with API key generation and verification.

```go
// GenerateAPIKey generates a cryptographically random API key.
// Length in bytes comes from cfg.Auth.APIKeyLength (default: 32 bytes → ~43 base64 chars).
// Returns the raw plaintext key (shown to user once) and its bcrypt hash (stored).
// Uses crypto/rand — NOT math/rand.
func GenerateAPIKey(cfg *config.Config) (plaintext string, hash string, err error)

// CheckAPIKey compares a plaintext API key against a bcrypt hash.
// Returns nil if they match, non-nil error otherwise.
func CheckAPIKey(plaintext, hash string) error

// FindUserByAPIKey searches all users and returns the one whose API key matches.
// Returns ErrUserNotFound if no user matches.
//
// PERFORMANCE NOTE: This performs a bcrypt comparison for each user in the store.
// At bcrypt cost 12, each comparison takes ~250ms.
// This is acceptable for up to ~20 users (worst case ~5 seconds).
// Sprint 4 must revisit this with an indexed lookup before user counts grow.
func FindUserByAPIKey(store *Store, plaintext string) (*User, error)
```

**API key format:**
- Raw random bytes from `crypto/rand`, encoded as base64 URL-safe string (no padding)
- Length: `cfg.Auth.APIKeyLength` bytes before encoding (default 32 bytes = ~43 chars)
- Stored as bcrypt hash in `User.APIKeyHash`
- Plaintext returned ONCE on creation — never stored or logged

**Acceptance Criteria:**
- `crypto/rand` is used — NOT `math/rand`
- Plaintext key is never logged or stored in plaintext
- `go build ./internal/auth/` compiles with no errors

---

&nbsp;

## Feature 4 — Auth Middleware

> The gatekeeper. Every request to a protected endpoint passes through this
> middleware. It resolves the caller's identity and attaches it to the request
> context. Handlers never do their own auth checking.

---

### Story 4.1 — Auth Context

**What:**
Create `internal/auth/context.go` defining how authenticated user identity
is stored in and retrieved from the request context.

```go
// contextKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with context keys from other packages.
type contextKey string

const userContextKey contextKey = "plomvix_authenticated_user"

// WithUser returns a new context with the authenticated user attached.
func WithUser(ctx context.Context, user *User) context.Context

// UserFromContext retrieves the authenticated user from the context.
// Returns nil if no user is present (unauthenticated request). Does not panic.
func UserFromContext(ctx context.Context) *User

// RequireUser retrieves the authenticated user from the context.
// Panics with a clear message if called on a route not protected by Middleware.
// Only call this inside handlers that are guaranteed to have auth middleware applied.
func RequireUser(ctx context.Context) *User
```

**Acceptance Criteria:**
- `contextKey` is unexported — prevents key collisions with other packages
- `UserFromContext` returns nil gracefully — does not panic
- `RequireUser` panics with message: `"plomvix: RequireUser called on unprotected route — add auth.Middleware"`

---

### Story 4.2 — Auth Middleware Implementation

**What:**
Create `internal/auth/middleware.go` implementing the authentication middleware.

**Middleware signature:**
```go
// Middleware returns an HTTP middleware that authenticates every request.
// Tries X-API-Key header first, then Authorization: Bearer JWT.
// Attaches the resolved User to the request context on success.
// Returns 401 JSON error if authentication fails.
func Middleware(store *Store, blacklist *Blacklist, cfg *config.Config) func(http.Handler) http.Handler
```

**Authentication flow — implement in this EXACT order:**
```
1. Check for X-API-Key header
   ├── Present → call FindUserByAPIKey(store, apiKeyValue)
   │   ├── Match found → attach user to context → call next handler → DONE
   │   └── No match → return 401 immediately
   │             IMPORTANT: do NOT fall through to JWT check.
   │             If a client sends X-API-Key, that is their explicit auth choice.
   │             A failed API key is always a 401 — never a JWT fallback.
   └── Not present → continue to step 2

2. Check for Authorization header with "Bearer " prefix
   ├── Present → extract token string → call ParseToken(token, cfg)
   │   ├── Parse error (expired, malformed, bad signature) → return 401
   │   ├── Token JTI is in blacklist → return 401 {message: "token has been revoked"}
   │   ├── Token valid → call store.GetUserByID(claims.UserID)
   │   │   ├── ErrUserNotFound → return 401
   │   │   │   (user was deleted after token was issued — token is now orphaned)
   │   │   └── User found → attach user to context → call next handler → DONE
   │   └── (no other cases)
   └── Not present → continue to step 3

3. Neither X-API-Key nor Authorization header present
   └── return 401 {code: "UNAUTHORIZED", message: "authentication required"}
```

**Acceptance Criteria:**
- API key check happens BEFORE JWT check — machine path has priority
- A present but invalid API key returns `401` immediately — JWT is never checked
- A valid JWT for a deleted user returns `401` — not `500`
- All 401 responses use `utils.Unauthorized()`
- No auth logic leaks into individual handlers
- Middleware is NOT in the global middleware chain — applied per route group only

---

### Story 4.3 — Admin-Only Middleware

**What:**
Add `RequireAdmin` middleware to `internal/auth/middleware.go`.

```go
// RequireAdmin returns an HTTP middleware that enforces admin role.
// MUST be used AFTER Middleware in the chain — assumes a User is already in context.
// Returns 403 Forbidden if the authenticated user does not have RoleAdmin.
// Returns 500 Internal Error (panic) if called without Middleware in chain —
// this is a programming error and must be loud, not silently swallowed.
func RequireAdmin() func(http.Handler) http.Handler
```

**Acceptance Criteria:**
- Returns `403` via `utils.Forbidden()` for non-admin users
- Panics with clear message if no user found in context (programming error)
- Applied to all `/admin/*` route groups

---

&nbsp;

## Feature 5 — Auth API Endpoints

> The login, logout, and token refresh endpoints.
> These are the only endpoints in Sprint 2 publicly accessible without prior authentication.

---

### Story 5.1 — Auth Handler

**What:**
Create `internal/auth/handler.go` implementing the three auth endpoints.

**Handler struct:**
```go
type Handler struct {
    store     *Store
    blacklist *Blacklist
    cfg       *config.Config
}

func NewHandler(store *Store, blacklist *Blacklist, cfg *config.Config) *Handler
```

---

**`POST /auth/login`**

- **Auth required:** No — public endpoint
- **Content-Type:** `application/json`

Request body:
```json
{
  "username": "admin",
  "password": "changeme"
}
```

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "token": "<jwt_string>",
    "expires_in": 3600,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "username": "admin",
      "role": "admin",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Validation failure — HTTP 400:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "username and password are required"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Auth failure — HTTP 401:
```json
{
  "status": "error",
  "error": {
    "code": "UNAUTHORIZED",
    "message": "invalid username or password"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Decode JSON body → if malformed return 400
2. Validate username and password fields are not empty → if empty return 400
3. Call store.GetUserByUsername(username)
   └── ErrUserNotFound → return 401 with SAME message as wrong password
4. Call CheckPassword(password, user.PasswordHash)
   └── Error → return 401 with SAME message as user not found
5. Call GenerateToken(user, cfg)
6. Return 200 with token, expires_in (from cfg), and user.ToResponse()
```

**Security:** Return identical 401 message for both "user not found" and "wrong password".
Distinguishing between them enables username enumeration attacks.

---

**`POST /auth/logout`**

- **Auth required:** Yes — JWT or API key
- **Content-Type:** none (no request body)

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "message": "logged out successfully"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Get authenticated user from context via RequireUser(r.Context())
2. Check if Authorization header has "Bearer " prefix
   ├── Present → extract token string → call ParseToken(token, cfg)
   │             → call blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
   └── Absent  → user authenticated via API key — no JWT to blacklist, skip to step 3
3. Return 200
```

**Note:** If the user authenticated via API key (no Bearer header present), logout still
returns 200 but has no token to blacklist. API key auth is stateless — revoking an API
key is done via the API key management endpoint, not logout.

---

**`POST /auth/refresh`**

- **Auth required:** Yes — JWT only (API key users do not need token refresh)
- **Content-Type:** none (no request body)

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "token": "<new_jwt_string>",
    "expires_in": 3600
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Get authenticated user from context via RequireUser(r.Context())
2. Call store.GetUserByID(user.ID) to get fresh user data
3. Extract current Bearer token from Authorization header
4. Call ParseToken(currentToken, cfg) to get current JTI and expiry
5. Call blacklist.Add(currentJTI, currentExpiry) — invalidate old token
6. Call GenerateToken(freshUser, cfg) — issue new token
7. Return 200 with new token and expires_in
```

**Acceptance Criteria:**
- Login returns 400 for missing/empty fields, 401 for wrong credentials
- Login never reveals whether username or password was wrong
- Logout immediately invalidates the JWT via blacklist
- Refresh atomically invalidates old token and issues new one
- All three endpoints use `utils` response helpers — no raw JSON writes
- No endpoint logs passwords, tokens, or hashes

---

&nbsp;

## Feature 6 — User Management API

> Admin-only endpoints for managing Plomvix user accounts.
> All endpoints require authentication AND admin role.

---

### Story 6.1 — User Handler

**What:**
Create `internal/auth/user_handler.go` implementing user management endpoints.

**Handler struct:**
```go
type UserHandler struct {
    store *Store
    cfg   *config.Config
}

func NewUserHandler(store *Store, cfg *config.Config) *UserHandler
```

---

**`POST /admin/users`**

- **Auth required:** Yes — admin only
- **Content-Type:** `application/json`

Request body:
```json
{
  "username": "newuser",
  "password": "securepassword"
}
```

**Validation rules:**

| Field | Rule |
|---|---|
| `username` | Not empty, 3–64 characters, alphanumeric + underscore + hyphen only |
| `password` | Not empty, minimum 8 characters |

Success response — HTTP 201:
```json
{
  "status": "ok",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "newuser",
    "role": "admin",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Validation failure — HTTP 400:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "validation failed",
    "details": [
      "username must be between 3 and 64 characters",
      "password must be at least 8 characters"
    ]
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Conflict — HTTP 409:
```json
{
  "status": "error",
  "error": {
    "code": "CONFLICT",
    "message": "username already exists"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Decode and validate request body → return 400 with all validation errors
2. Call store.UserExists(username) → if true return 409
3. Hash password with HashPassword()
4. Create User with new UUID v4, RoleAdmin, hashed password, now timestamps
5. Call store.CreateUser()
6. Return 201 with user.ToResponse()
```

---

**`GET /admin/users`**

- **Auth required:** Yes — admin only

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "users": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "username": "admin",
        "role": "admin",
        "created_at": "2024-01-15T10:30:00Z",
        "updated_at": "2024-01-15T10:30:00Z"
      }
    ],
    "count": 1
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Call store.ListUsers()
2. Convert each User to UserResponse via ToResponse()
3. Return 200 with users array and count
```

---

**`GET /admin/users/{id}`**

- **Auth required:** Yes — admin only

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "admin",
    "role": "admin",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Not found — HTTP 404:
```json
{
  "status": "error",
  "error": {
    "code": "NOT_FOUND",
    "message": "user not found"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Extract {id} from URL via chi.URLParam(r, "id")
2. Call store.GetUserByID(id) → if ErrUserNotFound return 404
3. Return 200 with user.ToResponse()
```

---

**`PATCH /admin/users/{id}`**

- **Auth required:** Yes — admin only
- **Content-Type:** `application/json`

Request body (all fields optional — only send fields to update):
```json
{
  "username": "newname",
  "password": "newpassword"
}
```

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "newname",
    "role": "admin",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:31:00Z"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Validation failure — HTTP 400:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "validation failed",
    "details": ["password must be at least 8 characters"]
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Conflict — HTTP 409 (if new username already taken by another user):
```json
{
  "status": "error",
  "error": {
    "code": "CONFLICT",
    "message": "username already exists"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Logic:**
```
1. Extract {id} from URL
2. Call store.GetUserByID(id) → if ErrUserNotFound return 404
3. Decode request body — use pointer fields to detect omitted vs empty
4. If username provided:
   - Validate format (3–64 chars, alphanumeric + _ + -)
   - If different from current → check store.UserExists(newUsername) → if true return 409
   - Update user.Username
5. If password provided:
   - Validate minimum 8 characters
   - Hash with HashPassword()
   - Update user.PasswordHash
6. Update user.UpdatedAt = time.Now()
7. Call store.UpdateUser()
8. Return 200 with user.ToResponse()
```

---

**`DELETE /admin/users/{id}`**

- **Auth required:** Yes — admin only

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "message": "user deleted"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Self-deletion attempt — HTTP 400:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "cannot delete your own account"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Last admin deletion attempt — HTTP 400:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "cannot delete the last admin user"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Not found — HTTP 404.

**Logic:**
```
1. Extract {id} from URL
2. Get authenticated user from context via RequireUser
3. If id == authenticatedUser.ID → return 400 "cannot delete your own account"
4. Call store.GetUserByID(id) → if ErrUserNotFound return 404
5. Call store.ListUsers() → if count == 1 → return 400 "cannot delete the last admin user"
6. Call store.DeleteUser(id)
7. Return 200
```

**Acceptance Criteria:**
- All responses use `user.ToResponse()` — never raw `User`
- No handler accepts `role` as input — all created users are admin in Sprint 2
- Delete guards prevent locking out of the system
- All validation errors collected and returned together in `details` array
- `CONFLICT` error code added to `pkg/utils/response.go` constants

---

&nbsp;

## Feature 7 — API Key Management API

> Admin-only endpoints for generating and revoking API keys.
> API keys are the machine-to-machine auth method for Plomvix.

---

### Story 7.1 — API Key Handler

**What:**
Create `internal/auth/apikey_handler.go` implementing API key management endpoints.

**Handler struct:**
```go
type APIKeyHandler struct {
    store *Store
    cfg   *config.Config
}

func NewAPIKeyHandler(store *Store, cfg *config.Config) *APIKeyHandler
```

---

**`POST /admin/users/{id}/apikey`**

- **Auth required:** Yes — admin only

Generates a new API key for the specified user.
Replaces any existing API key — a user has exactly one API key at a time.

Success response — HTTP 201:
```json
{
  "status": "ok",
  "data": {
    "api_key": "dGhpcyBpcyBhIHRlc3Qga2V5IGZvciBleGFtcGxl",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "message": "Store this key securely. It will not be shown again."
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Not found — HTTP 404 if user does not exist.

**Logic:**
```
1. Extract {id} from URL
2. Call store.GetUserByID(id) → if ErrUserNotFound return 404
3. Call GenerateAPIKey(cfg) → get plaintext and hash
4. Update user.APIKeyHash = hash
5. Update user.UpdatedAt = time.Now()
6. Call store.UpdateUser()
7. Return 201 with plaintext key, user_id, and warning message
```

**Security:** Plaintext key is returned ONCE in this response and never stored.
If the key is lost, generate a new one — there is no recovery.

---

**`DELETE /admin/users/{id}/apikey`**

- **Auth required:** Yes — admin only

Revokes the API key for the specified user.

Success response — HTTP 200 (also returned if user has no key — idempotent):
```json
{
  "status": "ok",
  "data": {
    "message": "API key revoked"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Not found — HTTP 404 if the USER does not exist (distinct from having no key).

**Logic:**
```
1. Extract {id} from URL
2. Call store.GetUserByID(id) → if ErrUserNotFound return 404
3. Set user.APIKeyHash = "" (clear the key regardless of whether one existed)
4. Update user.UpdatedAt = time.Now()
5. Call store.UpdateUser()
6. Return 200
```

**Idempotency note:** Revoking a key when the user has NO key returns 200 — not an error.
Revoking when the USER does not exist returns 404. These are different conditions.

---

**`GET /admin/users/{id}/apikey/status`**

- **Auth required:** Yes — admin only

Returns whether the user currently has an API key configured.
Never reveals the key or its hash.

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "has_api_key": true,
    "user_id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Not found — HTTP 404 if user does not exist.

**Logic:**
```
1. Extract {id} from URL
2. Call store.GetUserByID(id) → if ErrUserNotFound return 404
3. Return 200 with has_api_key: (user.APIKeyHash != "")
```

**Acceptance Criteria:**
- Plaintext API key returned exactly once — on creation only
- No endpoint returns or logs the stored hash
- Revoking a key when user has no key returns 200 (idempotent)
- Revoking when user does not exist returns 404
- `has_api_key` derived purely from `APIKeyHash != ""`

---

&nbsp;

## Feature 8 — Route Registration and Server Integration

> Wiring all Sprint 2 handlers into the HTTP server and updating
> main.go to initialize the auth system on startup.

---

### Story 8.1 — Register Auth Routes in Server

**What:**
Update `internal/server/server.go` to register Sprint 2 routes with correct middleware groups.

**Updated `server.New()` signature:**
```go
// New signature — accepts auth dependencies for route registration.
func New(cfg *config.Config, version string, store *auth.Store, blacklist *auth.Blacklist) *Server
```

**Full route table:**

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | None | Health check (Sprint 1) |
| POST | `/auth/login` | None | Login — returns JWT |
| POST | `/auth/logout` | JWT or API key | Invalidate current JWT |
| POST | `/auth/refresh` | JWT or API key | Issue new JWT, invalidate old |
| POST | `/admin/users` | Admin | Create user |
| GET | `/admin/users` | Admin | List all users |
| GET | `/admin/users/{id}` | Admin | Get single user |
| PATCH | `/admin/users/{id}` | Admin | Update user |
| DELETE | `/admin/users/{id}` | Admin | Delete user |
| POST | `/admin/users/{id}/apikey` | Admin | Generate API key |
| DELETE | `/admin/users/{id}/apikey` | Admin | Revoke API key |
| GET | `/admin/users/{id}/apikey/status` | Admin | Check API key status |

**Chi route grouping — use `{id}` syntax (chi v5 standard):**
```go
// Public — no auth
r.Get("/health", s.handleHealth)
r.Post("/auth/login", authHandler.Login)

// Protected — auth middleware required
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware(store, blacklist, cfg))
    r.Post("/auth/logout", authHandler.Logout)
    r.Post("/auth/refresh", authHandler.Refresh)
})

// Admin only — auth + admin role required
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware(store, blacklist, cfg))
    r.Use(auth.RequireAdmin())
    r.Post("/admin/users", userHandler.Create)
    r.Get("/admin/users", userHandler.List)
    r.Get("/admin/users/{id}", userHandler.Get)
    r.Patch("/admin/users/{id}", userHandler.Update)
    r.Delete("/admin/users/{id}", userHandler.Delete)
    r.Post("/admin/users/{id}/apikey", apiKeyHandler.Generate)
    r.Delete("/admin/users/{id}/apikey", apiKeyHandler.Revoke)
    r.Get("/admin/users/{id}/apikey/status", apiKeyHandler.Status)
})
```

**Acceptance Criteria:**
- All route paths use `{id}` not `:id` — chi v5 syntax throughout
- `/health` requires NO authentication
- `/auth/login` requires NO authentication
- Auth middleware is NOT in the global middleware chain — applied per group only
- `go build ./internal/server/` compiles with no errors

---

### Story 8.2 — Update main.go and .gitignore

**What:**
Update `cmd/plomvix/main.go` to initialize the auth system, and update `.gitignore`
to cover the new `data/system/` directory.

**`.gitignore` additions:**
```gitignore
# System data (BoltDB auth database)
data/system/*
!data/system/.gitkeep
```

Also create `data/system/.gitkeep` so the directory is tracked by git.

**Updated boot sequence — replaces Sprint 1 boot sequence from step 7 onward:**

```
(steps 1–6 from Sprint 1 unchanged: flags, banner, config, logger, startup log)

7.  Bootstrap data directories — updated function includes system dir:
      bootstrapDataDirs(cfg) now creates ALL of these:
        {data_dir}/wal
        {data_dir}/hot
        {data_dir}/cold/logs
        {data_dir}/cold/metrics
        {data_dir}/cold/json
        {data_dir}/cold/kv
        {data_dir}/system          ← NEW in Sprint 2
      On error: log error and os.Exit(1)

8.  Open BoltDB user store:
      store, err := auth.NewStore(
          filepath.Join(cfg.Storage.DataDir, "system", "auth.db"))
      if err != nil {
          logger.Error("failed to open user store", zap.Error(err))
          os.Exit(1)
      }
      // Note: defer registered AFTER all os.Exit(1) paths that need explicit cleanup

9.  Bootstrap default admin user:
      if err := auth.BootstrapAdminUser(store, cfg); err != nil {
          store.Close() // explicit close — os.Exit bypasses defers
          logger.Error("failed to bootstrap admin user", zap.Error(err))
          os.Exit(1)
      }
      defer store.Close() // safe to defer now — no more os.Exit after this point

10. Create token blacklist:
      blacklist := auth.NewBlacklist()
      defer blacklist.Stop()

11. Create server with auth dependencies:
      srv := server.New(cfg, Version, store, blacklist)

(remaining steps from Sprint 1 continue unchanged: signal listener, server start,
ready log, block, shutdown, final log, Sync, return)
```

**`bootstrapDataDirs` update — add system dir to the dirs slice:**
```go
filepath.Join(cfg.Storage.DataDir, "system"),
```
This is the only change needed to the existing `bootstrapDataDirs` function from Sprint 1.

**Acceptance Criteria:**
- `data/system/*` added to `.gitignore`
- `data/system/.gitkeep` created
- Auth store opened before server starts
- Bootstrap runs before server starts
- `store.Close()` deferred AFTER all `os.Exit(1)` error paths that would skip it
- Explicit `store.Close()` called before any `os.Exit(1)` that follows store creation
- `defer blacklist.Stop()` registered before server starts
- `go build ./cmd/plomvix/` compiles with no errors

---

&nbsp;

## Feature 9 — Tests

> Every auth component has unit tests.
> Integration tests cover the full HTTP-level auth flow end to end.

---

### Story 9.1 — Unit Tests for Auth Utilities

**What:**
Create `internal/auth/auth_test.go` with unit tests for all auth utility functions.

**Tests to implement:**

```
TestHashPassword:
  - HashPassword("password") returns non-empty string
  - HashPassword("password") returns different hash each call (bcrypt salt)
  - Result starts with "$2a$12$" (bcrypt prefix at cost 12)

TestCheckPassword:
  - CheckPassword("password", HashPassword("password")) returns nil
  - CheckPassword("wrong", HashPassword("password")) returns non-nil error

TestGenerateToken:
  - GenerateToken returns non-empty string
  - Token has exactly 3 dot-separated parts (JWT format)
  - Two calls for the same user produce different tokens (different JTI)
  - Token contains correct UserID, Username, Role in claims

TestParseToken:
  - ParseToken on valid token returns correct claims (UserID, Username, Role, JTI)
  - ParseToken on expired token returns non-nil error
  - ParseToken on tampered token (modified signature) returns non-nil error
  - ParseToken on empty string returns non-nil error
  - ParseToken with wrong secret returns non-nil error

TestBlacklist:
  - IsBlacklisted returns false for unknown JTI
  - Add(jti, futureExpiry) then IsBlacklisted(jti) returns true
  - Add(jti, pastExpiry) then IsBlacklisted(jti) returns false
    (expired entries are not blacklisted — they should have been pruned)
  - Concurrent Add and IsBlacklisted calls do not race (use -race flag)

TestGenerateAPIKey:
  - Returns non-empty plaintext and non-empty hash
  - Plaintext and hash are different strings
  - CheckAPIKey(plaintext, hash) returns nil
  - CheckAPIKey("wrong", hash) returns non-nil error
  - Two consecutive calls return different plaintexts
```

**Config construction for tests — do NOT call config.Load():**
```go
func testConfig() *config.Config {
    return &config.Config{
        Auth: config.AuthConfig{
            JWTSecret:        "test-secret-key",
            JWTExpirySeconds: 3600,
            APIKeyLength:     32,
        },
    }
}
```

**Acceptance Criteria:**
- All tests pass with `go test -race ./internal/auth/`
- No test calls `config.Load()`
- No test writes to disk outside of `t.TempDir()`

---

### Story 9.2 — User Store Tests

**What:**
Create `internal/auth/store_test.go` with tests for all store operations.

**Test helper — fresh store per test:**
```go
func newTestStore(t *testing.T) *Store {
    t.Helper()
    store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
    if err != nil {
        t.Fatalf("failed to create test store: %v", err)
    }
    t.Cleanup(func() { _ = store.Close() })
    return store
}
```

**Tests to implement:**

```
TestStoreCreateAndGet:
  - Create user → GetUserByID returns user with same fields
  - Create user → GetUserByUsername returns user with same fields
  - Create two users with same username → second returns ErrUserAlreadyExists
  - GetUserByID with unknown ID → returns ErrUserNotFound
  - GetUserByUsername with unknown username → returns ErrUserNotFound

TestStoreUpdate:
  - Create user → Update username → GetUserByID returns updated username
  - Update user that does not exist → returns ErrUserNotFound
  - UpdatedAt changes after update

TestStoreDelete:
  - Create user → DeleteUser → GetUserByID returns ErrUserNotFound
  - DeleteUser with unknown ID → returns ErrUserNotFound

TestStoreList:
  - Empty store → ListUsers returns empty slice (not nil, length 0)
  - Create 3 users with different CreatedAt → ListUsers returns all 3 in ascending CreatedAt order

TestUserExists:
  - UserExists for existing username → true, nil
  - UserExists for non-existent username → false, nil
```

**Acceptance Criteria:**
- All tests pass with `go test -race ./internal/auth/`
- Each test uses `newTestStore(t)` — no shared state between tests
- `t.TempDir()` used for all temp directories — auto-cleaned by Go test framework

---

### Story 9.3 — Integration Tests (HTTP Level)

**What:**
Create `internal/auth/integration_test.go` testing the full auth flow
at the HTTP level — not unit testing individual functions.

**Test setup — spin up a real chi router with all auth middleware:**
```go
func newTestRouter(t *testing.T) (http.Handler, *Store, *Blacklist) {
    t.Helper()
    store := newTestStore(t)
    cfg := testConfig()
    blacklist := NewBlacklist()
    t.Cleanup(func() { blacklist.Stop() })
    // bootstrap admin
    _ = BootstrapAdminUser(store, cfg)
    // wire router
    r := chi.NewRouter()
    authHandler := NewHandler(store, blacklist, cfg)
    userHandler := NewUserHandler(store, cfg)
    r.Post("/auth/login", authHandler.Login)
    r.Group(func(r chi.Router) {
        r.Use(Middleware(store, blacklist, cfg))
        r.Post("/auth/logout", authHandler.Logout)
        r.Post("/auth/refresh", authHandler.Refresh)
    })
    r.Group(func(r chi.Router) {
        r.Use(Middleware(store, blacklist, cfg))
        r.Use(RequireAdmin())
        r.Post("/admin/users", userHandler.Create)
        r.Get("/admin/users", userHandler.List)
    })
    return r, store, blacklist
}
```

**Integration tests to implement:**

```
TestLoginSuccess:
  - POST /auth/login with valid credentials → 200, token in response

TestLoginWrongPassword:
  - POST /auth/login with wrong password → 401
  - Response message same as wrong username (no enumeration)

TestLoginWrongUsername:
  - POST /auth/login with unknown username → 401
  - Response message same as wrong password

TestProtectedRouteWithJWT:
  - Login → use returned token on POST /admin/users → 201

TestProtectedRouteWithAPIKey:
  - Get admin's API key from store directly
  - Use X-API-Key header on GET /admin/users → 200

TestProtectedRouteNoAuth:
  - GET /admin/users with no auth headers → 401

TestLogoutInvalidatesToken:
  - Login → logout with token → use same token → 401

TestRefreshIssuesNewToken:
  - Login → get token A
  - POST /auth/refresh with token A → get token B
  - GET /admin/users with token A → 401 (old token now blacklisted)
  - GET /admin/users with token B → 200 (new token works)

TestInvalidAPIKey:
  - X-API-Key: "wrongkey" on protected route → 401
  - Does NOT fall through to JWT check
```

**Acceptance Criteria:**
- All integration tests pass with `go test -race ./internal/auth/`
- Tests use `httptest.NewRecorder` and `httptest.NewRequest` — no real network
- No test shares state — each test creates its own router and store

---

&nbsp;

## Feature 10 — API Documentation

> Every endpoint defined in Sprint 2 is documented for developers.
> This is the source of truth for Sprint 9's Swagger generation.

---

### Story 10.1 — Handler Godoc Comments

**What:**
Every handler method in Sprint 2 must have a godoc comment block that documents:
- HTTP method and path
- Authentication requirement
- Request body (if any)
- Success response code and shape
- All possible error codes and when they occur

**Format for every handler method:**
```go
// Login authenticates a user with username and password.
//
// POST /auth/login
// Auth: none (public endpoint)
//
// Request body: {"username": string, "password": string}
//
// Responses:
//   200 OK           — login successful, returns JWT token and user info
//   400 Bad Request  — VALIDATION_FAILED: username or password field is empty
//   401 Unauthorized — UNAUTHORIZED: invalid username or password
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
```

**Apply this comment format to ALL handler methods:**
- `Handler.Login`
- `Handler.Logout`
- `Handler.Refresh`
- `UserHandler.Create`
- `UserHandler.List`
- `UserHandler.Get`
- `UserHandler.Update`
- `UserHandler.Delete`
- `APIKeyHandler.Generate`
- `APIKeyHandler.Revoke`
- `APIKeyHandler.Status`

**Acceptance Criteria:**
- Every handler method has a godoc comment in the format above
- `go doc ./internal/auth/` shows documentation for all handlers
- Comments are accurate — match actual behaviour exactly

---

### Story 10.2 — API Reference Document

**What:**
Create `docs/api/auth.md` — a human-readable API reference for all Sprint 2 endpoints.
This file is the developer-facing documentation until Swagger is added in Sprint 9.

**File:** `docs/api/auth.md`

**Structure:**
```
# Plomvix Auth API Reference

## Authentication

### Methods
- JWT Bearer token via Authorization header
- API Key via X-API-Key header

### How to authenticate
[explain both methods with examples]

---

## Endpoints

### POST /auth/login
[method, auth, request, responses, example curl]

### POST /auth/logout
[method, auth, request, responses, example curl]

### POST /auth/refresh
[method, auth, request, responses, example curl]

### POST /admin/users
[method, auth, request, validation rules, responses, example curl]

### GET /admin/users
[method, auth, responses, example curl]

### GET /admin/users/{id}
[method, auth, responses, example curl]

### PATCH /admin/users/{id}
[method, auth, request, responses, example curl]

### DELETE /admin/users/{id}
[method, auth, rules, responses, example curl]

### POST /admin/users/{id}/apikey
[method, auth, responses, security note, example curl]

### DELETE /admin/users/{id}/apikey
[method, auth, idempotency note, responses, example curl]

### GET /admin/users/{id}/apikey/status
[method, auth, responses, example curl]

---

## Error Codes

| Code | HTTP Status | Description |
|---|---|---|
| UNAUTHORIZED | 401 | Missing, invalid, expired, or revoked credentials |
| FORBIDDEN | 403 | Authenticated but insufficient permissions |
| VALIDATION_FAILED | 400 | Request body missing or invalid fields |
| NOT_FOUND | 404 | Requested resource does not exist |
| CONFLICT | 409 | Resource already exists (e.g. duplicate username) |
| INTERNAL_ERROR | 500 | Unexpected server error |

---

## Standard Response Envelope

[show success and error envelope structures]
```

**Each endpoint entry must include:**
- Method + path
- Auth requirement
- Request body (if applicable) with field types and validation rules
- Success response with full JSON example
- All error responses with full JSON examples
- Example `curl` command

**Acceptance Criteria:**
- `docs/api/auth.md` exists and renders correctly on GitHub
- Every Sprint 2 endpoint is documented
- Every curl example is tested and produces the documented response
- Error codes table matches constants in `pkg/utils/response.go`
- `docs/` directory is created and tracked in git (not gitignored)

---

&nbsp;

## Sprint 2 — Definition of Done

Sprint 2 is complete when **all of the following are true:**

- [ ] `go mod tidy` runs with zero errors
- [ ] `go build ./...` compiles with zero errors and zero warnings
- [ ] `make test` passes with zero failures and race detector enabled
- [ ] `make vet` passes with zero issues
- [ ] On fresh boot, default admin user is created automatically
- [ ] On subsequent boots, no duplicate admin is created
- [ ] `POST /auth/login` with correct credentials returns a valid JWT
- [ ] `POST /auth/login` with wrong credentials returns `401` — identical message for wrong user vs wrong password
- [ ] `POST /auth/login` with missing fields returns `400`
- [ ] JWT accepted on protected endpoints via `Authorization: Bearer <token>`
- [ ] API key accepted on protected endpoints via `X-API-Key` header
- [ ] Invalid API key returns `401` immediately — does NOT fall through to JWT check
- [ ] `POST /auth/logout` invalidates JWT immediately — same token rejected on next request
- [ ] `POST /auth/refresh` issues new token and invalidates old one
- [ ] `GET /health` works without any authentication
- [ ] All `/admin/*` endpoints return `401` without authentication
- [ ] All `/admin/*` endpoints return `403` for non-admin users
- [ ] `POST /admin/users` creates a user with bcrypt-hashed password
- [ ] `GET /admin/users` lists all users without exposing password hashes or API key hashes
- [ ] `GET /admin/users/{id}` returns correct full JSON envelope
- [ ] `PATCH /admin/users/{id}` updates username and/or password and returns updated record
- [ ] `DELETE /admin/users/{id}` prevents self-deletion and last-admin deletion
- [ ] `POST /admin/users/{id}/apikey` returns plaintext key once and stores only hash
- [ ] `DELETE /admin/users/{id}/apikey` revokes key — old key returns `401` on next request
- [ ] `DELETE /admin/users/{id}/apikey` returns `200` when user has no key (idempotent)
- [ ] `DELETE /admin/users/{id}/apikey` returns `404` when user does not exist
- [ ] No password, hash, or API key value appears in any log line
- [ ] `PasswordHash` and `APIKeyHash` fields never appear in any API response
- [ ] BoltDB file created at `{data_dir}/system/auth.db` on first boot
- [ ] `data/system/*` added to `.gitignore`
- [ ] `store.Close()` explicitly called before any `os.Exit(1)` after store creation
- [ ] `defer store.Close()` and `defer blacklist.Stop()` called during graceful shutdown
- [ ] All route paths use `{id}` (chi v5) not `:id`
- [ ] All unit tests in `internal/auth/` pass with race detector
- [ ] All integration tests pass — login, protected routes, logout, refresh verified at HTTP level
- [ ] `docs/api/auth.md` exists and documents all 11 endpoints with curl examples
- [ ] Every handler method has a godoc comment documenting method, auth, request, responses

---

&nbsp;

## Sprint 2 — Story Summary

| Feature | Story | Description |
|---|---|---|
| 1 — User Storage | 1.1 | Add BoltDB dependency |
| 1 — User Storage | 1.2 | Add bcrypt and JWT dependencies |
| 1 — User Storage | 1.3 | Define User model |
| 1 — User Storage | 1.4 | Implement User store |
| 1 — User Storage | 1.5 | Bootstrap default admin user |
| 2 — Password Auth + JWT | 2.1 | Password hashing utilities |
| 2 — Password Auth + JWT | 2.2 | JWT signing and validation |
| 2 — Password Auth + JWT | 2.3 | Token blacklist |
| 3 — API Key Auth | 3.1 | API key generation and verification |
| 4 — Auth Middleware | 4.1 | Auth context |
| 4 — Auth Middleware | 4.2 | Auth middleware implementation |
| 4 — Auth Middleware | 4.3 | Admin-only middleware |
| 5 — Auth Endpoints | 5.1 | Login, logout, refresh handlers |
| 6 — User Management | 6.1 | User CRUD handlers |
| 7 — API Key Management | 7.1 | API key generate, revoke, status handlers |
| 8 — Server Integration | 8.1 | Register routes with correct middleware groups |
| 8 — Server Integration | 8.2 | Update main.go and .gitignore |
| 9 — Tests | 9.1 | Unit tests for auth utilities |
| 9 — Tests | 9.2 | User store tests |
| 9 — Tests | 9.3 | Integration tests (HTTP level) |
| 10 — API Documentation | 10.1 | Handler godoc comments |
| 10 — API Documentation | 10.2 | API reference document (docs/api/auth.md) |
| **Total** | **22 stories** | |

---

&nbsp;

*Plomvix — Built in India. Built for the world.*