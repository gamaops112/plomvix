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

// execDelete handles DELETE FROM t WHERE ...
func (e *SQLEngine) execDelete(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	del := req.Stmt.RawDelete()
	if del == nil {
		return nil, ErrUnsupportedDML
	}

	// WHERE is required.
	if del.Where == nil {
		return nil, ErrWhereRequired
	}

	tableName, err := extractSingleDMLTableName(del.TableExprs)
	if err != nil {
		return nil, err
	}

	// Resolve schema, heap, and bind WHERE.
	heapTarget, engSchema, err := e.resolveAndBindWhere(ctx, tableName, del.Where.Expr)
	if err != nil {
		return nil, err
	}

	mutHeap, ok := heapTarget.(*tableHeapAdapter)
	if !ok {
		return nil, ErrHeapMutationUnsupported
	}

	// Collect matching rows via Volcano pipeline.
	matched, err := e.collectMatchingRows(ctx, req, heapTarget, &engSchema, del.Where.Expr)
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

	if err := AsMutable(mutHeap).DeleteByRowID(ctx, req.TxContext, target.RowID); err != nil {
		return nil, fmt.Errorf("sql engine: delete: %w", err)
	}

	return &engine.Result{
		Stream:       nil,
		RowsAffected: 1,
		Message:      "DELETE 1",
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

// resolveAndBindWhere resolves the table schema, gets the heap, and binds the WHERE clause.
func (e *SQLEngine) resolveAndBindWhere(ctx context.Context, tableName string, where vitess.Expr) (planner.TableHeap, engine.Schema, error) {
	tableInfo, err := e.catalog.GetTable(ctx, tableName)
	if err != nil {
		return nil, engine.Schema{}, err
	}
	engSchema, err := schema.Decode(tableInfo.SchemaPayload)
	if err != nil {
		return nil, engine.Schema{}, fmt.Errorf("sql engine: decode schema: %w", err)
	}
	heapTarget, err := e.tables.GetTableHeap(tableInfo.TableID)
	if err != nil {
		return nil, engine.Schema{}, fmt.Errorf("sql engine: get table heap: %w", err)
	}
	return heapTarget, engSchema, nil
}

// collectMatchingRows builds the Volcano pipeline, opens it, and collects all matching rows.
func (e *SQLEngine) collectMatchingRows(ctx context.Context, req *engine.Request, heapTarget planner.TableHeap, engSchema *engine.Schema, where vitess.Expr) ([]engine.Row, error) {
	boundWhere, err := planner.BindWhere(where, *engSchema)
	if err != nil {
		if errors.Is(err, planner.ErrUnsupportedFeature) {
			return nil, ErrUnsupportedWhereExpr
		}
		return nil, err
	}

	scanNode := planner.NewSeqScanNode(heapTarget, e.decoder, *engSchema, req.TxContext)
	filterNode := planner.NewFilterNode(scanNode, boundWhere)

	if err := filterNode.Open(ctx); err != nil {
		return nil, err
	}
	defer filterNode.Close()

	var matched []engine.Row
	for {
		row, err := filterNode.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		matched = append(matched, row.DeepCopy())
	}
	return matched, nil
}
