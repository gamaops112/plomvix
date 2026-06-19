package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
)

// execUpdate handles UPDATE t SET ... WHERE ...
func (e *SQLEngine) execUpdate(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	upd := req.Stmt.RawUpdate()
	if upd == nil {
		return nil, ErrUnsupportedDML
	}

	// WHERE is required.
	if upd.Where == nil {
		return nil, ErrWhereRequired
	}

	tableName, err := extractSingleDMLTableName(upd.TableExprs)
	if err != nil {
		return nil, err
	}

	// --- 1. Validate SET assignments BEFORE any I/O ---
	tableInfo, err := e.catalog.GetTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	engSchema, err := schema.Decode(tableInfo.SchemaPayload)
	if err != nil {
		return nil, fmt.Errorf("sql engine: decode schema: %w", err)
	}

	// Build name→index map.
	schemaIndexByName := make(map[string]int, len(engSchema.Columns))
	for i, col := range engSchema.Columns {
		schemaIndexByName[strings.ToLower(col.Name)] = i
	}

	// Build setByName and validate SET expressions.
	setByName := make(map[string]engine.Datum, len(upd.Exprs))
	for _, ue := range upd.Exprs {
		colName := strings.ToLower(ue.Name.Name.String())
		if _, dup := setByName[colName]; dup {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateColumn, colName)
		}
		// Validate column exists in schema.
		if _, ok := schemaIndexByName[colName]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownColumn, colName)
		}
		// Map literal value using the same literal switch as INSERT.
		col := engSchema.Columns[schemaIndexByName[colName]]
		d, err := mapLiteral(ue.Expr, col)
		if err != nil {
			return nil, err
		}
		setByName[colName] = d
	}

	// --- 2. Resolve heap and assert mutability ---
	heapTarget, err := e.tables.GetTableHeap(tableInfo.TableID)
	if err != nil {
		return nil, fmt.Errorf("sql engine: get table heap: %w", err)
	}
	mutHeap, ok := heapTarget.(*tableHeapAdapter)
	if !ok {
		return nil, ErrHeapMutationUnsupported
	}

	// --- 3. Collect matching rows via Volcano ---
	matched, err := e.collectMatchingRows(ctx, req, heapTarget, &engSchema, upd.Where.Expr)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, ErrRowNotFound
	}
	if len(matched) > 1 {
		return nil, ErrMultiRowMutationUnsupported
	}

	target := matched[0]
	if target.RowID == 0 {
		return nil, ErrMissingRowID
	}

	// --- 4. Apply SET values to build new row ---
	newValues := make([]engine.Datum, len(target.Datums))
	copy(newValues, target.Datums)
	for colName, d := range setByName {
		idx := schemaIndexByName[colName]
		newValues[idx] = d
	}

	mh := AsMutable(mutHeap)
	if err := mh.UpdateByRowID(ctx, req.TxContext, target.RowID, newValues); err != nil {
		return nil, fmt.Errorf("sql engine: update: %w", err)
	}

	return &engine.Result{
		Stream:       nil,
		RowsAffected: 1,
		Message:      "UPDATE 1",
	}, nil
}
