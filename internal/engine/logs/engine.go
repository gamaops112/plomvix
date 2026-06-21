// Package logs provides a pluggable logs engine for Plomvix.
// engine.go implements the LogsEngine, a pluggable backend for
// append-only log ingestion and text-search queries.
package logs

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
	ErrUnsupportedQuery = errors.New("logs engine: query statement not supported in basic tier")
	ErrComplexWhere     = errors.New("logs engine: complex WHERE expression not supported")
)

// LogsEngine is a pluggable log query execution engine.
type LogsEngine struct {
	catalog    catalog.Catalog
	store      *LogsStore
	tokenIndex *TokenIndex      // enterprise: inverted token index for full-text search
	retention  *RetentionWorker // enterprise: background log retention
}

// NewLogsEngine creates a new LogsEngine.
func NewLogsEngine(cat catalog.Catalog, store *LogsStore) *LogsEngine {
	return NewLogsEngineWithIndex(cat, store, nil, nil)
}

// NewLogsEngineWithIndex creates a new LogsEngine with enterprise
// token index and retention worker support.
func NewLogsEngineWithIndex(cat catalog.Catalog, store *LogsStore, idx *TokenIndex, retention *RetentionWorker) *LogsEngine {
	return &LogsEngine{
		catalog:    cat,
		store:      store,
		tokenIndex: idx,
		retention:  retention,
	}
}

// Name returns the engine identifier.
func (e *LogsEngine) Name() string { return "logs" }

// Store returns the underlying store (for testing).
func (e *LogsEngine) Store() *LogsStore { return e.store }

// TokenIndex returns the token index (for testing/wiring).
func (e *LogsEngine) TokenIndex() *TokenIndex { return e.tokenIndex }

// Retention returns the retention worker (for testing/wiring).
func (e *LogsEngine) Retention() *RetentionWorker { return e.retention }

// ValidateSchema enforces that the schema has time, severity, attributes, and body columns.
func (e *LogsEngine) ValidateSchema(schemaJSON []byte) error {
	var cols []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(schemaJSON, &cols); err != nil {
		return fmt.Errorf("logs engine: invalid schema JSON: %w", err)
	}

	var hasTime, hasSeverity, hasBody bool
	for _, col := range cols {
		switch strings.ToLower(col.Name) {
		case "time":
			if col.Type == "int64" || col.Type == "uint64" {
				hasTime = true
			}
		case "severity":
			if col.Type == "string" {
				hasSeverity = true
			}
		case "attributes":
			// optional, no flag required
		case "body":
			if col.Type == "string" {
				hasBody = true
			}
		}
	}
	if !hasTime {
		return fmt.Errorf("logs engine: schema must contain a 'time' column (int64)")
	}
	if !hasSeverity {
		return fmt.Errorf("logs engine: schema must contain a 'severity' column (string)")
	}
	if !hasBody {
		return fmt.Errorf("logs engine: schema must contain a 'body' column (string)")
	}
	return nil
}

// Execute dispatches parsed statements to the logs store.
func (e *LogsEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	switch req.Stmt.Type() {
	case sqlparser.StmtInsert:
		return e.executeInsert(ctx, req)
	case sqlparser.StmtSelect:
		return e.executeSelect(ctx, req)
	default:
		return nil, ErrUnsupportedQuery
	}
}

// executeInsert processes INSERT INTO ... VALUES (...) for logs.
// JSON parsing: extracts severity/level/status keys, moves other fields to
// attributes_payload, places core message in body_payload.
// Raw text: entire value placed in body_payload, severity defaults to INFO.
func (e *LogsEngine) executeInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	insert := req.Stmt.RawInsert()
	if insert == nil {
		return nil, fmt.Errorf("logs engine: not an INSERT statement")
	}

	values, ok := insert.Rows.(vitess.Values)
	if !ok {
		return nil, ErrUnsupportedQuery
	}

	// Map column names to their positions.
	colMap := make(map[string]int)
	for i, col := range insert.Columns {
		colMap[strings.ToLower(col.String())] = i
	}

	// Process each row of VALUES.
	var rowsAffected int64
	for _, rowExprs := range values {
		rec, err := e.rowToLogRecord(colMap, rowExprs)
		if err != nil {
			return nil, err
		}
		if err := e.store.AppendLog(ctx, rec); err != nil {
			return nil, fmt.Errorf("logs engine: append log: %w", err)
		}
		rowsAffected++
	}

	return &engine.Result{RowsAffected: rowsAffected}, nil
}

// rowToLogRecord converts a Vitess INSERT row to a LogRecord.
// Handles JSON parsing for severity extraction and attribute separation.
func (e *LogsEngine) rowToLogRecord(colMap map[string]int, rowExprs vitess.ValTuple) (LogRecord, error) {
	rec := LogRecord{Severity: SeverityInfo}

	// Extract time column.
	if idx, ok := colMap["time"]; ok && idx < len(rowExprs) {
		ts, err := exprToInt64(rowExprs[idx])
		if err != nil {
			return LogRecord{}, fmt.Errorf("logs engine: invalid time value: %w", err)
		}
		rec.Timestamp = ts
	}

	// Extract body column and parse as JSON if possible.
	if idx, ok := colMap["body"]; ok && idx < len(rowExprs) {
		bodyStr := stripQuotes(vitess.String(rowExprs[idx]))
		rec.Body, rec.Severity, rec.Attributes = parseLogPayload(bodyStr)
	}

	// Explicit severity column overrides JSON-parsed severity.
	if idx, ok := colMap["severity"]; ok && idx < len(rowExprs) {
		sevStr := stripQuotes(vitess.String(rowExprs[idx]))
		rec.Severity = parseSeverity(sevStr)
	}

	// Explicit attributes column overrides parsed attributes.
	if idx, ok := colMap["attributes"]; ok && idx < len(rowExprs) {
		rec.Attributes = stripQuotes(vitess.String(rowExprs[idx]))
	}

	return rec, nil
}

// parseLogPayload attempts to parse a log line as JSON.
// If valid JSON, extracts severity from 'level'/'severity'/'status' keys,
// moves remaining non-standard keys to attributes, and uses 'message'/'msg'/'body' as body.
// If not valid JSON, treats the entire string as body with severity INFO.
func parseLogPayload(raw string) (body string, severity uint8, attributes string) {
	// Try JSON parsing.
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		// Not valid JSON: treat entire payload as body.
		return raw, SeverityInfo, ""
	}

	// Extract severity from common keys.
	sev := SeverityInfo
	for _, key := range []string{"level", "severity", "status"} {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				sev = parseSeverity(s)
				delete(m, key)
				break
			}
		}
	}

	// Extract body from common message keys.
	bodyText := ""
	for _, key := range []string{"message", "msg", "body"} {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				bodyText = s
				delete(m, key)
				break
			}
		}
	}
	if bodyText == "" {
		// No message key found: serialize the whole object as body.
		bodyText = raw
		// Clear remaining map since everything is in body.
		m = nil
	}

	// Remaining keys become attributes.
	attrsStr := ""
	if len(m) > 0 {
		attrsBytes, err := json.Marshal(m)
		if err == nil {
			attrsStr = string(attrsBytes)
		}
	}

	return bodyText, sev, attrsStr
}

// executeSelect handles SELECT ... FROM ... WHERE ... for logs.
// Supports time range filtering and body LIKE substring matching.
// Enterprise: uses the token index for fast full-text lookup.
func (e *LogsEngine) executeSelect(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	sel, ok := req.Stmt.RawAST().(*vitess.Select)
	if !ok {
		return nil, fmt.Errorf("logs engine: not a SELECT statement")
	}

	// Parse WHERE clause for time range and body LIKE filter.
	start, end, bodyFilter, err := parseLogsWhere(sel.Where)
	if err != nil {
		return nil, err
	}

	var records []LogRecord
	if e.tokenIndex != nil && bodyFilter != "" {
		// Enterprise: tokenize and use indexed search.
		tokens := Tokenize(bodyFilter)
		records, err = e.store.ScanRangeWithIndex(ctx, start, end, tokens)
	} else {
		records, err = e.store.ScanRange(ctx, start, end, bodyFilter)
	}
	if err != nil {
		return nil, fmt.Errorf("logs engine: scan range: %w", err)
	}

	schema := engine.Schema{
		Columns: []engine.Column{
			{Name: "time", Type: engine.TypeInt64},
			{Name: "severity", Type: engine.TypeString},
			{Name: "attributes", Type: engine.TypeString},
			{Name: "body", Type: engine.TypeString},
		},
	}

	return &engine.Result{
		Stream: &logsRowStream{records: records, schema: schema},
	}, nil
}

// parseLogsWhere extracts time range and body LIKE filter from a WHERE clause.
// Returns ErrUnsupportedQuery for placeholders, column references, or complex expressions.
func parseLogsWhere(where *vitess.Where) (start, end int64, bodyFilter string, err error) {
	if where == nil || where.Expr == nil {
		return 0, 0, "", nil
	}

	var unsupported bool
	walkLogsWhere(where.Expr, &start, &end, &bodyFilter, &unsupported)
	if unsupported {
		return 0, 0, "", ErrUnsupportedQuery
	}
	return start, end, bodyFilter, nil
}

func walkLogsWhere(expr vitess.Expr, start, end *int64, bodyFilter *string, unsupported *bool) {
	switch e := expr.(type) {
	case *vitess.AndExpr:
		walkLogsWhere(e.Left, start, end, bodyFilter, unsupported)
		walkLogsWhere(e.Right, start, end, bodyFilter, unsupported)

	case *vitess.ComparisonExpr:
		colName := strings.ToLower(vitess.String(e.Left))
		switch e.Operator {
		case vitess.GreaterEqualOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err != nil {
					*unsupported = true
					return
				}
				*start = v
			}
		case vitess.LessEqualOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err != nil {
					*unsupported = true
					return
				}
				*end = v
			}
		case vitess.GreaterThanOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err != nil {
					*unsupported = true
					return
				}
				*start = v + 1
			}
		case vitess.LessThanOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err != nil {
					*unsupported = true
					return
				}
				*end = v - 1
			}
		case vitess.EqualOp:
			if colName == "time" {
				v, err := strconv.ParseInt(vitess.String(e.Right), 10, 64)
				if err != nil {
					*unsupported = true
					return
				}
				*start = v
				*end = v
			}
		case vitess.LikeOp:
			if colName == "body" {
				// Must be a literal string — reject column refs or placeholders.
				rightStr := vitess.String(e.Right)
				if _, isCol := e.Right.(*vitess.ColName); isCol {
					*unsupported = true
					return
				}
				if _, isArg := e.Right.(*vitess.Argument); isArg {
					*unsupported = true
					return
				}
				// Strip leading/trailing % for substring match.
				filter := stripQuotes(rightStr)
				filter = strings.TrimPrefix(filter, "%")
				filter = strings.TrimSuffix(filter, "%")
				*bodyFilter = filter
			}
		case vitess.NotLikeOp:
			// NotLike is not supported in basic tier.
			*unsupported = true
		default:
			// Other operators (e.g. !=, IS NULL) are unsupported.
			*unsupported = true
		}

	case *vitess.BetweenExpr:
		if !e.IsBetween {
			break
		}
		colName := strings.ToLower(vitess.String(e.Left))
		if colName != "time" {
			*unsupported = true
			return
		}
		v1, err1 := strconv.ParseInt(vitess.String(e.From), 10, 64)
		v2, err2 := strconv.ParseInt(vitess.String(e.To), 10, 64)
		if err1 != nil || err2 != nil {
			*unsupported = true
			return
		}
		*start = v1
		*end = v2

	case *vitess.OrExpr, *vitess.NotExpr:
		// Complex expressions not supported in basic tier.
		*unsupported = true

	default:
		// Unknown expression type: allow but don't filter.
	}
}

// logsRowStream implements engine.RowStream for log scan results.
type logsRowStream struct {
	records []LogRecord
	schema  engine.Schema
	pos     int
}

func (s *logsRowStream) Next(ctx context.Context) (engine.Row, error) {
	if s.pos >= len(s.records) {
		return engine.Row{}, io.EOF
	}
	rec := s.records[s.pos]
	s.pos++
	return engine.Row{
		Datums: []engine.Datum{
			{Type: engine.TypeInt64, Value: rec.Timestamp},
			{Type: engine.TypeString, Value: severityToString(rec.Severity)},
			{Type: engine.TypeString, Value: rec.Attributes},
			{Type: engine.TypeString, Value: rec.Body},
		},
	}, nil
}

func (s *logsRowStream) Schema() engine.Schema {
	return s.schema.DeepCopy()
}

func (s *logsRowStream) Close() error {
	return nil
}

// --- AST helper functions (shared with metrics engine pattern) ---

func exprToInt64(expr vitess.Expr) (int64, error) {
	switch v := expr.(type) {
	case *vitess.Literal:
		return strconv.ParseInt(v.Val, 10, 64)
	default:
		return strconv.ParseInt(vitess.String(expr), 10, 64)
	}
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
