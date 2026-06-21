// Package metrics provides a time-series metrics engine for Plomvix.
// engine.go implements the MetricsEngine, a pluggable backend for
// append-only time-series ingestion and range-scan queries.
package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// Sentinel errors.
var (
	ErrMissingTimeColumn   = errors.New("metrics engine: schema must contain a 'time' column")
	ErrNoMetricValueColumn = errors.New("metrics engine: schema must contain at least one numeric value column")
	ErrUnsupportedQuery    = errors.New("metrics engine: query statement not supported in basic tier")
)

// MetricsEngine is a pluggable time-series query execution engine.
type MetricsEngine struct {
	catalog catalog.Catalog
	store   *MetricsStore
}

// NewMetricsEngine creates a new MetricsEngine.
func NewMetricsEngine(cat catalog.Catalog, store *MetricsStore) *MetricsEngine {
	return &MetricsEngine{
		catalog: cat,
		store:   store,
	}
}

// Name returns the engine identifier.
func (e *MetricsEngine) Name() string { return "metrics" }

// ValidateSchema enforces that the schema has a 'time' column and at least
// one numeric metric value column.
func (e *MetricsEngine) ValidateSchema(schemaJSON []byte) error {
	var cols []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(schemaJSON, &cols); err != nil {
		return fmt.Errorf("metrics engine: invalid schema JSON: %w", err)
	}

	var hasTime bool
	var hasNumeric bool
	for _, col := range cols {
		if strings.ToLower(col.Name) == "time" {
			if col.Type == "int64" || col.Type == "uint64" {
				hasTime = true
			}
			continue
		}
		switch col.Type {
		case "int64", "uint64", "float64":
			hasNumeric = true
		}
	}
	if !hasTime {
		return ErrMissingTimeColumn
	}
	if !hasNumeric {
		return ErrNoMetricValueColumn
	}
	return nil
}

// Execute dispatches parsed statements to the metrics store.
func (e *MetricsEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	switch req.Stmt.Type() {
	case sqlparser.StmtInsert:
		return e.executeInsert(ctx, req)
	case sqlparser.StmtSelect:
		return e.executeSelect(ctx, req)
	default:
		return nil, ErrUnsupportedQuery
	}
}

// executeInsert processes INSERT INTO ... VALUES (...) for metrics.
func (e *MetricsEngine) executeInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	insert := req.Stmt.RawInsert()
	if insert == nil {
		return nil, fmt.Errorf("metrics engine: not an INSERT statement")
	}

	values, ok := insert.Rows.(vitess.Values)
	if !ok {
		return nil, ErrUnsupportedQuery
	}

	tableName := strings.ToLower(insert.Table.TableNameString())

	// Map column names to their positions.
	colMap := make(map[string]int)
	for i, col := range insert.Columns {
		colMap[strings.ToLower(col.String())] = i
	}

	// Process each row of VALUES.
	var rowsAffected int64
	for _, rowExprs := range values {
		pt, err := e.rowToPoint(tableName, colMap, rowExprs)
		if err != nil {
			return nil, err
		}
		if err := e.store.AppendPoint(ctx, pt); err != nil {
			return nil, fmt.Errorf("metrics engine: append point: %w", err)
		}
		rowsAffected++
	}

	return &engine.Result{RowsAffected: rowsAffected}, nil
}

// rowToPoint converts a Vitess INSERT row (ValTuple) to a metric Point.
func (e *MetricsEngine) rowToPoint(tableName string, colMap map[string]int, rowExprs vitess.ValTuple) (Point, error) {
	pt := Point{MetricName: tableName}

	// Extract time column.
	if idx, ok := colMap["time"]; ok && idx < len(rowExprs) {
		ts, err := exprToInt64(rowExprs[idx])
		if err != nil {
			return Point{}, fmt.Errorf("metrics engine: invalid time value: %w", err)
		}
		pt.Timestamp = ts
	}

	// Extract tags column (if present).
	if idx, ok := colMap["tags"]; ok && idx < len(rowExprs) {
		pt.Tags = exprToString(rowExprs[idx])
	}

	// Extract metric_name column (if present).
	if idx, ok := colMap["metric_name"]; ok && idx < len(rowExprs) {
		pt.MetricName = exprToString(rowExprs[idx])
	}

	// Extract value column (first non-metadata numeric column).
	for colName, idx := range colMap {
		if colName == "time" || colName == "tags" || colName == "metric_name" {
			continue
		}
		if idx < len(rowExprs) {
			v, err := exprToFloat64(rowExprs[idx])
			if err != nil {
				return Point{}, fmt.Errorf("metrics engine: invalid value for column %s: %w", colName, err)
			}
			pt.Value = v
			break
		}
	}

	return pt, nil
}

// executeSelect handles SELECT ... FROM ... WHERE ... for metrics.
func (e *MetricsEngine) executeSelect(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	sel, ok := req.Stmt.RawAST().(*vitess.Select)
	if !ok {
		return nil, fmt.Errorf("metrics engine: not a SELECT statement")
	}

	// Parse WHERE clause for time range and tag filters.
	start, end, tags, unsupported := parseWhere(sel.Where)
	if unsupported {
		return nil, ErrUnsupportedQuery
	}

	points, err := e.store.ScanRange(ctx, start, end, tags)
	if err != nil {
		return nil, fmt.Errorf("metrics engine: scan range: %w", err)
	}

	schema := engine.Schema{
		Columns: []engine.Column{
			{Name: "time", Type: engine.TypeInt64},
			{Name: "tags", Type: engine.TypeString},
			{Name: "metric_name", Type: engine.TypeString},
			{Name: "value", Type: engine.TypeFloat64},
		},
	}

	return &engine.Result{
		Stream: &metricsRowStream{points: points, schema: schema},
	}, nil
}

// parseWhere extracts time range and tag filters from a Vitess WHERE clause.
// start=0, end=0 means no range filter applied.
func parseWhere(where *vitess.Where) (int64, int64, map[string]string, bool) {
	tags := make(map[string]string)
	var start, end int64

	if where == nil || where.Expr == nil {
		return start, end, tags, false
	}

	walkWhere(where.Expr, &start, &end, tags)
	return start, end, tags, false
}

func walkWhere(expr vitess.Expr, start, end *int64, tags map[string]string) {
	switch e := expr.(type) {
	case *vitess.AndExpr:
		walkWhere(e.Left, start, end, tags)
		walkWhere(e.Right, start, end, tags)

	case *vitess.ComparisonExpr:
		colName := strings.ToLower(vitess.String(e.Left))
		switch e.Operator {
		case vitess.GreaterEqualOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err == nil {
					*start = v
				}
			}
		case vitess.LessEqualOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err == nil {
					*end = v
				}
			}
		case vitess.GreaterThanOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err == nil {
					*start = v + 1
				}
			}
		case vitess.LessThanOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err == nil {
					*end = v - 1
				}
			}
		case vitess.EqualOp:
			val := stripQuotes(vitess.String(e.Right))
			tags[colName] = val
		}

	case *vitess.BetweenExpr:
		if !e.IsBetween {
			break
		}
		colName := strings.ToLower(vitess.String(e.Left))
		if colName != "time" {
			break
		}
		v1, err1 := strconv.ParseInt(vitess.String(e.From), 10, 64)
		v2, err2 := strconv.ParseInt(vitess.String(e.To), 10, 64)
		if err1 == nil && err2 == nil {
			*start = v1
			*end = v2
		}
	}
}

// metricsRowStream implements engine.RowStream for scan results.
type metricsRowStream struct {
	points []Point
	schema engine.Schema
	pos    int
}

func (s *metricsRowStream) Next(ctx context.Context) (engine.Row, error) {
	if s.pos >= len(s.points) {
		return engine.Row{}, io.EOF
	}
	pt := s.points[s.pos]
	s.pos++
	return engine.Row{
		Datums: []engine.Datum{
			{Type: engine.TypeInt64, Value: pt.Timestamp},
			{Type: engine.TypeString, Value: pt.Tags},
			{Type: engine.TypeString, Value: pt.MetricName},
			{Type: engine.TypeFloat64, Value: pt.Value},
		},
	}, nil
}

func (s *metricsRowStream) Schema() engine.Schema {
	return s.schema.DeepCopy()
}

func (s *metricsRowStream) Close() error {
	return nil
}

// --- AST helper functions ---

func exprToInt64(expr vitess.Expr) (int64, error) {
	switch v := expr.(type) {
	case *vitess.Literal:
		return strconv.ParseInt(v.Val, 10, 64)
	default:
		return strconv.ParseInt(vitess.String(expr), 10, 64)
	}
}

func exprToFloat64(expr vitess.Expr) (float64, error) {
	switch v := expr.(type) {
	case *vitess.Literal:
		return strconv.ParseFloat(v.Val, 64)
	default:
		return strconv.ParseFloat(vitess.String(expr), 64)
	}
}

func exprToString(expr vitess.Expr) string {
	return stripQuotes(vitess.String(expr))
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
