// Package sql implements the SQL query execution engine for Plomvix.
// It translates parsed ASTs into Volcano-model physical plans, caches plan
// templates keyed by fingerprint + schema version, and executes them against
// the on-disk Table Heap.
package sql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
)

// Sentinel errors for constructor validation.
var (
	ErrNilSchemaVersionProvider = errors.New("sql engine: nil schema version provider")
	ErrNilPlanCache             = errors.New("sql engine: nil plan cache")
	ErrNilLogger                = errors.New("sql engine: nil logger")
)

// SQLEngine implements engine.Engine for SQL queries.
type SQLEngine struct {
	catalog  catalog.Catalog
	versions planner.SchemaVersionProvider
	tables   planner.TableRegistry
	decoder  planner.RowDecoder
	cache    *planner.PlanCache
	log      *slog.Logger
}

// NewSQLEngine creates a new SQL engine. Returns error if critical deps are nil.
func NewSQLEngine(
	cat catalog.Catalog,
	versions planner.SchemaVersionProvider,
	tables planner.TableRegistry,
	decoder planner.RowDecoder,
	cache *planner.PlanCache,
	log *slog.Logger,
) (*SQLEngine, error) {
	if versions == nil {
		return nil, ErrNilSchemaVersionProvider
	}
	if cache == nil {
		return nil, ErrNilPlanCache
	}
	if log == nil {
		return nil, ErrNilLogger
	}
	return &SQLEngine{
		catalog:  cat,
		versions: versions,
		tables:   tables,
		decoder:  decoder,
		cache:    cache,
		log:      log,
	}, nil
}

// Name returns the engine identifier.
func (e *SQLEngine) Name() string { return "sql" }

// Execute follows the cache-first flow: look up plan template by fingerprint
// + schema version, build a fresh Operator tree, open, and execute.
func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (engine.RowStream, error) {
	start := time.Now()

	fingerprint := req.Stmt.Fingerprint()
	schemaVersion := e.versions.SchemaVersion()
	key := planner.CacheKey{Fingerprint: fingerprint, SchemaVersion: schemaVersion}

	tmpl := e.cache.Lookup(key)
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
		e.cache.Store(key, tmpl)
	} else {
		e.log.Debug("planner", "event", "cache_hit", "fingerprint", fingerprint)
	}

	heap, err := e.tables.GetTableHeap(tmpl.TableID)
	if err != nil {
		return nil, fmt.Errorf("sql engine: table heap %d: %w", tmpl.TableID, planner.ErrTableHeapNotFound)
	}

	op := tmpl.Build(heap, e.decoder)

	if err := op.Open(ctx); err != nil {
		_ = op.Close()
		return nil, err
	}

	e.log.Debug("planner",
		"event", "plan_opened",
		"fingerprint", fingerprint,
		"total_latency_ns", time.Since(start).Nanoseconds(),
	)

	return &operatorStream{op: op}, nil
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
