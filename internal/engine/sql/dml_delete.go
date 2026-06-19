package sql

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// execDelete handles DELETE FROM t WHERE ... (enterprise: multi-row + full-table).
func (e *SQLEngine) execDelete(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	del := req.Stmt.RawDelete()
	if del == nil {
		return nil, ErrUnsupportedDML
	}

	// Full-table DELETE gate.
	if del.Where == nil {
		if !req.AllowFullTableDelete {
			return nil, ErrDeleteAllRequiresConfirmation
		}
	}

	tableName, err := extractSingleDMLTableName(del.TableExprs)
	if err != nil {
		return nil, err
	}

	// Resolve schema and heap.
	tableInfo, err := e.catalog.GetTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	engSchema, err := schema.Decode(tableInfo.SchemaPayload)
	if err != nil {
		return nil, fmt.Errorf("sql engine: decode schema: %w", err)
	}
	heapTarget, err := e.tables.GetTableHeap(tableInfo.TableID)
	if err != nil {
		return nil, fmt.Errorf("sql engine: get table heap: %w", err)
	}

	mutHeap, ok := heapTarget.(*tableHeapAdapter)
	if !ok {
		return nil, ErrHeapMutationUnsupported
	}

	// Collect all matching rows (or all rows for full-table).
	matched, err := e.collectMatchingRowsEnterprise(ctx, req, heapTarget, &engSchema, del.Where)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, ErrRowNotFound
	}

	// Build mutations.
	mutations := make([]RowMutation, len(matched))
	for i, row := range matched {
		if row.RowID == 0 {
			return nil, ErrMissingRowID
		}
		mutations[i] = RowMutation{RowID: row.RowID, Op: OpDelete}
	}

	rowsAffected, err := AsMutable(mutHeap).BatchMutate(ctx, req.TxContext, mutations)
	if err != nil {
		e.log.Warn("dml: DELETE mutation failed",
			"table", tableName,
			"rows_succeeded", rowsAffected,
			"total_matched", len(matched),
			"write_tx_id", req.TxContext.WriteTxID,
			"error", err.Error(),
		)
		return nil, err
	}

	e.log.Info("dml: DELETE",
		"table", tableName,
		"rows_affected", rowsAffected,
		"write_tx_id", req.TxContext.WriteTxID,
		"conflict_checked", true,
	)

	return &engine.Result{
		Stream:       nil,
		RowsAffected: int64(rowsAffected),
		Message:      fmt.Sprintf("DELETE %d", rowsAffected),
	}, nil
}

// extractSingleDMLTableName extracts the target table name from Vitess TableExprs.
func extractSingleDMLTableName(exprs vitess.TableExprs) (string, error) {
	if len(exprs) != 1 {
		return "", ErrUnsupportedDML
	}
	aliased, ok := exprs[0].(*vitess.AliasedTableExpr)
	if !ok {
		return "", ErrUnsupportedDML
	}
	tname, ok := aliased.Expr.(vitess.TableName)
	if !ok {
		return "", ErrUnsupportedDML
	}
	name := tname.Name.String()
	if name == "" {
		return "", ErrUnsupportedDML
	}
	return name, nil
}

// collectMatchingRowsEnterprise collects all rows, respecting maxMutationRows.
func (e *SQLEngine) collectMatchingRowsEnterprise(ctx context.Context, req *engine.Request, heapTarget planner.TableHeap, engSchema *engine.Schema, where *vitess.Where) ([]engine.Row, error) {
	scanNode := planner.NewSeqScanNode(heapTarget, e.decoder, *engSchema, req.TxContext)
	var source planner.Operator = scanNode

	if where != nil {
		boundWhere, err := planner.BindWhere(where.Expr, *engSchema)
		if err != nil {
			if errors.Is(err, planner.ErrUnsupportedFeature) {
				return nil, ErrUnsupportedWhereExpr
			}
			return nil, err
		}
		source = planner.NewFilterNode(scanNode, boundWhere)
	}

	if err := source.Open(ctx); err != nil {
		return nil, err
	}
	defer source.Close()

	var matched []engine.Row
	for {
		row, err := source.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		matched = append(matched, row.DeepCopy())
		if e.maxMutationRows > 0 && len(matched) > e.maxMutationRows {
			return nil, ErrMutationLimitExceeded
		}
	}
	return matched, nil
}
