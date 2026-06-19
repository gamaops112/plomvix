package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
)

// execUpdate handles UPDATE t SET ... WHERE ... (enterprise: multi-row).
func (e *SQLEngine) execUpdate(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	upd := req.Stmt.RawUpdate()
	if upd == nil {
		return nil, ErrUnsupportedDML
	}

	// WHERE is required for UPDATE (unchanged from setup plan).
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
		if _, ok := schemaIndexByName[colName]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownColumn, colName)
		}
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

	// --- 3. Collect matching rows via Volcano (enterprise: multi-row) ---
	matched, err := e.collectMatchingRowsEnterprise(ctx, req, heapTarget, &engSchema, upd.Where)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, ErrRowNotFound
	}

	// --- 4. Build RowMutation list ---
	mutations := make([]RowMutation, len(matched))
	for i, row := range matched {
		if row.RowID == 0 {
			return nil, ErrMissingRowID
		}
		newValues := make([]engine.Datum, len(row.Datums))
		copy(newValues, row.Datums)
		for colName, datum := range setByName {
			idx := schemaIndexByName[colName]
			newValues[idx] = datum
		}
		mutations[i] = RowMutation{RowID: row.RowID, Op: OpUpdate, NewValues: newValues}
	}

	mh := AsMutable(mutHeap)
	rowsAffected, err := mh.BatchMutate(ctx, req.TxContext, mutations)
	if err != nil {
		e.log.Warn("dml: UPDATE mutation failed",
			"table", tableName,
			"rows_succeeded", rowsAffected,
			"total_matched", len(matched),
			"write_tx_id", req.TxContext.WriteTxID,
			"error", err.Error(),
		)
		return nil, err
	}

	e.log.Info("dml: UPDATE",
		"table", tableName,
		"rows_affected", rowsAffected,
		"write_tx_id", req.TxContext.WriteTxID,
		"conflict_checked", true,
	)

	return &engine.Result{
		Stream:       nil,
		RowsAffected: int64(rowsAffected),
		Message:      fmt.Sprintf("UPDATE %d", rowsAffected),
	}, nil
}
