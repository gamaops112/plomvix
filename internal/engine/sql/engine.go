// Package sql implements the SQL query execution engine for Plomvix.
// It translates parsed ASTs into Volcano-model physical plans, caches plan
// templates keyed by fingerprint + schema version, and executes them against
// the on-disk Table Heap. It also handles DDL (CREATE TABLE, DROP TABLE).
package sql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	"github.com/plomvix/plomvix/internal/engine/sql/tx"
	"github.com/plomvix/plomvix/internal/engine/sql/vacuum"
	"github.com/plomvix/plomvix/internal/sqlparser"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// Sentinel errors.
var (
	ErrNilSchemaVersionProvider = errors.New("sql engine: nil schema version provider")
	ErrNilPlanCache             = errors.New("sql engine: nil plan cache")
	ErrNilVacuumManager         = errors.New("sql engine: nil vacuum manager")
	ErrTableExists              = errors.New("sql engine: table already exists")
	ErrEmptySchema              = errors.New("sql engine: empty schema (zero columns)")
	ErrUnsupportedDDL           = errors.New("sql engine: unsupported DDL operation")
	ErrUnsupportedFeature       = errors.New("sql engine: unsupported SQL feature in basic tier")
)

// SQLEngine implements engine.Engine for SQL queries and DDL.
type SQLEngine struct {
	catalog         catalog.Catalog
	versions        planner.SchemaVersionProvider
	tables          TableManager
	decoder         planner.RowDecoder
	cache           *planner.PlanCache
	txm             *tx.Manager
	vacuum          *vacuum.Manager
	log             *slog.Logger
	maxBatchSize    int
	maxMutationRows int
}

// SQLEngineConfig holds all injectable dependencies for the SQL engine.
type SQLEngineConfig struct {
	Catalog         catalog.Catalog
	Versions        planner.SchemaVersionProvider
	TableManager    TableManager
	Decoder         planner.RowDecoder
	PlanCache       *planner.PlanCache
	TxManager       *tx.Manager
	VacuumManager   *vacuum.Manager
	Logger          *slog.Logger
	MaxBatchSize    int // Must be >= 1. 0 defaults to 1000.
	MaxMutationRows int // 0 defaults to 1000. -1 disables the guard.
}

// NewSQLEngine creates a new SQL engine. Returns error if critical deps are nil.
func NewSQLEngine(cfg SQLEngineConfig) (*SQLEngine, error) {
	if cfg.Catalog == nil {
		return nil, ErrNilCatalog
	}
	if cfg.TableManager == nil {
		return nil, ErrNilTableRegistry
	}
	if cfg.PlanCache == nil {
		return nil, ErrNilPlanCache
	}
	if cfg.Logger == nil {
		return nil, ErrNilLogger
	}
	if cfg.TxManager == nil {
		return nil, ErrNilTxManager
	}
	if cfg.VacuumManager == nil {
		return nil, ErrNilVacuumManager
	}
	if cfg.Versions == nil {
		return nil, ErrNilSchemaVersionProvider
	}
	mb := cfg.MaxBatchSize
	if mb <= 0 {
		mb = 1000
	}
	mmr := cfg.MaxMutationRows
	if mmr == 0 {
		mmr = 1000
	}
	return &SQLEngine{
		catalog:         cfg.Catalog,
		versions:        cfg.Versions,
		tables:          cfg.TableManager,
		decoder:         cfg.Decoder,
		cache:           cfg.PlanCache,
		txm:             cfg.TxManager,
		vacuum:          cfg.VacuumManager,
		log:             cfg.Logger,
		maxBatchSize:    mb,
		maxMutationRows: mmr,
	}, nil
}

// Name returns the engine identifier.
func (e *SQLEngine) Name() string { return "sql" }

// ValidateSchema implements catalog.Engine. It validates a JSON-encoded
// table schema against the engine's column type system.
func (e *SQLEngine) ValidateSchema(schemaJSON []byte) error {
	_, err := schema.Decode(schemaJSON)
	return err
}

// Execute dispatches based on statement type.
// DML statements (INSERT, UPDATE, DELETE) are handled BEFORE the plan cache
// lookup. DML never interacts with the plan cache — no Lookup(), no Store().
// This is enforced by code structure: the DML arm is above the SELECT path.
func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	// Allocate WriteTxID exactly once for DML and DDL.
	if req.Stmt.Type() == sqlparser.StmtInsert || req.Stmt.Type() == sqlparser.StmtUpdate || req.Stmt.Type() == sqlparser.StmtDelete || req.Stmt.Type() == sqlparser.StmtDDL {
		req.TxContext.WriteTxID = e.txm.NextWriteTx()
	}

	// DML: handled above the plan cache (never cached).
	switch req.Stmt.Type() {
	case sqlparser.StmtSelect:
		return e.executeSelect(ctx, req)
	case sqlparser.StmtDDL:
		return e.executeDDL(ctx, req)
	case sqlparser.StmtInsert:
		return e.execInsert(ctx, req)
	case sqlparser.StmtUpdate:
		return e.execUpdate(ctx, req)
	case sqlparser.StmtDelete:
		return e.execDelete(ctx, req)
	default:
		return nil, ErrUnsupportedFeature
	}
}

// executeSelect is the cache-first SELECT path.
func (e *SQLEngine) executeSelect(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	start := time.Now()
	req.TxContext.ReadTxID = e.txm.NextReadTx()

	fingerprint := req.Stmt.Fingerprint()
	schemaVersion := e.versions.SchemaVersion()
	cacheKey := planner.CacheKey{Fingerprint: fingerprint, SchemaVersion: schemaVersion}

	tmpl := e.cache.Lookup(cacheKey)
	if tmpl == nil {
		e.log.Debug("planner", "event", "cache_miss", "fingerprint", fingerprint)
		planStart := time.Now()
		var err error
		tmpl, err = planner.Plan(ctx, e.catalog, req)
		if err != nil {
			return nil, err
		}
		e.log.Debug("planner",
			"event", "plan_generated",
			"fingerprint", fingerprint,
			"latency_ns", time.Since(planStart).Nanoseconds(),
		)
		e.cache.Store(cacheKey, tmpl)
	} else {
		e.log.Debug("planner", "event", "cache_hit", "fingerprint", fingerprint)
	}

	heap, err := e.tables.GetTableHeap(tmpl.TableID)
	if err != nil {
		return nil, fmt.Errorf("sql engine: table heap %d: %w", tmpl.TableID, planner.ErrTableHeapNotFound)
	}

	op := tmpl.Build(heap, e.decoder, req.TxContext)
	if err := op.Open(ctx); err != nil {
		_ = op.Close()
		return nil, err
	}

	e.log.Debug("planner",
		"event", "plan_opened",
		"fingerprint", fingerprint,
		"total_latency_ns", time.Since(start).Nanoseconds(),
	)

	return &engine.Result{Stream: &operatorStream{op: op}}, nil
}

// executeDDL handles CREATE TABLE and DROP TABLE.
func (e *SQLEngine) executeDDL(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	ddlStmt := req.Stmt.RawDDL()
	if ddlStmt == nil {
		return nil, ErrUnsupportedDDL
	}

	action := ddlStmt.GetAction()
	switch action {
	case vitess.CreateDDLAction:
		return e.executeCreateTable(ctx, req, ddlStmt)
	case vitess.DropDDLAction:
		return e.executeDropTable(ctx, req, ddlStmt)
	default:
		return nil, ErrUnsupportedDDL
	}
}

// executeCreateTable handles CREATE TABLE.
func (e *SQLEngine) executeCreateTable(ctx context.Context, req *engine.Request, ddlStmt vitess.DDLStatement) (*engine.Result, error) {
	tableName := ddlStmt.GetTable().Name.String()
	if tableName == "" {
		return nil, fmt.Errorf("sql engine: empty table name")
	}

	// Check global DDL permission.
	ok, err := e.catalog.CheckGlobalPermission(ctx, req.UserID, catalog.ActionDDL)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnsupportedFeature
	}

	// Convert AST columns to heap.Schema.
	schema, err := ddlColumnsToHeapSchema(ddlStmt)
	if err != nil {
		return nil, err
	}
	if len(schema.Columns) == 0 {
		return nil, ErrEmptySchema
	}

	// Encode schema payload for catalog storage.
	schemaPayload, err := EncodeSchemaPayload(schema)
	if err != nil {
		return nil, fmt.Errorf("sql engine: encode schema: %w", err)
	}

	// Allocate table ID.
	tableID, err := e.catalog.AllocateTableID(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize physical heap; get the file path for potential cleanup.
	_, heapPath, err := e.tables.CreateTableHeap(ctx, tableID, schema)
	if err != nil {
		return nil, fmt.Errorf("sql engine: create heap: %w", err)
	}

	// Register in catalog (only after heap succeeds).
	if err := e.catalog.RegisterTable(ctx, tableID, e.Name(), tableName, schemaPayload); err != nil {
		// Transactional cleanup: remove orphaned heap file.
		_ = os.Remove(heapPath)
		return nil, fmt.Errorf("sql engine: register table: %w", err)
	}

	return &engine.Result{
		Message: fmt.Sprintf("CREATE TABLE %s (table_id=%d)", tableName, tableID),
	}, nil
}

// executeDropTable handles DROP TABLE.
func (e *SQLEngine) executeDropTable(ctx context.Context, req *engine.Request, ddlStmt vitess.DDLStatement) (*engine.Result, error) {
	// For DROP TABLE, the table name is in GetFromTables(), not GetTable().
	fromTables := ddlStmt.GetFromTables()
	if len(fromTables) == 0 {
		return nil, fmt.Errorf("sql engine: empty table name")
	}
	tableName := fromTables[0].Name.String()
	if tableName == "" {
		return nil, fmt.Errorf("sql engine: empty table name")
	}

	// Check global DDL permission.
	ok, err := e.catalog.CheckGlobalPermission(ctx, req.UserID, catalog.ActionDDL)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnsupportedFeature
	}

	// Look up the table info to get the TableID before dropping.
	info, err := e.catalog.GetTable(ctx, tableName)
	if err != nil {
		return nil, err
	}

	if err := e.catalog.DropTable(ctx, tableName); err != nil {
		return nil, err
	}

	// Schedule physical file deletion via vacuum manager (non-blocking).
	heapPath := e.tables.HeapPath(info.TableID)
	_ = e.vacuum.ScheduleDeletion(info.TableID, heapPath)

	return &engine.Result{
		Message: fmt.Sprintf("DROP TABLE %s", tableName),
	}, nil
}

// ddlColumnsToHeapSchema converts Vitess DDL column definitions to a heap.Schema.
// Constraints (NOT NULL, DEFAULT, PRIMARY KEY) are explicitly ignored in Basic.
func ddlColumnsToHeapSchema(ddlStmt vitess.DDLStatement) (heap.Schema, error) {
	spec := ddlStmt.GetTableSpec()
	if spec == nil || len(spec.Columns) == 0 {
		return heap.Schema{}, ErrEmptySchema
	}

	columns := make([]heap.Column, 0, len(spec.Columns))
	colNames := make(map[string]bool)

	for _, col := range spec.Columns {
		name := col.Name.String()
		if name == "" {
			continue
		}
		if colNames[name] {
			return heap.Schema{}, fmt.Errorf("sql engine: duplicate column %q", name)
		}
		colNames[name] = true

		kind, err := vitessTypeToKeyKind(col.Type.Type)
		if err != nil {
			return heap.Schema{}, fmt.Errorf("sql engine: column %q: %w", name, err)
		}

		columns = append(columns, heap.Column{Name: name, Kind: kind})
	}

	if len(columns) == 0 {
		return heap.Schema{}, ErrEmptySchema
	}

	// Use the first column as PK in Basic tier.
	return heap.Schema{
		Columns:   columns,
		PKIndices: []int{0},
	}, nil
}

// vitessTypeToKeyKind maps a Vitess type string to a key.Kind.
func vitessTypeToKeyKind(vt string) (key.Kind, error) {
	switch vt {
	case "int", "integer", "bigint", "smallint", "tinyint", "mediumint":
		return key.KindInt64, nil
	case "varchar", "char", "text", "tinytext", "mediumtext", "longtext":
		return key.KindString, nil
	case "blob", "varbinary", "binary", "tinyblob", "mediumblob", "longblob":
		return key.KindBytes, nil
	case "boolean", "bool":
		return key.KindInt64, nil
	default:
		return 0, fmt.Errorf("sql engine: unsupported column type %q in basic tier", vt)
	}
}

// EncodeSchemaPayload encodes a heap.Schema to binary bytes for catalog storage.
func EncodeSchemaPayload(s heap.Schema) ([]byte, error) {
	engCols := make([]engine.Column, len(s.Columns))
	for i, col := range s.Columns {
		engCols[i] = engine.Column{Name: col.Name, Type: keyKindToEngineType(col.Kind)}
	}
	engSchema := engine.Schema{Columns: engCols}
	return schema.Encode(engSchema)
}

func keyKindToEngineType(k key.Kind) engine.Type {
	switch k {
	case key.KindUint64:
		return engine.TypeUint64
	case key.KindInt64:
		return engine.TypeInt64
	case key.KindString:
		return engine.TypeString
	case key.KindBytes:
		return engine.TypeBytes
	default:
		return engine.TypeNull
	}
}

// operatorStream adapts a planner.Operator into an engine.RowStream.
type operatorStream struct {
	op planner.Operator
}

func (s *operatorStream) Next(ctx context.Context) (engine.Row, error) { return s.op.Next(ctx) }
func (s *operatorStream) Schema() engine.Schema                        { return s.op.Schema().DeepCopy() }
func (s *operatorStream) Close() error                                 { return s.op.Close() }

var _ engine.Engine = (*SQLEngine)(nil)
var _ engine.RowStream = (*operatorStream)(nil)
