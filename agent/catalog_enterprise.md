catalog_enterprise.md — Global System Catalog (Enterprise Hardening)
Scope
This plan hardens the Global System Catalog (`catalog` package) delivered 
in `catalog_setup.md`. It introduces Role-Based Access Control (RBAC), 
schema versioning with history tracking, production-grade `bcrypt` 
authentication with automatic legacy migration, and immutable DDL audit 
logging.

Prerequisite: Basic Plan Amendment
Before coding EITHER `catalog_setup.md` or this plan, the `schemaTables` 
definition in `catalog_setup.md` MUST be updated to include:
`{Name: "schema_version", Kind: key.KindUint64}`. 
This avoids runtime schema migrations. Basic `CreateTable` will default 
this to `1`.

Contract this tier honestly provides (read this before writing code)
- Logical Correlation, NOT Cross-Table Atomicity: A single Catalog operation 
  uses the exact same `txID` to write to multiple system tables. This 
  provides a shared logical version for correlation. It does NOT provide 
  atomic rollback. If a cascade delete fails during `DropRole`, the role 
  is still dropped but orphaned grants may remain. True cross-table atomic 
  transactions are deferred.
- Immediate In-Memory Tx Reservation: To prevent concurrent operations from 
  reserving the same `txID` (which would cause a Heap monotonic conflict 
  on `_plomvix_meta`), `c.nextTxID` is incremented in-memory *immediately* 
  upon reservation while holding the write lock, before any I/O occurs. 
  If the subsequent meta or data write fails, the TxID is safely "gapped" 
  in-memory. On restart, the Catalog reloads the last durable meta value 
  from disk.
- Orphan-Tolerant RBAC: Because cascade deletes are best-effort, `CheckPermission` 
  explicitly ignores `roleID`s that are no longer present in `rolesByID`. 
  This ensures orphaned rows left behind by a partial `DropRole` failure 
  are completely harmless.
- Production Authentication: Passwords are hashed using `golang.org/x/crypto/bcrypt`. 
  Legacy SHA256 hashes are transparently migrated on successful login using 
  the strict DDL safe-locking pattern, with explicit conflict handling for 
  concurrent migrations.
- Global Grants: Passing an empty string (`""`) for `tableName` in `Grant` 
  or `Revoke` resolves to `tableID = 0`, representing a global permission.
- Immutable Audit & History: `_plomvix_audit_log` and `_plomvix_schema_history` 
  are append-only. Cascade deletes are internal and do NOT generate separate 
  audit entries.
- System Actor: All Catalog DDL methods use `user_id=0` (System) for audit logs.

Allowed Dependencies
- `golang.org/x/crypto/bcrypt` (Explicitly allowed. `go mod tidy` must 
  verify no other new dependencies are added).

Constants & System Table Definitions (Additions)
package catalog

import "golang.org/x/crypto/bcrypt"

const (
    SystemTableRolesID         uint64 = 4
    SystemTableGrantsID        uint64 = 5
    SystemTableUserRolesID     uint64 = 6
    SystemTableSchemaHistoryID uint64 = 7
    SystemTableAuditLogID      uint64 = 8
)

type Action string
const (
    ActionRead  Action = "READ"
    ActionWrite Action = "WRITE"
    ActionDDL   Action = "DDL"
)

// New System Table Schemas (roles, grants, user_roles, history, audit)
var (
    schemaRoles = heap.Schema{ /* ... */ }
    schemaGrants = heap.Schema{ /* ... */ }
    schemaUserRoles = heap.Schema{ /* ... */ }
    schemaHistory = heap.Schema{ /* ... */ }
    schemaAudit = heap.Schema{ /* ... */ }
)

Public API Additions
package catalog

type RoleInfo struct { RoleID uint64; RoleName string }
type GrantInfo struct { GrantID uint64; RoleID uint64; TableID uint64; Action Action }
type SchemaHistoryEntry struct { HistoryID uint64; TableID uint64; Version uint64; Action string; SchemaPayload []byte; Timestamp uint64 }

type Catalog interface {
    // ... Basic methods ...
    CreateRole(ctx context.Context, roleName string) error
    DropRole(ctx context.Context, roleName string) error
    AssignRole(ctx context.Context, username, roleName string) error
    RevokeRole(ctx context.Context, username, roleName string) error
    Grant(ctx context.Context, roleName, tableName string, action Action) error
    Revoke(ctx context.Context, roleName, tableName string, action Action) error
    CheckPermission(ctx context.Context, userID, tableID uint64, action Action) (bool, error)
    GetSchemaHistory(ctx context.Context, tableName string) ([]SchemaHistoryEntry, error)
}

var (
    // ... Basic errors ...
    ErrRoleNotFound           = errors.New("catalog: role not found")
    ErrDuplicateRole          = errors.New("catalog: role name already exists")
    ErrPermissionDenied       = errors.New("catalog: permission denied")
    ErrInvalidAction          = errors.New("catalog: invalid RBAC action")
    ErrDuplicateRoleAssignment = errors.New("catalog: user already has this role")
    ErrDuplicateGrant         = errors.New("catalog: grant already exists")
    ErrGrantNotFound          = errors.New("catalog: grant not found")
)

Tasks (do in order, one at a time)

Task 1 — Package updates, Schemas, Cache Extensions, and Bootstrap Handles
Update `internal/catalog/catalog.go` with new constants, errors, API methods, and structs.
Add new `heap.Table` handles to the `catalog` struct:
- `rolesHandle`, `grantsHandle`, `userRolesHandle`, `historyHandle`, `auditHandle`.

Implement `reserveTx() uint64`: 
- MUST be called while holding `c.mu` (Write Lock). 
- Increments `c.nextTxID` immediately and returns the new value. 
- This prevents concurrent operations from reserving the same TxID.

Update `internal/catalog/cache.go` to add maps, reverse-lookups, and counters:
- `roles map[string]RoleInfo`, `rolesByID map[uint64]RoleInfo`
- `grants map[uint64][]GrantInfo` (keyed by role_id)
- `grantsByKey map[grantKey]GrantInfo` (where `grantKey` = `{roleID, tableID, action}`)
- `userRoles map[uint64][]uint64` (keyed by user_id)
- `userRoleAssignments map[userRoleKey]uint64` (where `userRoleKey` = `{userID, roleID}` -> `user_role_id`)
- `usersByID map[uint64]UserInfo`
- Counters: `nextRoleID`, `nextGrantID`, `nextUserRoleID`, `nextHistoryID`, `nextAuditLogID`.

Update `bootstrap.go`:
- Call `h.OpenTable` for the 5 new system tables and store the handles.

Update `loadCache`:
- Scan the 5 new system tables. Populate reverse-lookup maps.
Tests: Verify bootstrap opens all handles. Verify cache loads correctly.

Task 2 — Bcrypt Authentication & Safe Legacy Migration
Update `CreateUser` to use `bcrypt.GenerateFromPassword` (cost 10).
Update `Authenticate` to attempt bcrypt, then fallback to legacy SHA256 check.
If legacy match succeeds, call `migrateLegacyPassword(ctx, user, password)`.

Implement `migrateLegacyPassword`:
- Lock `mu`. Check `started` and `cache != nil`.
- CRITICAL CONFLICT CHECK: If `username` is already in `pendingUsers`, unlock and return `ErrConflict`.
- `txID := c.reserveTx()` (Increments immediately under lock).
- Add `username` to `pendingUsers`. Unlock.
- Defer `pendingUsers` cleanup.
- Generate bcrypt hash.
- META-FIRST: Update `_plomvix_meta` to `txID` using `heap.Tx{ID: txID}`.
- HEAP UPDATE: `usersHandle.Update(ctx, heap.Tx{ID: txID}, pkValues, newValues)`.
- Lock `mu`. RE-CHECK `started && cache != nil`. Update cache, unlock.
Tests: Bcrypt create/auth. Manual legacy SHA256 insert -> auth success -> verify upgraded. Concurrent legacy logins -> one succeeds, other gets ErrConflict.

Task 3 — RBAC DDL (Roles, Assignments, and Cascading Deletes)
Implement `CreateRole`, `AssignRole`, `DropRole`, `RevokeRole`.
CRITICAL: These MUST follow the safe-locking pattern:
1. Lock `mu`. Re-check state.
2. `txID := c.reserveTx()` (Increments immediately).
3. Add to `pending...` map. Unlock.
4. Defer cleanup.
5. META-FIRST: Update `_plomvix_meta` to `txID`.
6. HEAP WRITES: Insert/Delete using `heap.Tx{ID: txID}`.
7. Lock `mu`, update cache, unlock.

`DropRole` specifics:
- Scan `cache.grantsByKey` and `cache.userRoleAssignments` to find associated PKs.
- Best-effort delete the role, grants, and user_roles using the shared `txID`. 
- Append exactly ONE audit entry for `DropRole`. Cascade deletes are silent.

Tests: 
- Normal case: Create, assign, drop role. Verify cascade deletes. 
- Partial failure case: Simulate cascade delete failure. Verify role is dropped from `rolesByID`, and `CheckPermission` ignores orphaned rows.
- Concurrent CreateRole/CreateUser/CreateTable must never reserve the same txID (verify via audit log or meta table history).

Task 4 — RBAC Grants & Permission Checking
Implement `Grant` and `Revoke` using the same immediate-reservation safe-locking pattern.
- `Grant`: If `tableName == ""`, resolve to `tableID = 0` (Global). Check `cache.grantsByKey` to prevent `ErrDuplicateGrant`.
- `Revoke`: Resolve `grantKey`. Lookup in `cache.grantsByKey` to find `GrantID` (PK). If missing, return `ErrGrantNotFound`.

Implement `CheckPermission(ctx, userID, tableID, action)`:
1. Validate `action`. If invalid, return `false, ErrInvalidAction`.
2. RLock `mu`. **Defer RUnlock `mu`**.
3. Check `started`.
4. Lookup user in `cache.usersByID[userID]`. If `IsAdmin`, return `true, nil`.
5. Fetch `roleIDs` from `cache.userRoles[userID]`.
6. For each `roleID`:
   - If `roleID` is NOT in `cache.rolesByID`, skip it (handles orphans).
   - Check `cache.grants[roleID]` for a matching `(tableID, action)` or global `(0, action)`.
   - If match found, return `true, nil`.
7. Return `false, nil`.

Tests: Grant, Revoke, CheckPermission. Global grant works. Invalid action returns ErrInvalidAction.

Task 5 — Schema Versioning & History Integration
Update `CreateTable` and `DropTable` (from Basic plan) to use the immediate-reservation pattern.
- Inside the unlocked Heap I/O phase, use the shared `txID` to insert into `_plomvix_schema_history`.
Implement `GetSchemaHistory` (O(N) full scan, filtered in memory).
- Return deep copies of `SchemaPayload` byte slices.
Tests: Create, drop. Verify history. Mutating returned payload doesn't affect state.

Task 6 — Audit Logging Integration
Verify ALL DDL methods append to `_plomvix_audit_log` using `user_id=0` and the shared `txID`.
Tests: Verify exactly one audit entry per top-level DDL call.

Task 7 — Concurrency & Race Testing
`go test -race`. Concurrent `CheckPermission`, `Grant`, `Revoke`.
Verify concurrent DDL operations generate strictly monotonic, unique TxIDs in the audit log.

Task 8 — Edge Cases & Compliance
- Invalid action string.
- Assign non-existent role.
- Revoke non-existent grant / role assignment.

Task 9 — Benchmarks
- `CheckPermission` latency.
- `DropRole` with 100 cascading grants.

Task 10 — docs/catalog.md update & Dependency Verification
Update docs. Run `go mod tidy` to verify only `bcrypt` was added. Update substring-check test.

Completion criteria
All 10 tasks implemented and tested. `go test -race` passes. Bootstrap opens all 5 new table handles. Reverse-lookup maps allow precise PK resolution. `CheckPermission` uses `defer RUnlock` and skips orphaned roles. Tx reservation increments `c.nextTxID` immediately under lock, preventing concurrent TxID collisions. Legacy migration handles concurrent logins. `go mod tidy` confirms only bcrypt dependency. Documentation updated.