package sql

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// InsertableTableHeap is the local contract required by the engine for
// appending rows. The physical TableHeap adapter must satisfy this.
type InsertableTableHeap interface {
	Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error
}

// execInsert parses a Vitess INSERT AST, resolves the schema, maps literals
// to engine.Datum values, and appends a single row to the target heap.
func (e *SQLEngine) execInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	insert := req.Stmt.RawInsert()
	if insert == nil {
		return nil, ErrUnsupportedDML
	}

	// --- 1. AST shape validation ---
	values, ok := insert.Rows.(vitess.Values)
	if !ok {
		return nil, ErrInsertSelectUnsupported
	}
	if len(values) != 1 {
		return nil, ErrBatchInsertUnsupported
	}
	rowExprs := values[0] // ValTuple = []Expr

	// --- 2. Resolve schema & heap ---
	tableName := insert.Table.TableNameString()
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

	insertHeap, ok := heapTarget.(InsertableTableHeap)
	if !ok {
		return nil, ErrHeapInsertUnsupported
	}

	// --- 3. Column alignment ---
	// Build schema index by name.
	schemaIndexByName := make(map[string]int, len(engSchema.Columns))
	for i, col := range engSchema.Columns {
		schemaIndexByName[strings.ToLower(col.Name)] = i
	}

	// Initialize mapped row with typed NULL for every schema column.
	mappedRow := make(engine.Row, len(engSchema.Columns))
	for i, col := range engSchema.Columns {
		mappedRow[i] = engine.Datum{Type: col.Type, Value: nil}
	}

	if len(insert.Columns) > 0 {
		// Column list provided: INSERT INTO t (a,b) VALUES (1,2)
		if len(insert.Columns) != len(rowExprs) {
			return nil, ErrColumnCountMismatch
		}
		seen := make(map[string]bool)
		for i, colIdent := range insert.Columns {
			colName := strings.ToLower(colIdent.String())
			if seen[colName] {
				return nil, fmt.Errorf("%w: %q", ErrDuplicateColumn, colName)
			}
			seen[colName] = true
			idx, ok := schemaIndexByName[colName]
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrUnknownColumn, colName)
			}
			d, err := mapLiteral(rowExprs[i], engSchema.Columns[idx])
			if err != nil {
				return nil, err
			}
			mappedRow[idx] = d
		}
	} else {
		// No column list: map by schema order.
		if len(rowExprs) != len(engSchema.Columns) {
			return nil, ErrColumnCountMismatch
		}
		for i, expr := range rowExprs {
			d, err := mapLiteral(expr, engSchema.Columns[i])
			if err != nil {
				return nil, err
			}
			mappedRow[i] = d
		}
	}

	// --- 4. Append row ---
	if err := insertHeap.Insert(ctx, req.TxContext, mappedRow); err != nil {
		return nil, fmt.Errorf("sql engine: insert: %w", err)
	}

	return &engine.Result{
		Stream:       nil,
		RowsAffected: 1,
		Message:      "INSERT 0 1",
	}, nil
}

// mapLiteral converts a Vitess expression (literal or NULL) to an engine.Datum
// based on the target column type. Expressions and functions are rejected.
func mapLiteral(expr vitess.Expr, col engine.Column) (engine.Datum, error) {
	switch v := expr.(type) {
	case *vitess.NullVal:
		// Typed NULL using the target column type.
		return engine.Datum{Type: col.Type, Value: nil}, nil
	case *vitess.Literal:
		return mapLiteralByType(v, col)
	default:
		return engine.Datum{}, ErrUnsupportedInsertValue
	}
}

// mapLiteralByType converts a Vitess Literal to engine.Datum using ValType.
func mapLiteralByType(lit *vitess.Literal, col engine.Column) (engine.Datum, error) {
	switch lit.Type {
	case vitess.IntVal:
		val, err := strconv.ParseInt(lit.Val, 10, 64)
		if err != nil {
			return engine.Datum{}, fmt.Errorf("%w: cannot parse int %q", ErrTypeMismatch, lit.Val)
		}
		switch col.Type {
		case engine.TypeInt64:
			return engine.Datum{Type: engine.TypeInt64, Value: val}, nil
		case engine.TypeUint64:
			if val < 0 {
				return engine.Datum{}, fmt.Errorf("%w: negative value for uint64 column", ErrTypeMismatch)
			}
			return engine.Datum{Type: engine.TypeUint64, Value: uint64(val)}, nil
		case engine.TypeFloat64:
			return engine.Datum{Type: engine.TypeFloat64, Value: float64(val)}, nil
		case engine.TypeString:
			return engine.Datum{Type: engine.TypeString, Value: lit.Val}, nil
		case engine.TypeBytes:
			return engine.Datum{Type: engine.TypeBytes, Value: []byte(lit.Val)}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot map integer to column type %d", ErrTypeMismatch, col.Type)
		}
	case vitess.FloatVal:
		val, err := strconv.ParseFloat(lit.Val, 64)
		if err != nil {
			return engine.Datum{}, fmt.Errorf("%w: cannot parse float %q", ErrTypeMismatch, lit.Val)
		}
		switch col.Type {
		case engine.TypeFloat64:
			return engine.Datum{Type: engine.TypeFloat64, Value: val}, nil
		case engine.TypeInt64:
			return engine.Datum{Type: engine.TypeInt64, Value: int64(val)}, nil
		case engine.TypeUint64:
			if val < 0 {
				return engine.Datum{}, fmt.Errorf("%w: negative value for uint64 column", ErrTypeMismatch)
			}
			return engine.Datum{Type: engine.TypeUint64, Value: uint64(val)}, nil
		case engine.TypeString:
			return engine.Datum{Type: engine.TypeString, Value: lit.Val}, nil
		case engine.TypeBytes:
			return engine.Datum{Type: engine.TypeBytes, Value: []byte(lit.Val)}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot map float to column type %d", ErrTypeMismatch, col.Type)
		}
	case vitess.StrVal:
		switch col.Type {
		case engine.TypeString:
			return engine.Datum{Type: engine.TypeString, Value: lit.Val}, nil
		case engine.TypeBytes:
			return engine.Datum{Type: engine.TypeBytes, Value: []byte(lit.Val)}, nil
		case engine.TypeFloat64:
			val, err := strconv.ParseFloat(lit.Val, 64)
			if err != nil {
				return engine.Datum{}, fmt.Errorf("%w: cannot parse string %q as float", ErrTypeMismatch, lit.Val)
			}
			return engine.Datum{Type: engine.TypeFloat64, Value: val}, nil
		case engine.TypeInt64:
			val, err := strconv.ParseInt(lit.Val, 10, 64)
			if err != nil {
				return engine.Datum{}, fmt.Errorf("%w: cannot parse string %q as int", ErrTypeMismatch, lit.Val)
			}
			return engine.Datum{Type: engine.TypeInt64, Value: val}, nil
		case engine.TypeUint64:
			val, err := strconv.ParseInt(lit.Val, 10, 64)
			if err != nil || val < 0 {
				return engine.Datum{}, fmt.Errorf("%w: cannot parse string %q as uint", ErrTypeMismatch, lit.Val)
			}
			return engine.Datum{Type: engine.TypeUint64, Value: uint64(val)}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot map string to column type %d", ErrTypeMismatch, col.Type)
		}
	case vitess.HexNum, vitess.HexVal:
		return engine.Datum{}, ErrUnsupportedInsertValue
	default:
		return engine.Datum{}, ErrUnsupportedInsertValue
	}
}
