// Package catalog provides the Global System Catalog, the server-level control
// plane for Plomvix. It manages system metadata, registers pluggable engines,
// provides schema resolution, and handles basic authentication.
//
// The Catalog persists its own system tables via the Enterprise Table Heap
// using a strict Meta-First TxID persistence strategy with safe multi-phase
// locking and Stop() draining.
package catalog

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// System table IDs.
const (
	SystemTableTablesID uint64 = 1
	SystemTableUsersID  uint64 = 2
	SystemTableMetaID   uint64 = 3

	MetaKeyNextTxID = "catalog_next_tx_id"
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
}

// UserInfo holds catalog metadata for a single user.
type UserInfo struct {
	UserID       uint64
	Username     string
	PasswordHash []byte
	IsAdmin      bool
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
}

// Sentinel errors.
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

// catalog is the concrete implementation.
type catalog struct {
	h       *heap.Heap
	mu      sync.RWMutex
	engines map[string]Engine

	started  bool
	starting bool
	cache    *cache

	nextTxID uint64

	tablesHandle heap.Table
	usersHandle  heap.Table
	metaHandle   heap.Table
}

// New creates a new Catalog backed by the given Heap.
func New(h *heap.Heap) Catalog {
	return &catalog{
		h:       h,
		engines: make(map[string]Engine),
	}
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

// genPasswordHash generates a salted SHA-256 hash for a password.
func genPasswordHash(password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("catalog: rand: %w", err)
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	digest := h.Sum(nil)
	return append(salt, digest...), nil
}

// verifyPassword compares a plaintext password against a stored hash.
func verifyPassword(hash []byte, password string) bool {
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

// compile-time check
var _ Catalog = (*catalog)(nil)
