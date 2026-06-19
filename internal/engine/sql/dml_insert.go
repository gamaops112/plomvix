package sql

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	"github.com/plomvix/plomvix/internal/sqlparser"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// InsertableTableHeap is the local contract for appending rows.
// Enterprise extensions: InsertBatch, BeginInsertStream.
type InsertableTableHeap interface {
	Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error
	// InsertBatch atomically appends multiple rows under one write lock.
	InsertBatch(ctx context.Context, tx engine.TxContext, rows []engine.Row) (int, error)
	// BeginInsertStream starts a streaming insert session.
	BeginInsertStream(ctx context.Context, tx engine.TxContext) (InsertStream, error)
}

// InsertStream is a session for row-by-row appending.
type InsertStream interface {
	Append(ctx context.Context, row engine.Row) error
	Commit() error
	Abort() error
}

// execInsert handles batch INSERT VALUES and dispatches INSERT SELECT.
func (e *SQLEngine) execInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	insert := req.Stmt.RawInsert()
	if insert == nil {
		return nil, ErrUnsupportedDML
	}

	values, ok := insert.Rows.(vitess.Values)
	if !ok {
		return e.execInsertSelect(ctx, req, insert)
	}

	// --- Batch INSERT VALUES ---
	if len(values) > e.maxBatchSize {
		return nil, ErrBatchTooLarge
	}

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

	// Validate all rows first (zero heap writes on any error).
	mappedRows := make([]engine.Row, 0, len(values))
	for _, rowExprs := range values {
		row, err := buildRow(&engSchema, insert.Columns, rowExprs)
		if err != nil {
			return nil, err
		}
		mappedRows = append(mappedRows, row)
	}

	rowsAffected, err := insertHeap.InsertBatch(ctx, req.TxContext, mappedRows)
	if err != nil {
		return nil, err
	}

	e.log.Info("insert",
		slog.String("table", tableName),
		slog.Int("rows_affected", rowsAffected),
		slog.Uint64("write_tx_id", req.TxContext.WriteTxID),
	)

	return &engine.Result{
		Stream:       nil,
		RowsAffected: int64(rowsAffected),
		Message:      fmt.Sprintf("INSERT 0 %d", rowsAffected),
	}, nil
}

// execInsertSelect handles INSERT INTO t SELECT ...
func (e *SQLEngine) execInsertSelect(ctx context.Context, req *engine.Request, stmt *vitess.Insert) (*engine.Result, error) {
	tableName := stmt.Table.TableNameString()
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

	// Build a SELECT statement string from the AST and re-parse it.
	selectSQL := vitess.String(stmt.Rows)
	p, err := sqlparser.New()
	if err != nil {
		return nil, fmt.Errorf("sql engine: create parser: %w", err)
	}
	selectStmt, err := p.Parse(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("sql engine: parse SELECT: %w", err)
	}
	selectReq := &engine.Request{
		Stmt:      selectStmt,
		UserID:    req.UserID,
		TxContext: req.TxContext,
	}

	op, err := planner.Plan(ctx, e.catalog, selectReq)
	if err != nil {
		return nil, fmt.Errorf("sql engine: plan SELECT: %w", err)
	}
	srcHeap, err := e.tables.GetTableHeap(op.TableID)
	if err != nil {
		return nil, fmt.Errorf("sql engine: source heap: %w", err)
	}
	physOp := op.Build(srcHeap, e.decoder, req.TxContext)
	if err := physOp.Open(ctx); err != nil {
		return nil, err
	}
	defer physOp.Close()

	stream, err := insertHeap.BeginInsertStream(ctx, req.TxContext)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = stream.Abort()
		}
	}()

	var rowsAffected int64
	for {
		srcRow, err := physOp.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		targetRow, err := coerceRow(&engSchema, srcRow)
		if err != nil {
			return nil, err
		}
		if err := stream.Append(ctx, targetRow); err != nil {
			return nil, err
		}
		rowsAffected++
	}

	if err := stream.Commit(); err != nil {
		return nil, err
	}
	committed = true

	e.log.Info("insert",
		slog.String("table", tableName),
		slog.Int64("rows_affected", rowsAffected),
		slog.Uint64("write_tx_id", req.TxContext.WriteTxID),
	)

	return &engine.Result{
		Stream:       nil,
		RowsAffected: rowsAffected,
		Message:      fmt.Sprintf("INSERT 0 %d", rowsAffected),
	}, nil
}

// buildRow maps a single Vitess Values row to an engine.Row, applying DEFAULTs
// and enforcing NOT NULL. Called for every row before any heap write.
func buildRow(engSchema *engine.Schema, insertCols vitess.Columns, rowExprs vitess.ValTuple) (engine.Row, error) {
	// Build name → index map.
	schemaIndexByName := make(map[string]int, len(engSchema.Columns))
	for i, col := range engSchema.Columns {
		schemaIndexByName[strings.ToLower(col.Name)] = i
	}

	// Initialize with DEFAULT values or typed NULL.
	mappedRow := make(engine.Row, len(engSchema.Columns))
	for i, col := range engSchema.Columns {
		if col.DefaultValue != nil {
			mappedRow[i] = col.DefaultValue.DeepCopy()
		} else {
			mappedRow[i] = engine.Datum{Type: col.Type, Value: nil}
		}
	}

	if len(insertCols) > 0 {
		if len(insertCols) != len(rowExprs) {
			return nil, ErrColumnCountMismatch
		}
		seen := make(map[string]bool)
		for i, colIdent := range insertCols {
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

	// NOT NULL enforcement.
	for i, col := range engSchema.Columns {
		if col.NotNull && mappedRow[i].Value == nil {
			return nil, fmt.Errorf("%w: column %q", ErrNotNullViolation, col.Name)
		}
	}

	return mappedRow, nil
}

// coerceRow converts a source row to the target schema, applying type coercion
// and NOT NULL checks. Used for INSERT SELECT.
func coerceRow(targetSchema *engine.Schema, srcRow engine.Row) (engine.Row, error) {
	if len(srcRow) != len(targetSchema.Columns) {
		// Pad or truncate? For simplicity, require exact match.
		return nil, ErrColumnCountMismatch
	}
	out := make(engine.Row, len(targetSchema.Columns))
	for i, col := range targetSchema.Columns {
		d, err := coerceDatum(srcRow[i], col)
		if err != nil {
			return nil, err
		}
		if col.NotNull && d.Value == nil {
			return nil, fmt.Errorf("%w: column %q", ErrNotNullViolation, col.Name)
		}
		out[i] = d
	}
	return out, nil
}

// coerceDatum converts a datum to the target column type if possible.
func coerceDatum(src engine.Datum, col engine.Column) (engine.Datum, error) {
	if src.Value == nil {
		return engine.Datum{Type: col.Type, Value: nil}, nil
	}
	if src.Type == col.Type {
		return src.DeepCopy(), nil
	}
	// Simple coercion: numeric ↔ numeric, string ↔ string.
	switch col.Type {
	case engine.TypeInt64:
		switch v := src.Value.(type) {
		case int64:
			return engine.Datum{Type: col.Type, Value: v}, nil
		case uint64:
			return engine.Datum{Type: col.Type, Value: int64(v)}, nil
		case float64:
			return engine.Datum{Type: col.Type, Value: int64(v)}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot coerce to int64", ErrTypeMismatch)
		}
	case engine.TypeUint64:
		switch v := src.Value.(type) {
		case int64:
			if v < 0 {
				return engine.Datum{}, fmt.Errorf("%w: negative to uint64", ErrTypeMismatch)
			}
			return engine.Datum{Type: col.Type, Value: uint64(v)}, nil
		case uint64:
			return engine.Datum{Type: col.Type, Value: v}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot coerce to uint64", ErrTypeMismatch)
		}
	case engine.TypeFloat64:
		switch v := src.Value.(type) {
		case int64:
			return engine.Datum{Type: col.Type, Value: float64(v)}, nil
		case uint64:
			return engine.Datum{Type: col.Type, Value: float64(v)}, nil
		case float64:
			return engine.Datum{Type: col.Type, Value: v}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot coerce to float64", ErrTypeMismatch)
		}
	case engine.TypeString:
		switch v := src.Value.(type) {
		case string:
			return engine.Datum{Type: col.Type, Value: v}, nil
		case []byte:
			return engine.Datum{Type: col.Type, Value: string(v)}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot coerce to string", ErrTypeMismatch)
		}
	case engine.TypeBytes:
		switch v := src.Value.(type) {
		case string:
			return engine.Datum{Type: col.Type, Value: []byte(v)}, nil
		case []byte:
			return engine.Datum{Type: col.Type, Value: v}, nil
		default:
			return engine.Datum{}, fmt.Errorf("%w: cannot coerce to bytes", ErrTypeMismatch)
		}
	default:
		return engine.Datum{}, ErrTypeMismatch
	}
}

// mapLiteral converts a Vitess expression (literal or NULL) to an engine.Datum
// based on the target column type.
func mapLiteral(expr vitess.Expr, col engine.Column) (engine.Datum, error) {
	switch v := expr.(type) {
	case *vitess.NullVal:
		return engine.Datum{Type: col.Type, Value: nil}, nil
	case *vitess.Literal:
		return mapLiteralByType(v, col)
	default:
		return engine.Datum{}, ErrUnsupportedInsertValue
	}
}

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
