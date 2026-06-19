catalog_setup.md — Global System Catalog (Basic)
Scope
This plan delivers the Global System Catalog (`catalog` package), the 
server-level control plane for Plomvix. It manages system metadata 
(`_plomvix_tables`, `_plomvix_users`, `_plomvix_meta`), registers pluggable 
engines (SQL, Metrics, Logs), and provides schema resolution and basic 
authentication.

The Catalog acts as a "client" to the SQL Engine's Enterprise Table Heap 
to persist its own system tables, but it serves the entire server. 

This plan does NOT deliver complex Role-Based Access Control (RBAC), 
column-level permissions, SQL parsing, query routing, or physical table 
data deletion. It provides strict table/user uniqueness, basic password 
authentication, and an in-memory cache for fast metadata lookups.

Contract this tier honestly provides (read this before writing code)
- Meta-First Tx Persistence: The Enterprise Heap requires a `heap.Tx`. 
  The Catalog maintains an internal `nextTxID`. To survive restarts safely, 
  the Catalog persists its `nextTxID` into `_plomvix_meta` *before* writing 
  the actual catalog data row. Both writes use the exact same `txID`. 
  Crucially, `c.nextTxID` is updated in-memory *immediately* after the 
  meta write succeeds. If the subsequent data write fails, the TxID is 
  safely "gapped" in-memory and on-disk, preventing reuse in the same process.
- Safe Locking & Stop Draining: To prevent deadlocks, the Catalog NEVER 
  holds its internal `sync.RWMutex` (`mu`) while performing Heap I/O. 
  DDL operations use a multi-phase lock/unlock pattern with `pending` maps. 
  To prevent nil-pointer panics if `Stop()` runs between phases, the 
  write-lock reservation phase explicitly re-checks `!started || cache == nil`.
  `Stop()` explicitly checks these `pending` maps and returns `ErrConflict` 
  if operations are in-flight, preventing cache teardown during I/O.
- In-Memory Cache & Deep Copies: The Catalog loads system tables into memory 
  on `Start()`. All read operations are served exclusively from the cache. 
  All cache reads and writes MUST perform deep copies of `[]byte` fields.
- Metadata-Only DDL: `DropTable` removes catalog metadata only.
- Honest Basic Authentication: Passwords are hashed using `crypto/sha256` 
  with a random 16-byte salt. Empty passwords are allowed. Comparison 
  uses `crypto/subtle.ConstantTimeCompare`.

Constants & System Table Definitions
package catalog

import (
    "context"
    "errors"
    "github.com/plomvix/plomvix/internal/engine/sql/heap"
    "github.com/plomvix/plomvix/internal/engine/sql/key"
)

const (
    SystemTableTablesID uint64 = 1
    SystemTableUsersID  uint64 = 2
    SystemTableMetaID   uint64 = 3
    
    MetaKeyNextTxID = "catalog_next_tx_id"
)

var (
    schemaTables = heap.Schema{
        TableID: SystemTableTablesID,
        Columns: []heap.Column{
            {Name: "table_id", Kind: key.KindUint64},
            {Name: "engine_name", Kind: key.KindString},
            {Name: "table_name", Kind: key.KindString},
            {Name: "schema_payload", Kind: key.KindBytes},
        },
        PKIndices: []int{0},
    }

    schemaUsers = heap.Schema{
        TableID: SystemTableUsersID,
        Columns: []heap.Column{
            {Name: "user_id", Kind: key.KindUint64},
            {Name: "username", Kind: key.KindString},
            {Name: "password_hash", Kind: key.KindBytes},
            {Name: "is_admin", Kind: key.KindUint64}, 
        },
        PKIndices: []int{0},
    }

    schemaMeta = heap.Schema{
        TableID: SystemTableMetaID,
        Columns: []heap.Column{
            {Name: "meta_key", Kind: key.KindString},
            {Name: "meta_uint64", Kind: key.KindUint64},
        },
        PKIndices: []int{0},
    }
)

Public API
package catalog

type Engine interface {
    Name() string 
    ValidateSchema(schemaJSON []byte) error
}

type TableInfo struct {
    TableID       uint64
    EngineName    string
    TableName     string
    SchemaPayload []byte 
}

type UserInfo struct {
    UserID       uint64
    Username     string
    PasswordHash []byte 
    IsAdmin      bool
}

type Catalog interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    RegisterEngine(e Engine) error
    CreateTable(ctx context.Context, engineName, tableName string, schemaJSON []byte) error
    DropTable(ctx context.Context, tableName string) error
    GetTable(ctx context.Context, tableName string) (TableInfo, error)
    CreateUser(ctx context.Context, username, password string, isAdmin bool) error
    Authenticate(ctx context.Context, username, password string) (UserInfo, error)
}

var (
    ErrEngineNotFound        = errors.New("catalog: engine not registered")
    ErrDuplicateEngine       = errors.New("catalog: engine already registered")
    ErrInvalidEngine         = errors.New("catalog: invalid engine (nil or empty name)")
    ErrTableNotFound         = errors.New("catalog: table not found")
    ErrDuplicateTable        = errors.New("catalog: table name already exists")
    ErrDuplicateUser         = errors.New("catalog: username already exists")
    ErrInvalidSchema         = errors.New("catalog: engine rejected schema payload")
    ErrAuthFailed            = errors.New("catalog: authentication failed")
    ErrCatalogNotStarted     = errors.New("catalog: not started")
    ErrCatalogAlreadyStarted = errors.New("catalog: already started or starting")
    ErrEmptyName             = errors.New("catalog: name cannot be empty")
    ErrConflict              = errors.New("catalog: concurrent operation conflict")
)

func New(h *heap.Heap) Catalog

Tasks (do in order, one at a time)

Task 1 — Package skeleton, Engine Registry
Create `internal/catalog/catalog.go`. Define all types, interfaces, and errors.
Implement `New(h *heap.Heap) Catalog` returning a concrete `*catalog` struct.
The struct must hold:
- `h *heap.Heap`, `tablesHandle`, `usersHandle`, `metaHandle` (heap.Table)
- `engines map[string]Engine`
- `cache *cache` 
- `mu sync.RWMutex` 
- `started bool`, `starting bool`
- `nextTxID uint64`

Implement `RegisterEngine(e Engine)`:
- Validate `e != nil` and `e.Name() != ""`, else return `ErrInvalidEngine`.
- Lock `mu`. If `started` OR `starting`, unlock and return `ErrCatalogAlreadyStarted`.
- If `e.Name()` exists in map, unlock and return `ErrDuplicateEngine`.
- Add to map, unlock.
Tests: Register valid engine. Register nil/empty (ErrInvalidEngine). Register duplicate (ErrDuplicateEngine). Register after start/during start fails.

Task 2 — System Table Bootstrap & Meta Initialization
Create `internal/catalog/bootstrap.go`.
Implement `bootstrap(ctx context.Context) error`:
1. Open `_plomvix_tables`, `_plomvix_users`, and `_plomvix_meta` via `h.OpenTable`.
2. Store these `heap.Table` handles in the catalog struct.
3. Scan `_plomvix_meta`. If the `MetaKeyNextTxID` row does not exist, 
   insert it with value `1` using a temporary `heap.Tx{ID: 1}`. 
   (Value is 1, not 0, to prevent the first real catalog write from 
   attempting to reuse txID=1, which would violate the Heap's per-PK 
   monotonic rule).
4. DO NOT create a default admin user.
Tests: Bootstrap on fresh Heap succeeds, creates meta row with value 1, leaves tables/users empty.

Task 3 — In-Memory Cache Layer & Deep Copies
Create `internal/catalog/cache.go`.
Define the `cache` struct holding:
- `tables map[string]TableInfo`, `users map[string]UserInfo`
- `pendingTables map[string]struct{}`, `pendingUsers map[string]struct{}`
- `nextTableID uint64`, `nextUserID uint64`

Implement deep-copy helpers: `copyTableInfo` and `copyUserInfo` that explicitly clone `[]byte` slices.
Implement `loadCache(ctx context.Context) (*cache, error)`:
- Scan `_plomvix_meta` using `math.MaxUint64` as Tx. 
- Look for `meta_key == MetaKeyNextTxID`. Set `c.nextTxID = meta_uint64`.
- Scan `_plomvix_tables` and `_plomvix_users`. Populate maps using deep copies. Track `nextTableID` and `nextUserID`.
Tests: Insert data directly into Heap, call `loadCache`, verify maps are populated, byte slices are independent copies, and `nextTxID` is correctly restored. 
CRITICAL BOOTSTRAP TEST: Fresh bootstrap -> loadCache -> CreateTable succeeds and uses txID=2, not txID=1.

Task 4 — Catalog Start & Stop (Safe Locking & Drain)
Implement `Start(ctx)`:
1. Lock `mu`. If `started`, unlock and return nil. If `starting`, unlock and return `ErrCatalogAlreadyStarted`. Set `starting = true`. Unlock `mu`.
2. Setup deferred failure handler to reset `starting=false` on error.
3. Call `bootstrap(ctx)` (NO LOCK HELD).
4. Call `loadCache(ctx)` (NO LOCK HELD).
5. Lock `mu`. Set `c.cache = newCache`, `c.started = true`, `c.starting = false`. Unlock.

Implement `Stop(ctx)`:
1. Lock `mu`. 
2. If `!started`, unlock and return nil (idempotent).
3. CRITICAL DRAIN CHECK: If `len(cache.pendingTables) > 0` or `len(cache.pendingUsers) > 0`, unlock and return `ErrConflict`. Stop MUST NOT clear the cache while operations are in-flight.
4. Set `started = false`, `starting = false`, `cache = nil`. Unlock.
Tests: Start -> Stop -> Start lifecycle. Stop returns ErrConflict if a CreateTable is in its unlocked Heap I/O phase.

Task 5 — Table Management API (Meta-First & State Re-checks)
Implement `CreateTable(ctx, engineName, tableName, schemaJSON)`:
1. Validate `tableName != ""` and `engineName != ""` (else `ErrEmptyName`).
2. RLock `mu`. Check `started`. Get `engines[engineName]`. RUnlock.
3. Call `eng.ValidateSchema(schemaJSON)` (NO LOCK HELD).
4. Lock `mu` (Write Lock). 
   - CRITICAL STATE RE-CHECK: If `!started || c.cache == nil`, Unlock and return `ErrCatalogNotStarted`. (Prevents panic if Stop() ran during schema validation).
   - Check `cache.tables` and `cache.pendingTables`. If either exists, Unlock and return `ErrDuplicateTable`.
   - Reserve `txID = c.nextTxID + 1`.
   - Add `tableName` to `cache.pendingTables`. `reserved := true`. Unlock `mu`.
5. Setup deferred cleanup for `pendingTables`.
6. META-FIRST PERSISTENCE: Update `_plomvix_meta` row for `MetaKeyNextTxID` to `txID` using `heap.Tx{ID: txID}`. If this fails, return error.
7. TX CONSUMPTION: Lock `mu`. Set `c.nextTxID = txID`. Unlock `mu`. (The TxID is now permanently consumed in-memory, even if the next step fails).
8. HEAP DATA WRITE: `tablesHandle.Insert(ctx, heap.Tx{ID: txID}, rowValues)`.
9. Lock `mu`. Add deep-copied `TableInfo` to `cache.tables`. Unlock.

Implement `DropTable(ctx, tableName)`:
1. Validate `tableName != ""`.
2. Lock `mu` (Write Lock). 
   - CRITICAL STATE RE-CHECK: If `!started || c.cache == nil`, Unlock, return `ErrCatalogNotStarted`.
   - Check `cache.tables`. If missing, Unlock, return `ErrTableNotFound`.
   - If in `cache.pendingTables`, Unlock, return `ErrConflict`.
   - Reserve `txID`. Add to `pendingTables`. Unlock.
3. Setup deferred cleanup.
4. META-FIRST: Update `_plomvix_meta` to `txID` using `heap.Tx{ID: txID}`.
5. TX CONSUMPTION: Lock `mu`. Set `c.nextTxID = txID`. Unlock `mu`.
6. HEAP DATA DELETE: `tablesHandle.Delete(ctx, heap.Tx{ID: txID}, pkValues)`.
7. Lock `mu`. Remove from `cache.tables`. Unlock.

Implement `GetTable(ctx, tableName)`:
1. RLock `mu`. Check `started`. Lookup cache. Deep-copy. RUnlock.
Tests: Create, get, drop. Duplicate fails. Unregistered engine fails. Mutating returned payload doesn't affect cache. Simulate Stop() running during ValidateSchema -> CreateTable safely returns ErrCatalogNotStarted without panicking. Simulate data write failure -> verify nextTxID is still incremented (gapped).

Task 6 — User Management API & Honest Authentication
Implement `CreateUser(ctx, username, password, isAdmin)`:
1. Validate `username != ""`. 
2. Lock `mu` (Write Lock). 
   - CRITICAL STATE RE-CHECK: If `!started || c.cache == nil`, Unlock, return `ErrCatalogNotStarted`.
   - Check `cache.users` and `pendingUsers`. 
   - Reserve `txID`. Add to `pendingUsers`. Unlock.
3. Setup deferred cleanup for `pendingUsers`.
4. Generate 16-byte salt via `crypto/rand`, compute `sha256(salt + password)`.
5. META-FIRST: Update `_plomvix_meta` to `txID` using `heap.Tx{ID: txID}`.
6. TX CONSUMPTION: Lock `mu`. Set `c.nextTxID = txID`. Unlock `mu`.
7. HEAP DATA INSERT: `usersHandle.Insert(ctx, heap.Tx{ID: txID}, rowValues)`.
8. Lock `mu`. Add to `cache.users`. Unlock.

Implement `Authenticate(ctx, username, password)`:
1. RLock `mu`. Check `started` and `cache != nil`. Lookup `UserInfo`. Deep copy. RUnlock.
2. If missing, return `ErrAuthFailed`.
3. Extract salt, compute hash. Use `crypto/subtle.ConstantTimeCompare`.
Tests: Create, auth success, auth wrong password, auth missing user, auth empty password. Duplicate username fails.

Task 7 — Concurrency & Race Testing
Add `go test -race ./internal/catalog/...`.
- 50 goroutines calling `GetTable` concurrently.
- Concurrent `CreateTable` and `DropTable`.
Tests: Verify no races, cache remains consistent.

Task 8 — Edge Cases & Compliance
- Drop non-existent table.
- Empty names rejected.

Task 9 — Benchmarks
- `GetTable` (Cache Hit latency).
- `Authenticate` latency.

Task 10 — docs/catalog.md
Write documentation covering:
- The Global Control Plane architecture and Safe Locking sequences.
- The Meta-First TxID persistence strategy and in-memory gap safety.
- Stop() draining via pending maps and state re-checks.
- Honest Basic authentication.
Add substring-check test in `internal/catalog/docs_test.go`.

Completion criteria
All 10 tasks implemented and tested. `go test -race` passes. The Catalog 
successfully bootstraps on a fresh Heap. DDL operations use the strict 
lock/unlock/defer pattern. TxID is persisted to `_plomvix_meta` BEFORE 
the catalog data row is written, and `c.nextTxID` is updated in-memory 
immediately after the meta write to prevent in-process reuse on data 
write failure. Write-lock reservation phases re-check `!started || cache == nil` 
to prevent nil-pointer panics if `Stop()` executes between phases. `Stop()` 
safely rejects shutdown if operations are pending. All read operations 
return deep copies from the O(1) in-memory cache. Documentation exists.