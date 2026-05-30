# Plomvix — Sprint 2 Code Plan
### For: DeepSeek V4 Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

You are continuing to build **Plomvix** — Sprint 1 is complete.
Sprint 2 adds the full authentication system: JWT, API keys, user management,
and admin-only route protection.

**Sprint 1 files already exist:**
```
cmd/plomvix/main.go
internal/config/config.go
internal/logger/logger.go
internal/server/server.go
pkg/utils/utils.go
pkg/utils/response.go
config.yaml
Makefile
```

**Sprint 2 produces new files in:**
```
internal/auth/          ← all new auth code
docs/api/               ← new API documentation directory
```

**And modifies:**
```
go.mod / go.sum         ← new dependencies
.gitignore              ← new data/system/* entry
cmd/plomvix/main.go     ← auth initialization in boot sequence
internal/server/server.go ← auth routes registered
pkg/utils/response.go   ← CONFLICT error code added
```

---

## TASK 01 — Add all Sprint 2 dependencies

**Action:**
```bash
go get go.etcd.io/bbolt
go get golang.org/x/crypto
go get github.com/golang-jwt/jwt/v5
go mod tidy
```

**Verify:**
- `go.etcd.io/bbolt`, `golang.org/x/crypto`, `github.com/golang-jwt/jwt/v5`
  all appear in `go.mod`
- `go mod tidy` exits with zero errors
- `go build ./...` still compiles with no errors

---

## TASK 02 — Update .gitignore and create data/system directory

**Action — Part A:** Add to `.gitignore` after the existing `data/cold/**` block:
```gitignore
# System data (BoltDB auth database)
data/system/*
!data/system/.gitkeep
```

**Action — Part B:** Create the directory and placeholder:
```bash
mkdir -p data/system
touch data/system/.gitkeep
```

**Verify:**
- `git check-ignore -v data/system/auth.db` shows it is ignored
- `git check-ignore -v data/system/.gitkeep` shows it is NOT ignored
- `ls data/system/.gitkeep` confirms file exists

---

## TASK 03 — Add CONFLICT error code to pkg/utils/response.go

**Action:** Add `CodeConflict` to the existing error code constants in
`pkg/utils/response.go`. Find the existing constants block and add one line:

```go
const (
    CodeValidationFailed  = "VALIDATION_FAILED"
    CodeUnauthorized      = "UNAUTHORIZED"
    CodeForbidden         = "FORBIDDEN"
    CodeNotFound          = "NOT_FOUND"
    CodeInternalError     = "INTERNAL_ERROR"
    CodeHealthCheckFailed = "HEALTH_CHECK_FAILED"
    CodeServiceUnavail    = "SERVICE_UNAVAILABLE"
    CodeConflict          = "CONFLICT"          // ← ADD THIS LINE
)
```

Also add the `Conflict` helper function:
```go
// Conflict writes a 409 JSON error response.
func Conflict(w http.ResponseWriter, r *http.Request, message string) {
    requestID := r.Header.Get("X-Request-ID")
    writeJSON(w, http.StatusConflict, ErrorResponse{
        Status:    "error",
        Error:     ErrorBody{Code: CodeConflict, Message: message},
        RequestID: requestID,
    })
}
```

**Verify:** `go build ./pkg/utils/` compiles with no errors.

---

## TASK 04 — Create internal/auth/model.go

**Action:** Create `internal/auth/model.go`.

**Imports required:**
```go
import "time"
```

**Full file content:**
```go
package auth

import "time"

// Role defines the permission level of a Plomvix user.
// In Sprint 2 all users are Admin. RBAC is deferred to a future version.
type Role string

const (
    RoleAdmin Role = "admin"
)

// User represents a Plomvix user account.
// PasswordHash and APIKeyHash use json:"-" so they are NEVER included
// in JSON output even if a User is accidentally serialized directly.
type User struct {
    ID           string    `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"-"` // bcrypt hash — excluded from all JSON
    Role         Role      `json:"role"`
    APIKeyHash   string    `json:"-"` // bcrypt hash of API key — excluded from all JSON
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// UserResponse is the safe public representation of a User.
// Never contains PasswordHash or APIKeyHash.
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

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 05 — Create internal/auth/store.go

**Action:** Create `internal/auth/store.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "errors"
    "sort"
    "time"

    bolt "go.etcd.io/bbolt"
)
```

**Sentinel errors — define at package level:**
```go
var (
    ErrUserNotFound      = errors.New("user not found")
    ErrUserAlreadyExists = errors.New("username already exists")
)
```

**Bucket name constant:**
```go
var usersBucket = []byte("users")
```

**Store struct:**
```go
type Store struct {
    db *bolt.DB
}
```

**`NewStore(path string) (*Store, error)`:**
- Open BoltDB at `path` with timeout `1 * time.Second`
- Create `usersBucket` inside a write transaction if it does not exist
- Return `&Store{db: db}, nil` on success

**`Close() error`:**
- Call and return `s.db.Close()`

**`CreateUser(u *User) error`:**
- Open a write transaction
- Check if any existing user has the same `Username`
  — scan all values in `usersBucket`, unmarshal each, compare `.Username`
  — if match found, return `ErrUserAlreadyExists`
- Marshal `u` to JSON
- Put `[]byte(u.ID)` → JSON bytes in `usersBucket`
- Return nil on success

**`GetUserByID(id string) (*User, error)`:**
- Open a read transaction
- Get value at key `[]byte(id)` from `usersBucket`
- If nil, return `nil, ErrUserNotFound`
- Unmarshal and return

**`GetUserByUsername(username string) (*User, error)`:**
- Open a read transaction
- Iterate all values in `usersBucket`
- Unmarshal each, compare `.Username`
- Return first match or `nil, ErrUserNotFound`

**`UpdateUser(u *User) error`:**
- Open a write transaction
- Get existing value at `[]byte(u.ID)` — if nil return `ErrUserNotFound`
- Marshal updated `u` to JSON
- Put back at same key
- Return nil

**`DeleteUser(id string) error`:**
- Open a write transaction
- Get existing value at `[]byte(id)` — if nil return `ErrUserNotFound`
- Delete the key
- Return nil

**`ListUsers() ([]*User, error)`:**
- Open a read transaction
- Iterate all values in `usersBucket`, unmarshal each into `[]*User`
- Sort slice by `CreatedAt` ascending using `sort.Slice`
- Return slice (never nil — return empty slice if bucket is empty)

**`UserExists(username string) (bool, error)`:**
- Open a read transaction
- Iterate all values, check for matching `Username`
- Return `true, nil` if found, `false, nil` if not

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 06 — Create internal/auth/password.go

**Action:** Create `internal/auth/password.go`.

**Imports required:**
```go
import "golang.org/x/crypto/bcrypt"
```

**Full file content:**
```go
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plaintext password using bcrypt with cost 12.
// Returns the hash string or an error.
func HashPassword(password string) (string, error) {
    hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    if err != nil {
        return "", err
    }
    return string(hashed), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
// Returns nil if they match, non-nil error if they do not.
func CheckPassword(password, hash string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 07 — Create internal/auth/jwt.go

**Action:** Create `internal/auth/jwt.go`.

**Imports required:**
```go
import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"

    "github.com/plomvix/plomvix/internal/config"
)
```

**Claims struct — include JTI from the start:**
```go
// Claims holds the payload of a Plomvix JWT token.
// JTI is a UUID v4 generated per token, used for blacklist lookup on logout.
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     Role   `json:"role"`
    JTI      string `json:"jti"` // UUID v4 — unique per token
    jwt.RegisteredClaims
}
```

**`GenerateToken(user *User, cfg *config.Config) (string, error)`:**
```go
func GenerateToken(user *User, cfg *config.Config) (string, error) {
    now := time.Now()
    claims := &Claims{
        UserID:   user.ID,
        Username: user.Username,
        Role:     user.Role,
        JTI:      uuid.New().String(),
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(
                now.Add(time.Duration(cfg.Auth.JWTExpirySeconds) * time.Second)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(cfg.Auth.JWTSecret))
}
```

**`ParseToken(tokenString string, cfg *config.Config) (*Claims, error)`:**
```go
func ParseToken(tokenString string, cfg *config.Config) (*Claims, error) {
    token, err := jwt.ParseWithClaims(
        tokenString,
        &Claims{},
        func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v",
                    token.Header["alg"])
            }
            return []byte(cfg.Auth.JWTSecret), nil
        },
    )
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token claims")
    }
    return claims, nil
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 08 — Create internal/auth/blacklist.go

**Action:** Create `internal/auth/blacklist.go`.

**Imports required:**
```go
import (
    "sync"
    "time"
)
```

**Full implementation:**
```go
package auth

import (
    "sync"
    "time"
)

// Blacklist is a thread-safe in-memory store of invalidated JWT token IDs.
// Tokens are identified by their JTI claim (UUID v4).
// Entries are automatically pruned when their expiry time passes.
type Blacklist struct {
    mu      sync.RWMutex
    entries map[string]time.Time // jti → expiry time
    done    chan struct{}
}

// NewBlacklist creates a new Blacklist and starts a background pruning goroutine
// that removes expired entries every 5 minutes.
func NewBlacklist() *Blacklist {
    b := &Blacklist{
        entries: make(map[string]time.Time),
        done:    make(chan struct{}),
    }
    go b.prune()
    return b
}

// Add adds a token JTI to the blacklist until its expiry time.
func (b *Blacklist) Add(jti string, expiry time.Time) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.entries[jti] = expiry
}

// IsBlacklisted returns true if the given JTI is currently in the blacklist.
func (b *Blacklist) IsBlacklisted(jti string) bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    expiry, ok := b.entries[jti]
    if !ok {
        return false
    }
    // Entry exists — only blacklisted if not yet expired
    return time.Now().Before(expiry)
}

// Stop signals the background pruning goroutine to exit.
// Call during graceful shutdown.
func (b *Blacklist) Stop() {
    close(b.done)
}

// prune runs in a background goroutine, removing expired entries every 5 minutes.
func (b *Blacklist) prune() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            b.mu.Lock()
            now := time.Now()
            for jti, expiry := range b.entries {
                if expiry.Before(now) {
                    delete(b.entries, jti)
                }
            }
            b.mu.Unlock()
        case <-b.done:
            return
        }
    }
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 09 — Create internal/auth/apikey.go

**Action:** Create `internal/auth/apikey.go`.

**Imports required:**
```go
import (
    "crypto/rand"
    "encoding/base64"

    "github.com/plomvix/plomvix/internal/config"
)
```

**Full implementation:**
```go
package auth

import (
    "crypto/rand"
    "encoding/base64"

    "github.com/plomvix/plomvix/internal/config"
)

// GenerateAPIKey generates a cryptographically random API key.
// Uses crypto/rand — NOT math/rand.
// Returns the plaintext key (show to user once, never store) and its bcrypt hash.
func GenerateAPIKey(cfg *config.Config) (plaintext string, hash string, err error) {
    b := make([]byte, cfg.Auth.APIKeyLength)
    if _, err = rand.Read(b); err != nil {
        return "", "", err
    }
    plaintext = base64.RawURLEncoding.EncodeToString(b)
    hash, err = HashPassword(plaintext) // reuses bcrypt helper from password.go
    if err != nil {
        return "", "", err
    }
    return plaintext, hash, nil
}

// CheckAPIKey compares a plaintext API key against a bcrypt hash.
// Returns nil if they match, non-nil error otherwise.
func CheckAPIKey(plaintext, hash string) error {
    return CheckPassword(plaintext, hash) // reuses bcrypt helper from password.go
}

// FindUserByAPIKey searches all users for one whose API key matches the plaintext.
// Returns ErrUserNotFound if no user matches.
//
// PERFORMANCE NOTE: bcrypt comparison per user (~250ms each at cost 12).
// Acceptable for ≤20 users. Sprint 4 must revisit with indexed lookup.
func FindUserByAPIKey(store *Store, plaintext string) (*User, error) {
    users, err := store.ListUsers()
    if err != nil {
        return nil, err
    }
    for _, u := range users {
        if u.APIKeyHash == "" {
            continue
        }
        if CheckAPIKey(plaintext, u.APIKeyHash) == nil {
            return u, nil
        }
    }
    return nil, ErrUserNotFound
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 10 — Create internal/auth/context.go

**Action:** Create `internal/auth/context.go`.

**Imports required:**
```go
import "context"
```

**Full implementation:**
```go
package auth

import "context"

// contextKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with context keys from other packages.
type contextKey string

const userContextKey contextKey = "plomvix_authenticated_user"

// WithUser returns a new context with the authenticated user attached.
func WithUser(ctx context.Context, user *User) context.Context {
    return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context.
// Returns nil if no user is present. Does not panic.
func UserFromContext(ctx context.Context) *User {
    user, _ := ctx.Value(userContextKey).(*User)
    return user
}

// RequireUser retrieves the authenticated user from the context.
// Panics if no user is present — only call inside handlers protected by Middleware.
func RequireUser(ctx context.Context) *User {
    user := UserFromContext(ctx)
    if user == nil {
        panic("plomvix: RequireUser called on unprotected route — add auth.Middleware")
    }
    return user
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 11 — Create internal/auth/middleware.go

**Action:** Create `internal/auth/middleware.go`.

**Imports required:**
```go
import (
    "net/http"
    "strings"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**`Middleware` function — implement authentication flow exactly:**
```go
// Middleware returns an HTTP middleware that authenticates every request.
// Checks X-API-Key header first, then Authorization: Bearer JWT.
// Attaches the resolved User to context on success. Returns 401 on failure.
func Middleware(store *Store, blacklist *Blacklist, cfg *config.Config) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

            // Step 1: API key check — if header present, this is the ONLY auth method tried
            if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
                user, err := FindUserByAPIKey(store, apiKey)
                if err != nil {
                    utils.Unauthorized(w, r, "invalid API key")
                    return
                }
                next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
                return
            }

            // Step 2: JWT check
            authHeader := r.Header.Get("Authorization")
            if strings.HasPrefix(authHeader, "Bearer ") {
                tokenString := strings.TrimPrefix(authHeader, "Bearer ")
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
                    // User deleted after token was issued — token is orphaned
                    utils.Unauthorized(w, r, "invalid or expired token")
                    return
                }
                next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
                return
            }

            // Step 3: Neither header present
            utils.Unauthorized(w, r, "authentication required")
        })
    }
}
```

**`RequireAdmin` function — add in the same file:**
```go
// RequireAdmin returns an HTTP middleware that enforces admin role.
// Must be used AFTER Middleware — assumes a User is already in context.
// Returns 403 for non-admin users.
// Panics if no user in context — this is a programming error.
func RequireAdmin() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := RequireUser(r.Context())
            if user.Role != RoleAdmin {
                utils.Forbidden(w, r, "admin role required")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 12 — Create internal/auth/bootstrap.go

**Action:** Create `internal/auth/bootstrap.go`.

**Imports required:**
```go
import (
    "time"

    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
)
```

**Full implementation:**
```go
package auth

import (
    "time"

    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
)

// BootstrapAdminUser creates the default admin user if no users exist in the store.
// Safe to call on every startup — does nothing if users already exist.
func BootstrapAdminUser(store *Store, cfg *config.Config) error {
    users, err := store.ListUsers()
    if err != nil {
        return err
    }
    if len(users) > 0 {
        return nil // users already exist, nothing to do
    }

    passwordHash, err := HashPassword(cfg.Auth.DefaultAdminPassword)
    if err != nil {
        return err
    }

    _, apiKeyHash, err := GenerateAPIKey(cfg)
    if err != nil {
        return err
    }

    user := User{
        ID:           uuid.New().String(),
        Username:     cfg.Auth.DefaultAdminUsername,
        PasswordHash: passwordHash,
        Role:         RoleAdmin,
        APIKeyHash:   apiKeyHash,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    if err := store.CreateUser(&user); err != nil {
        return err
    }

    logger.Info("default admin user created",
        zap.String("username", cfg.Auth.DefaultAdminUsername),
    )
    logger.Warn("default admin credentials are set to defaults — change before exposing to network")

    return nil
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 13 — Create internal/auth/handler.go

**Action:** Create `internal/auth/handler.go` with the three auth endpoints.

**Imports required:**
```go
import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
type Handler struct {
    store     *Store
    blacklist *Blacklist
    cfg       *config.Config
}

func NewHandler(store *Store, blacklist *Blacklist, cfg *config.Config) *Handler {
    return &Handler{store: store, blacklist: blacklist, cfg: cfg}
}
```

**`Login` handler:**
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
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }
    if req.Username == "" || req.Password == "" {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "username and password are required")
        return
    }

    user, err := h.store.GetUserByUsername(req.Username)
    if err != nil {
        utils.Unauthorized(w, r, "invalid username or password")
        return
    }
    if err := CheckPassword(req.Password, user.PasswordHash); err != nil {
        utils.Unauthorized(w, r, "invalid username or password")
        return
    }

    token, err := GenerateToken(user, h.cfg)
    if err != nil {
        utils.InternalError(w, r, "failed to generate token")
        return
    }

    utils.OK(w, r, map[string]interface{}{
        "token":      token,
        "expires_in": h.cfg.Auth.JWTExpirySeconds,
        "user":       user.ToResponse(),
    })
}
```

**`Logout` handler:**
```go
// Logout invalidates the current JWT token by adding it to the blacklist.
// If authenticated via API key (no Bearer token), returns 200 with no action.
//
// POST /auth/logout
// Auth: JWT or API key
//
// Responses:
//   200 OK — logged out successfully
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    _ = RequireUser(r.Context()) // ensures middleware ran

    authHeader := r.Header.Get("Authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := ParseToken(tokenString, h.cfg)
        if err == nil {
            h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
        }
    }
    // If no Bearer token (API key auth), nothing to blacklist — return 200 either way

    utils.OK(w, r, map[string]interface{}{
        "message": "logged out successfully",
    })
}
```

**`Refresh` handler:**
```go
// Refresh invalidates the current JWT and issues a new one.
//
// POST /auth/refresh
// Auth: JWT only
//
// Responses:
//   200 OK           — new token issued
//   500 Internal     — INTERNAL_ERROR: token generation failed
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
    user := RequireUser(r.Context())

    freshUser, err := h.store.GetUserByID(user.ID)
    if err != nil {
        utils.InternalError(w, r, "failed to fetch user")
        return
    }

    // Blacklist the current token
    authHeader := r.Header.Get("Authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        if claims, err := ParseToken(tokenString, h.cfg); err == nil {
            h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
        }
    }

    newToken, err := GenerateToken(freshUser, h.cfg)
    if err != nil {
        utils.InternalError(w, r, "failed to generate token")
        return
    }

    utils.OK(w, r, map[string]interface{}{
        "token":      newToken,
        "expires_in": h.cfg.Auth.JWTExpirySeconds,
    })
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 14 — Create internal/auth/user_handler.go

**Action:** Create `internal/auth/user_handler.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "net/http"
    "regexp"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
type UserHandler struct {
    store *Store
    cfg   *config.Config
}

func NewUserHandler(store *Store, cfg *config.Config) *UserHandler {
    return &UserHandler{store: store, cfg: cfg}
}
```

**Username validation regex — define at package level:**
```go
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
```

**`Create` handler — `POST /admin/users`:**
```go
// Create creates a new user account.
//
// POST /admin/users
// Auth: admin only
//
// Request body: {"username": string, "password": string}
//
// Responses:
//   201 Created      — user created, returns UserResponse
//   400 Bad Request  — VALIDATION_FAILED: invalid username or password format
//   409 Conflict     — CONFLICT: username already exists
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }

    var errs []string
    if !usernameRegex.MatchString(req.Username) {
        errs = append(errs,
            "username must be 3-64 characters, alphanumeric, underscore, or hyphen only")
    }
    if len(req.Password) < 8 {
        errs = append(errs, "password must be at least 8 characters")
    }
    if len(errs) > 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "validation failed", errs...)
        return
    }

    exists, err := h.store.UserExists(req.Username)
    if err != nil {
        utils.InternalError(w, r, "failed to check username")
        return
    }
    if exists {
        utils.Conflict(w, r, "username already exists")
        return
    }

    hash, err := HashPassword(req.Password)
    if err != nil {
        utils.InternalError(w, r, "failed to hash password")
        return
    }

    now := time.Now()
    user := &User{
        ID:           uuid.New().String(),
        Username:     req.Username,
        PasswordHash: hash,
        Role:         RoleAdmin,
        CreatedAt:    now,
        UpdatedAt:    now,
    }
    if err := h.store.CreateUser(user); err != nil {
        utils.InternalError(w, r, "failed to create user")
        return
    }

    utils.Created(w, r, user.ToResponse())
}
```

**`List` handler — `GET /admin/users`:**
```go
// List returns all user accounts.
//
// GET /admin/users
// Auth: admin only
//
// Responses:
//   200 OK — returns users array and count
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
    users, err := h.store.ListUsers()
    if err != nil {
        utils.InternalError(w, r, "failed to list users")
        return
    }
    responses := make([]UserResponse, len(users))
    for i, u := range users {
        responses[i] = u.ToResponse()
    }
    utils.OK(w, r, map[string]interface{}{
        "users": responses,
        "count": len(responses),
    })
}
```

**`Get` handler — `GET /admin/users/{id}`:**
```go
// Get returns a single user by ID.
//
// GET /admin/users/{id}
// Auth: admin only
//
// Responses:
//   200 OK  — returns UserResponse
//   404     — NOT_FOUND: user not found
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := h.store.GetUserByID(id)
    if err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }
    utils.OK(w, r, user.ToResponse())
}
```

**`Update` handler — `PATCH /admin/users/{id}`:**
```go
// Update updates username and/or password for a user.
// Only provided fields are updated — use pointer fields to detect omitted vs empty.
//
// PATCH /admin/users/{id}
// Auth: admin only
//
// Request body: {"username": string (optional), "password": string (optional)}
//
// Responses:
//   200 OK  — updated UserResponse
//   400     — VALIDATION_FAILED
//   404     — NOT_FOUND
//   409     — CONFLICT: new username already taken
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := h.store.GetUserByID(id)
    if err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }

    var req struct {
        Username *string `json:"username"`
        Password *string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }

    var errs []string

    if req.Username != nil {
        if !usernameRegex.MatchString(*req.Username) {
            errs = append(errs,
                "username must be 3-64 characters, alphanumeric, underscore, or hyphen only")
        }
    }
    if req.Password != nil && len(*req.Password) < 8 {
        errs = append(errs, "password must be at least 8 characters")
    }
    if len(errs) > 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "validation failed", errs...)
        return
    }

    if req.Username != nil && *req.Username != user.Username {
        exists, err := h.store.UserExists(*req.Username)
        if err != nil {
            utils.InternalError(w, r, "failed to check username")
            return
        }
        if exists {
            utils.Conflict(w, r, "username already exists")
            return
        }
        user.Username = *req.Username
    }

    if req.Password != nil {
        hash, err := HashPassword(*req.Password)
        if err != nil {
            utils.InternalError(w, r, "failed to hash password")
            return
        }
        user.PasswordHash = hash
    }

    user.UpdatedAt = time.Now()
    if err := h.store.UpdateUser(user); err != nil {
        utils.InternalError(w, r, "failed to update user")
        return
    }
    utils.OK(w, r, user.ToResponse())
}
```

**`Delete` handler — `DELETE /admin/users/{id}`:**
```go
// Delete removes a user account.
// Cannot delete your own account or the last admin user.
//
// DELETE /admin/users/{id}
// Auth: admin only
//
// Responses:
//   200 OK  — user deleted
//   400     — VALIDATION_FAILED: self-deletion or last admin
//   404     — NOT_FOUND: user not found
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    caller := RequireUser(r.Context())

    if id == caller.ID {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "cannot delete your own account")
        return
    }

    if _, err := h.store.GetUserByID(id); err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }

    users, err := h.store.ListUsers()
    if err != nil {
        utils.InternalError(w, r, "failed to list users")
        return
    }
    if len(users) == 1 {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "cannot delete the last admin user")
        return
    }

    if err := h.store.DeleteUser(id); err != nil {
        utils.InternalError(w, r, "failed to delete user")
        return
    }
    utils.OK(w, r, map[string]interface{}{"message": "user deleted"})
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 15 — Create internal/auth/apikey_handler.go

**Action:** Create `internal/auth/apikey_handler.go`.

**Imports required:**
```go
import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
type APIKeyHandler struct {
    store *Store
    cfg   *config.Config
}

func NewAPIKeyHandler(store *Store, cfg *config.Config) *APIKeyHandler {
    return &APIKeyHandler{store: store, cfg: cfg}
}
```

**`Generate` handler — `POST /admin/users/{id}/apikey`:**
```go
// Generate creates a new API key for the specified user.
// Replaces any existing key. Returns plaintext ONCE — never stored.
//
// POST /admin/users/{id}/apikey
// Auth: admin only
//
// Responses:
//   201 Created — api_key (plaintext, show once), user_id, message
//   404         — NOT_FOUND: user not found
func (h *APIKeyHandler) Generate(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := h.store.GetUserByID(id)
    if err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }

    plaintext, hash, err := GenerateAPIKey(h.cfg)
    if err != nil {
        utils.InternalError(w, r, "failed to generate API key")
        return
    }

    user.APIKeyHash = hash
    user.UpdatedAt = time.Now()
    if err := h.store.UpdateUser(user); err != nil {
        utils.InternalError(w, r, "failed to save API key")
        return
    }

    utils.Created(w, r, map[string]interface{}{
        "api_key": plaintext,
        "user_id": user.ID,
        "message": "Store this key securely. It will not be shown again.",
    })
}
```

**`Revoke` handler — `DELETE /admin/users/{id}/apikey`:**
```go
// Revoke removes the API key for the specified user.
// Idempotent — returns 200 if the user has no key (no error).
// Returns 404 only if the USER does not exist.
//
// DELETE /admin/users/{id}/apikey
// Auth: admin only
//
// Responses:
//   200 OK — API key revoked (or user had no key — idempotent)
//   404    — NOT_FOUND: user not found
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := h.store.GetUserByID(id)
    if err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }

    user.APIKeyHash = ""
    user.UpdatedAt = time.Now()
    if err := h.store.UpdateUser(user); err != nil {
        utils.InternalError(w, r, "failed to revoke API key")
        return
    }

    utils.OK(w, r, map[string]interface{}{"message": "API key revoked"})
}
```

**`Status` handler — `GET /admin/users/{id}/apikey/status`:**
```go
// Status returns whether the specified user has an API key configured.
// Never reveals the key or its hash.
//
// GET /admin/users/{id}/apikey/status
// Auth: admin only
//
// Responses:
//   200 OK — has_api_key bool, user_id
//   404    — NOT_FOUND: user not found
func (h *APIKeyHandler) Status(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := h.store.GetUserByID(id)
    if err != nil {
        utils.NotFound(w, r, "user not found")
        return
    }

    utils.OK(w, r, map[string]interface{}{
        "has_api_key": user.APIKeyHash != "",
        "user_id":     user.ID,
    })
}
```

**Verify:** `go build ./internal/auth/` compiles with no errors.

---

## TASK 16 — Update internal/server/server.go

**Action:** Update `internal/server/server.go` — three changes.

**Change 1 — Update `Server` struct to hold auth dependencies:**
```go
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
    version    string
    store      *auth.Store      // ← ADD
    blacklist  *auth.Blacklist  // ← ADD
}
```

**Change 2 — Update `New()` signature and route registration:**
```go
func New(cfg *config.Config, version string, store *auth.Store, blacklist *auth.Blacklist) *Server {
    s := &Server{
        router:    chi.NewRouter(),
        cfg:       cfg,
        startTime: time.Now(),
        version:   version,
        store:     store,
        blacklist: blacklist,
    }
    // ... (existing httpServer setup unchanged) ...
    s.setupMiddleware()
    s.setupRoutes()
    return s
}
```

**Change 3 — Update `setupRoutes()` to register all Sprint 2 routes:**
```go
func (s *Server) setupRoutes() {
    authHandler    := auth.NewHandler(s.store, s.blacklist, s.cfg)
    userHandler    := auth.NewUserHandler(s.store, s.cfg)
    apiKeyHandler  := auth.NewAPIKeyHandler(s.store, s.cfg)

    // Public — no auth
    s.router.Get("/health", s.handleHealth)
    s.router.Post("/auth/login", authHandler.Login)

    // Protected — auth required
    s.router.Group(func(r chi.Router) {
        r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
        r.Post("/auth/logout", authHandler.Logout)
        r.Post("/auth/refresh", authHandler.Refresh)
    })

    // Admin only — auth + admin role
    s.router.Group(func(r chi.Router) {
        r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
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
}
```

**Import to add at top of server.go:**
```go
"github.com/plomvix/plomvix/internal/auth"
```

**Verify:** `go build ./internal/server/` compiles with no errors.

---

## TASK 17 — Update cmd/plomvix/main.go

**Action:** Update `cmd/plomvix/main.go` boot sequence.

**Change 1 — Update `bootstrapDataDirs` to include `system` dir:**
```go
func bootstrapDataDirs(cfg *config.Config) error {
    dirs := []string{
        filepath.Join(cfg.Storage.DataDir, "wal"),
        filepath.Join(cfg.Storage.DataDir, "hot"),
        filepath.Join(cfg.Storage.DataDir, "cold", "logs"),
        filepath.Join(cfg.Storage.DataDir, "cold", "metrics"),
        filepath.Join(cfg.Storage.DataDir, "cold", "json"),
        filepath.Join(cfg.Storage.DataDir, "cold", "kv"),
        filepath.Join(cfg.Storage.DataDir, "system"),  // ← ADD THIS LINE
    }
    // ... rest unchanged ...
}
```

**Change 2 — Add auth initialization after `bootstrapDataDirs` call:**

Insert these steps between the existing `bootstrapDataDirs` call and the `server.New()` call:

```go
// Open user store
store, err := auth.NewStore(
    filepath.Join(cfg.Storage.DataDir, "system", "auth.db"))
if err != nil {
    logger.Error("failed to open user store", zap.Error(err))
    os.Exit(1)
}

// Bootstrap admin user — explicit close before any os.Exit after this point
if err := auth.BootstrapAdminUser(store, cfg); err != nil {
    store.Close()
    logger.Error("failed to bootstrap admin user", zap.Error(err))
    os.Exit(1)
}
defer store.Close()

// Create blacklist
blacklist := auth.NewBlacklist()
defer blacklist.Stop()
```

**Change 3 — Update `server.New()` call to pass auth dependencies:**
```go
// Before (Sprint 1):
srv := server.New(cfg, Version)

// After (Sprint 2):
srv := server.New(cfg, Version, store, blacklist)
```

**Imports to add:**
```go
"github.com/plomvix/plomvix/internal/auth"
```

**Verify:** `go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 18 — Create internal/auth/auth_test.go

**Action:** Create `internal/auth/auth_test.go` with unit tests for all auth utilities.

**Package declaration:** `package auth`

**Test config helper — do NOT call config.Load():**
```go
func testConfig() *config.Config {
    return &config.Config{
        Auth: config.AuthConfig{
            JWTSecret:        "test-secret-key-minimum-length",
            JWTExpirySeconds: 3600,
            APIKeyLength:     32,
        },
    }
}
```

**Test user helper:**
```go
func testUser() *User {
    return &User{
        ID:       "test-user-id",
        Username: "testuser",
        Role:     RoleAdmin,
    }
}
```

**Tests to implement:**

```
TestHashPassword:
  hash, err := HashPassword("password")
  - err must be nil
  - hash must be non-empty
  - hash must start with "$2a$12$"
  - calling again returns different hash (bcrypt salting)

TestCheckPassword:
  hash, _ := HashPassword("password")
  - CheckPassword("password", hash) must return nil
  - CheckPassword("wrong", hash) must return non-nil error

TestGenerateToken:
  token, err := GenerateToken(testUser(), testConfig())
  - err must be nil
  - token must be non-empty
  - strings.Split(token, ".") must have length 3
  - call twice → different tokens (different JTI)
  - ParseToken the result → claims.UserID must equal testUser().ID
  - claims.Role must equal RoleAdmin
  - claims.JTI must be non-empty

TestParseToken:
  valid, _ := GenerateToken(testUser(), testConfig())
  - ParseToken(valid, testConfig()) returns non-nil claims, nil error
  - claims.UserID == "test-user-id"
  - claims.JTI non-empty
  - ParseToken("", testConfig()) returns non-nil error
  - ParseToken("not.a.token", testConfig()) returns non-nil error
  - ParseToken(valid, cfgWithWrongSecret) returns non-nil error
  - For expired token: use JWTExpirySeconds: -3600 (1 hour in the past)
    to guarantee expiry regardless of any clock leeway:
    expiredCfg := &config.Config{Auth: config.AuthConfig{
        JWTSecret: "test-secret-key-minimum-length", JWTExpirySeconds: -3600}}
    expiredToken, _ := GenerateToken(testUser(), expiredCfg)
    ParseToken(expiredToken, expiredCfg) → must return non-nil error

TestBlacklist:
  b := NewBlacklist()
  defer b.Stop()
  - IsBlacklisted("unknown-jti") must return false
  - b.Add("jti-1", time.Now().Add(1*time.Hour))
    IsBlacklisted("jti-1") must return true
  - b.Add("jti-2", time.Now().Add(-1*time.Hour)) (already expired)
    IsBlacklisted("jti-2") must return false

TestGenerateAPIKey:
  plaintext, hash, err := GenerateAPIKey(testConfig())
  - err must be nil
  - plaintext must be non-empty
  - hash must be non-empty
  - plaintext != hash
  - CheckAPIKey(plaintext, hash) must return nil
  - CheckAPIKey("wrong", hash) must return non-nil error
  - two calls must return different plaintexts
```

**Verify:** `go test -race ./internal/auth/` — all TestHash*, TestCheck*,
TestGenerate*, TestParse*, TestBlacklist tests pass.

---

## TASK 19 — Create internal/auth/store_test.go

**Action:** Create `internal/auth/store_test.go`.

**Package declaration:** `package auth`

**Test store helper:**
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

**Test user factory:**
```go
func makeUser(username string) *User {
    return &User{
        ID:           uuid.New().String(),
        Username:     username,
        PasswordHash: "hash",
        Role:         RoleAdmin,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }
}
```

**Tests to implement:**

```
TestStoreCreateAndGet:
  store := newTestStore(t)
  u := makeUser("alice")
  - CreateUser(u) must return nil
  - GetUserByID(u.ID) returns user with same ID and Username
  - GetUserByUsername("alice") returns user with same ID
  - CreateUser(makeUser("alice")) returns ErrUserAlreadyExists
  - GetUserByID("nonexistent") returns ErrUserNotFound
  - GetUserByUsername("nonexistent") returns ErrUserNotFound

TestStoreUpdate:
  store := newTestStore(t)
  u := makeUser("bob")
  CreateUser(u)
  u.Username = "bob-updated"
  - UpdateUser(u) must return nil
  - GetUserByID(u.ID) returns Username == "bob-updated"
  - UpdateUser(&User{ID: "ghost"}) returns ErrUserNotFound

TestStoreDelete:
  store := newTestStore(t)
  u := makeUser("carol")
  CreateUser(u)
  - DeleteUser(u.ID) must return nil
  - GetUserByID(u.ID) returns ErrUserNotFound
  - DeleteUser("ghost") returns ErrUserNotFound

TestStoreList:
  store := newTestStore(t)
  - ListUsers() on empty store returns empty slice (len == 0, not nil)
  Create 3 users with explicit distinct CreatedAt values — do NOT use time.Sleep
  (time.Sleep is flaky; OS scheduler may batch sleeps producing identical times):
    u1 := makeUser("user1"); u1.CreatedAt = time.Now().Add(-2 * time.Second)
    u2 := makeUser("user2"); u2.CreatedAt = time.Now().Add(-1 * time.Second)
    u3 := makeUser("user3"); u3.CreatedAt = time.Now()
    store.CreateUser(u1), store.CreateUser(u2), store.CreateUser(u3)
  - ListUsers() returns all 3 in ascending CreatedAt order
    (u1 first, u3 last)

TestUserExists:
  store := newTestStore(t)
  u := makeUser("dave")
  CreateUser(u)
  - UserExists("dave") returns true, nil
  - UserExists("unknown") returns false, nil
```

**Imports needed:**
```go
import (
    "path/filepath"
    "testing"
    "time"

    "github.com/google/uuid"
)
```

**Verify:** `go test -race ./internal/auth/` — all TestStore* tests pass.

---

## TASK 20 — Create internal/auth/integration_test.go

**Action:** Create `internal/auth/integration_test.go`.

**Package declaration:** `package auth`

**Imports required:**
```go
import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/go-chi/chi/v5"
)
```

**Test router helper — returns both handler and store so tests can do direct store setup:**
```go
func newTestRouter(t *testing.T) (http.Handler, *Store) {
    t.Helper()
    store := newTestStore(t)
    cfg := testConfig()
    blacklist := NewBlacklist()
    t.Cleanup(func() { blacklist.Stop() })

    if err := BootstrapAdminUser(store, cfg); err != nil {
        t.Fatalf("bootstrap failed: %v", err)
    }

    r := chi.NewRouter()
    authHandler   := NewHandler(store, blacklist, cfg)
    userHandler   := NewUserHandler(store, cfg)

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
    return r, store
}
```

**Login helper:**
```go
func loginAs(t *testing.T, router http.Handler, username, password string) string {
    t.Helper()
    body, _ := json.Marshal(map[string]string{
        "username": username, "password": password,
    })
    req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("login failed: status %d", w.Code)
    }
    var resp map[string]interface{}
    json.NewDecoder(w.Body).Decode(&resp)
    return resp["data"].(map[string]interface{})["token"].(string)
}
```

**Tests to implement:**

```
TestLoginSuccess:
  router, _ := newTestRouter(t)
  body := {"username":"admin","password":"changeme"}
  POST /auth/login → 200, response has "data.token" field

TestLoginWrongPassword:
  router, _ := newTestRouter(t)
  POST /auth/login {"username":"admin","password":"wrong"} → 401
  error message == "invalid username or password"

TestLoginWrongUsername:
  router, _ := newTestRouter(t)
  POST /auth/login {"username":"nobody","password":"changeme"} → 401
  error message == "invalid username or password"  ← SAME as wrong password

TestLoginMissingFields:
  router, _ := newTestRouter(t)
  POST /auth/login {} → 400, code == "VALIDATION_FAILED"

TestProtectedRouteWithJWT:
  router, _ := newTestRouter(t)
  token := loginAs(t, router, "admin", "changeme")
  POST /admin/users with Authorization: Bearer <token>
  body: {"username":"newuser","password":"password123"}
  → 201

TestProtectedRouteNoAuth:
  router, _ := newTestRouter(t)
  GET /admin/users with no headers → 401

TestProtectedRouteWithAPIKey:
  router, store := newTestRouter(t)
  // Bootstrap admin has a random hashed API key — plaintext unknown.
  // Generate a known API key and assign it to admin directly:
  plaintext, hash, _ := GenerateAPIKey(testConfig())
  admin, _ := store.GetUserByUsername("admin")
  admin.APIKeyHash = hash
  store.UpdateUser(admin)
  // Now authenticate with the known plaintext key
  GET /admin/users with X-API-Key: plaintext → 200

TestLogoutInvalidatesToken:
  router, _ := newTestRouter(t)
  token := loginAs(t, router, "admin", "changeme")
  POST /auth/logout with token → 200
  GET /admin/users with same token → 401

TestRefreshIssuesNewToken:
  router, _ := newTestRouter(t)
  tokenA := loginAs(t, router, "admin", "changeme")
  POST /auth/refresh with tokenA → 200, get tokenB
  GET /admin/users with tokenA → 401 (blacklisted)
  GET /admin/users with tokenB → 200 (valid)

TestInvalidAPIKey:
  router, _ := newTestRouter(t)
  GET /admin/users with X-API-Key: "wrongkey" → 401
  (must NOT fall through to JWT — no Authorization header sent)
```

**Verify:** `go test -race ./internal/auth/` — all integration tests pass.

---

## TASK 21 — Create docs/api/auth.md

**Action:** Create the directory and API reference document.

```bash
mkdir -p docs/api
```

Create `docs/api/auth.md` with the following structure.
Write actual content — not placeholders.

```markdown
# Plomvix Auth API Reference

## Authentication

Plomvix supports two authentication methods. Include one of the following
on every request to a protected endpoint:

**JWT Bearer token** (human users):
  Authorization: Bearer <jwt_token>

**API Key** (machine/service clients):
  X-API-Key: <api_key>

If both headers are present, X-API-Key takes priority.
A failed API key check returns 401 immediately — the JWT is not checked as fallback.

---

## Endpoints

[For each of the 11 endpoints, write a section with:]
  - Method + path
  - Auth requirement
  - Request body with field types and validation rules (if applicable)
  - Success response — full JSON example
  - All error responses — full JSON examples with codes
  - Example curl command

Endpoints to document:
  POST   /auth/login
  POST   /auth/logout
  POST   /auth/refresh
  POST   /admin/users
  GET    /admin/users
  GET    /admin/users/{id}
  PATCH  /admin/users/{id}
  DELETE /admin/users/{id}
  POST   /admin/users/{id}/apikey
  DELETE /admin/users/{id}/apikey
  GET    /admin/users/{id}/apikey/status

---

## Error Codes

| Code | HTTP Status | Description |
|---|---|---|
| UNAUTHORIZED | 401 | Missing, invalid, expired, or revoked credentials |
| FORBIDDEN | 403 | Authenticated but insufficient permissions |
| VALIDATION_FAILED | 400 | Request body missing required fields or fails validation |
| NOT_FOUND | 404 | Requested resource does not exist |
| CONFLICT | 409 | Resource already exists (e.g. duplicate username) |
| INTERNAL_ERROR | 500 | Unexpected server error |

---

## Standard Response Envelope

Success:
{
  "status": "ok",
  "data": { ... },
  "request_id": "uuid"
}

Error:
{
  "status": "error",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": ["optional", "array", "of", "specifics"]
  },
  "request_id": "uuid"
}
```

**Acceptance Criteria:**
- `docs/api/auth.md` exists and renders on GitHub
- Every endpoint has a curl example using `http://localhost:8080`
- All JSON examples match the actual handler responses
- `docs/` directory is NOT in `.gitignore`

**Verify:** `cat docs/api/auth.md` shows full content. `ls docs/api/` shows the file.

---

## TASK 22 — Full build and smoke test

**Action:** Run the following verification sequence.

```bash
#!/bin/bash
set -euo pipefail

SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Build ==="
make vet
make build

echo ""
echo "=== Step 2: Run tests ==="
make test

echo ""
echo "=== Step 3: Boot server ==="
./plomvix > /tmp/plomvix_s2.log 2>&1 &
SERVER_PID=$!
sleep 2

echo ""
echo "=== Step 4: Health check still works (no auth) ==="
curl -sf http://localhost:8080/health | jq .

echo ""
echo "=== Step 5: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' \
  | jq -r '.data.token')
echo "Token: ${TOKEN:0:20}..."

echo ""
echo "=== Step 6: Protected route requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/users)
[ "$STATUS" -eq 401 ] && echo "PASS: no auth → 401" || { echo "FAIL: got $STATUS"; exit 1; }

echo ""
echo "=== Step 7: Protected route with JWT ==="
curl -sf http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN" | jq .

echo ""
echo "=== Step 8: Create user ==="
curl -sf -X POST http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}' | jq .

echo ""
echo "=== Step 9: Duplicate username returns 409 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}')
[ "$STATUS" -eq 409 ] && echo "PASS: duplicate → 409" || { echo "FAIL: got $STATUS"; exit 1; }

echo ""
echo "=== Step 10: Logout invalidates token ==="
curl -sf -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer $TOKEN" | jq .
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 401 ] && echo "PASS: logged out token → 401" || { echo "FAIL: got $STATUS"; exit 1; }

echo ""
echo "=== Step 11: Generate API key ==="
NEW_TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
USER_ID=$(curl -sf http://localhost:8080/admin/users \
  -H "Authorization: Bearer $NEW_TOKEN" | jq -r '.data.users[0].id')
API_KEY=$(curl -sf -X POST "http://localhost:8080/admin/users/${USER_ID}/apikey" \
  -H "Authorization: Bearer $NEW_TOKEN" | jq -r '.data.api_key')
echo "API Key generated: ${API_KEY:0:10}..."

echo ""
echo "=== Step 12: Use API key for auth ==="
curl -sf http://localhost:8080/admin/users \
  -H "X-API-Key: $API_KEY" | jq .
echo "PASS: API key auth works"

echo ""
echo "=== Step 13: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 2 smoke test DONE  "
echo "================================================"
```

**Expected results per step:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Build + vet | Binary produced, no errors |
| 2 | Unit + integration tests | All pass with race detector |
| 3 | Boot | Server starts, default admin created |
| 4 | Health | 200 without any auth |
| 5 | Login | Returns valid JWT |
| 6 | No auth | 401 on protected route |
| 7 | JWT auth | 200 on admin route |
| 8 | Create user | 201, user appears in list |
| 9 | Duplicate username | 409 CONFLICT |
| 10 | Logout | Token rejected after logout |
| 11 | Generate API key | Plaintext key returned once |
| 12 | API key auth | 200 using X-API-Key header |
| 13 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  Install dependencies
TASK 02  →  .gitignore + data/system/.gitkeep
TASK 03  →  Add CONFLICT code to response.go
TASK 04  →  internal/auth/model.go
TASK 05  →  internal/auth/store.go
TASK 06  →  internal/auth/password.go
TASK 07  →  internal/auth/jwt.go
TASK 08  →  internal/auth/blacklist.go
TASK 09  →  internal/auth/apikey.go
TASK 10  →  internal/auth/context.go
TASK 11  →  internal/auth/middleware.go
TASK 12  →  internal/auth/bootstrap.go
TASK 13  →  internal/auth/handler.go
TASK 14  →  internal/auth/user_handler.go
TASK 15  →  internal/auth/apikey_handler.go
TASK 16  →  internal/server/server.go (update)
TASK 17  →  cmd/plomvix/main.go (update)
TASK 18  →  internal/auth/auth_test.go
TASK 19  →  internal/auth/store_test.go
TASK 20  →  internal/auth/integration_test.go
TASK 21  →  docs/api/auth.md
TASK 22  →  smoke test — all 13 steps must pass
```

---

*Sprint 2 complete when TASK 22 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*