// Package router provides the Global Query Router for dispatching parsed
// SQL statements to the correct query execution engine. It validates
// statements, resolves table metadata via the catalog, checks permissions,
// and ensures all tables belong to the same engine.
package router

import (
	"context"
	"errors"
	"math"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

// Sentinel errors.
var (
	ErrCrossEngineJoinNotSupported = errors.New("router: cross-engine joins are not supported")
	ErrEngineNotFound              = errors.New("router: engine not found")
	ErrPermissionDenied            = errors.New("router: permission denied")
	ErrUnsupportedStatement        = errors.New("router: statement type not supported in basic tier")
	ErrNoTargetTable               = errors.New("router: no target table found")
)

// Router dispatches parsed statements to registered engines.
type Router struct {
	catalog catalog.Catalog
	engines map[string]engine.Engine
}

// New creates a new Router with the given catalog and engine registry.
func New(cat catalog.Catalog) *Router {
	return &Router{
		catalog: cat,
		engines: make(map[string]engine.Engine),
	}
}

// RegisterEngine adds an engine to the router's dispatch table.
func (r *Router) RegisterEngine(e engine.Engine) {
	r.engines[e.Name()] = e
}

// Route validates the statement, resolves tables, checks permissions, and
// dispatches to the appropriate engine.
func (r *Router) Route(ctx context.Context, userID uint64, stmt sqlparser.Statement) (engine.RowStream, error) {
	if stmt.Type() != sqlparser.StmtSelect {
		return nil, ErrUnsupportedStatement
	}

	tables := stmt.TargetTables()
	if len(tables) == 0 {
		return nil, ErrNoTargetTable
	}

	var targetEngine string
	for _, tableName := range tables {
		info, err := r.catalog.GetTable(ctx, tableName)
		if err != nil {
			return nil, err
		}

		ok, err := r.catalog.CheckPermission(ctx, userID, info.TableID, catalog.ActionRead)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrPermissionDenied
		}

		if targetEngine == "" {
			targetEngine = info.EngineName
		} else if targetEngine != info.EngineName {
			return nil, ErrCrossEngineJoinNotSupported
		}
	}

	eng, ok := r.engines[targetEngine]
	if !ok {
		return nil, ErrEngineNotFound
	}

	return eng.Execute(ctx, &engine.Request{
		Stmt:      stmt,
		UserID:    userID,
		TxContext: engine.TxContext{ReadTxID: math.MaxUint64},
	})
}
