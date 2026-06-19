// Package catalog provides the Global System Catalog, the server-level control
// plane for Plomvix. It manages system metadata, RBAC, schema versioning,
// audit logging, and bcrypt-based authentication.
//
// Enterprise tier additions: RBAC roles/grants, bcrypt passwords with legacy
// SHA256 migration, immutable audit log, schema history tracking, and
// immediate in-memory TxID reservation.
package catalog

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"sync"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"golang.org/x/crypto/bcrypt"
)

// System table IDs.
const (
	SystemTableTablesID        uint64 = 1
	SystemTableUsersID         uint64 = 2
	SystemTableMetaID          uint64 = 3
	SystemTableRolesID         uint64 = 4
	SystemTableGrantsID        uint64 = 5
	SystemTableUserRolesID     uint64 = 6
	SystemTableSchemaHistoryID uint64 = 7
	SystemTableAuditLogID      uint64 = 8

	MetaKeyNextTxID = "catalog_next_tx_id"
)

// Action represents an RBAC action type.
type Action string

const (
	ActionRead  Action = "READ"
	ActionWrite Action = "WRITE"
	ActionDDL   Action = "DDL"
)

// System table schemas.
var (
	schemaTables = heap.Schema{
		TableID: SystemTableTablesID,
		Columns: []heap.Column{
			{Name: "table_id", Kind: key.KindUint64},
			{Name: "engine_name", Kind: key.KindString},
			{Name: "table_name", Kind: key.KindString},
			{Name: "schema_payload", Kind: key.KindBytes},
			{Name: "schema_version", Kind: key.KindUint64},
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

	schemaRoles = heap.Schema{
		TableID: SystemTableRolesID,
		Columns: []heap.Column{
			{Name: "role_id", Kind: key.KindUint64},
			{Name: "role_name", Kind: key.KindString},
		},
		PKIndices: []int{0},
	}

	schemaGrants = heap.Schema{
		TableID: SystemTableGrantsID,
		Columns: []heap.Column{
			{Name: "grant_id", Kind: key.KindUint64},
			{Name: "role_id", Kind: key.KindUint64},
			{Name: "table_id", Kind: key.KindUint64},
			{Name: "action", Kind: key.KindString},
		},
		PKIndices: []int{0},
	}

	schemaUserRoles = heap.Schema{
		TableID: SystemTableUserRolesID,
		Columns: []heap.Column{
			{Name: "user_role_id", Kind: key.KindUint64},
			{Name: "user_id", Kind: key.KindUint64},
			{Name: "role_id", Kind: key.KindUint64},
		},
		PKIndices: []int{0},
	}

	schemaSchemaHistory = heap.Schema{
		TableID: SystemTableSchemaHistoryID,
		Columns: []heap.Column{
			{Name: "history_id", Kind: key.KindUint64},
			{Name: "table_id", Kind: key.KindUint64},
			{Name: "version", Kind: key.KindUint64},
			{Name: "action", Kind: key.KindString},
			{Name: "schema_payload", Kind: key.KindBytes},
			{Name: "timestamp", Kind: key.KindUint64},
		},
		PKIndices: []int{0},
	}

	schemaAuditLog = heap.Schema{
		TableID: SystemTableAuditLogID,
		Columns: []heap.Column{
			{Name: "audit_id", Kind: key.KindUint64},
			{Name: "tx_id", Kind: key.KindUint64},
			{Name: "user_id", Kind: key.KindUint64},
			{Name: "action", Kind: key.KindString},
			{Name: "target_type", Kind: key.KindString},
			{Name: "target_name", Kind: key.KindString},
			{Name: "timestamp", Kind: key.KindUint64},
		},
		PKIndices: []int{0},
	}
)

// Engine is a pluggable backend engine registered with the catalog.
type Engine interface {
	Name() string
	ValidateSchema(schemaJSON []byte) error
}

// TableInfo holds catalog metadata for a single user table.
type TableInfo struct {
	TableID       uint64
	EngineName    string
	TableName     string
	SchemaPayload []byte
	SchemaVersion uint64
}

// UserInfo holds catalog metadata for a single user.
type UserInfo struct {
	UserID       uint64
	Username     string
	PasswordHash []byte
	IsAdmin      bool
}

// RoleInfo holds RBAC role metadata.
type RoleInfo struct {
	RoleID   uint64
	RoleName string
}

// GrantInfo holds a single RBAC grant.
type GrantInfo struct {
	GrantID uint64
	RoleID  uint64
	TableID uint64
	Action  Action
}

// SchemaHistoryEntry records a schema change event.
type SchemaHistoryEntry struct {
	HistoryID     uint64
	TableID       uint64
	Version       uint64
	Action        string
	SchemaPayload []byte
	Timestamp     uint64
}

// Catalog is the global system catalog interface.
type Catalog interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterEngine(e Engine) error
	CreateTable(ctx context.Context, engineName, tableName string, schemaJSON []byte) error
	DropTable(ctx context.Context, tableName string) error
	GetTable(ctx context.Context, tableName string) (TableInfo, error)
	CreateUser(ctx context.Context, username, password string, isAdmin bool) error
	Authenticate(ctx context.Context, username, password string) (UserInfo, error)

	// DDL lifecycle (safe ordering: allocate → physical create → register).
	AllocateTableID(ctx context.Context) (uint64, error)
	RegisterTable(ctx context.Context, tableID uint64, engineName, tableName string, schemaPayload []byte) error
	// CheckGlobalPermission checks if user can perform a global action (e.g. DDL).
	CheckGlobalPermission(ctx context.Context, userID uint64, action Action) (bool, error)

	// Enterprise RBAC.
	CreateRole(ctx context.Context, roleName string) error
	DropRole(ctx context.Context, roleName string) error
	AssignRole(ctx context.Context, username, roleName string) error
	RevokeRole(ctx context.Context, username, roleName string) error
	Grant(ctx context.Context, roleName, tableName string, action Action) error
	Revoke(ctx context.Context, roleName, tableName string, action Action) error
	CheckPermission(ctx context.Context, userID, tableID uint64, action Action) (bool, error)

	// Enterprise schema history.
	GetSchemaHistory(ctx context.Context, tableName string) ([]SchemaHistoryEntry, error)

	// SchemaVersion returns the current DDL version counter.
	SchemaVersion() uint64
}

// Sentinel errors.
var (
	ErrEngineNotFound          = errors.New("catalog: engine not registered")
	ErrDuplicateEngine         = errors.New("catalog: engine already registered")
	ErrInvalidEngine           = errors.New("catalog: invalid engine (nil or empty name)")
	ErrTableNotFound           = errors.New("catalog: table not found")
	ErrDuplicateTable          = errors.New("catalog: table name already exists")
	ErrDuplicateUser           = errors.New("catalog: username already exists")
	ErrInvalidSchema           = errors.New("catalog: engine rejected schema payload")
	ErrAuthFailed              = errors.New("catalog: authentication failed")
	ErrCatalogNotStarted       = errors.New("catalog: not started")
	ErrCatalogAlreadyStarted   = errors.New("catalog: already started or starting")
	ErrEmptyName               = errors.New("catalog: name cannot be empty")
	ErrConflict                = errors.New("catalog: concurrent operation conflict")
	ErrRoleNotFound            = errors.New("catalog: role not found")
	ErrDuplicateRole           = errors.New("catalog: role name already exists")
	ErrPermissionDenied        = errors.New("catalog: permission denied")
	ErrInvalidAction           = errors.New("catalog: invalid RBAC action")
	ErrDuplicateRoleAssignment = errors.New("catalog: user already has this role")
	ErrDuplicateGrant          = errors.New("catalog: grant already exists")
	ErrGrantNotFound           = errors.New("catalog: grant not found")
)

// catalog is the concrete implementation.
type catalog struct {
	h       *heap.Heap
	mu      sync.RWMutex
	engines map[string]Engine

	started  bool
	starting bool
	cache    *cache

	nextTxID uint64

	tablesHandle    heap.Table
	usersHandle     heap.Table
	metaHandle      heap.Table
	rolesHandle     heap.Table
	grantsHandle    heap.Table
	userRolesHandle heap.Table
	historyHandle   heap.Table
	auditHandle     heap.Table

	schemaVersion uint64
}

// New creates a new Catalog backed by the given Heap.
func New(h *heap.Heap) Catalog {
	return &catalog{
		h:       h,
		engines: make(map[string]Engine),
	}
}

// reserveTx increments nextTxID immediately under lock and returns the new value.
// Must be called while holding c.mu (Write Lock).
func (c *catalog) reserveTx() uint64 {
	c.nextTxID++
	return c.nextTxID
}

// RegisterEngine registers a pluggable engine. Must be called before Start().
func (c *catalog) RegisterEngine(e Engine) error {
	if e == nil || e.Name() == "" {
		return ErrInvalidEngine
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started || c.starting {
		return ErrCatalogAlreadyStarted
	}
	if _, ok := c.engines[e.Name()]; ok {
		return ErrDuplicateEngine
	}
	c.engines[e.Name()] = e
	return nil
}

// isValidAction checks if an Action string is one of the valid RBAC actions.
func isValidAction(a Action) bool {
	return a == ActionRead || a == ActionWrite || a == ActionDDL
}

// genBcryptHash generates a bcrypt hash with cost 10.
func genBcryptHash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), 10)
}

// isLegacyHash returns true if the hash is a legacy SHA256 (not bcrypt).
// bcrypt hashes start with "$2a$".
func isLegacyHash(hash []byte) bool {
	return len(hash) < 4 || string(hash[:4]) != "$2a$"
}

// verifyPassword compares a plaintext password against a stored hash.
// Supports bcrypt and legacy SHA256.
func verifyPassword(hash []byte, password string) bool {
	if len(hash) == 0 {
		return false
	}
	if isLegacyHash(hash) {
		if len(hash) < 16 {
			return false
		}
		salt := hash[:16]
		expected := hash[16:]
		h := sha256.New()
		h.Write(salt)
		h.Write([]byte(password))
		actual := h.Sum(nil)
		return subtle.ConstantTimeCompare(expected, actual) == 1
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

// compile-time check
var _ Catalog = (*catalog)(nil)
