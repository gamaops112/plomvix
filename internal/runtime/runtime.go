// Package runtime composes Plomvix core foundations (config, logger, lifecycle)
// into a fully wired database daemon. It constructs the storage pager, KVStore,
// catalog, SQL engine, router, parser, and PG wire protocol server, registers
// them with the lifecycle manager, and starts them in dependency order.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/engine/sql"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/system"
	"github.com/plomvix/plomvix/internal/engine/sql/tx"
	"github.com/plomvix/plomvix/internal/engine/sql/vacuum"
	"github.com/plomvix/plomvix/internal/lifecycle"
	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/router"
	srv "github.com/plomvix/plomvix/internal/server"
	"github.com/plomvix/plomvix/internal/sqlparser"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// DefaultConfigPath is the default configuration file path.
const DefaultConfigPath = "config.toml"

// DefaultStartupTimeout is the default timeout for lifecycle start.
const DefaultStartupTimeout = 30 * time.Second

// DefaultShutdownTimeout is the default timeout for lifecycle stop.
const DefaultShutdownTimeout = 30 * time.Second

// Enterprise runtime errors for classified error handling.
var (
	ErrInvalidOptions  = errors.New("runtime: invalid options")
	ErrLoadConfig      = errors.New("runtime: load config")
	ErrCreateLogger    = errors.New("runtime: create logger")
	ErrStartLifecycle  = errors.New("runtime: start lifecycle")
	ErrStopLifecycle   = errors.New("runtime: stop lifecycle")
	ErrRuntimePanic    = errors.New("runtime: panic")
	ErrShutdownTimeout = errors.New("runtime: shutdown timeout")
)

// Options controls runtime behavior.
type Options struct {
	ConfigPath      string
	PortOverride    int // 0 = use config value; non-zero overrides cfg.Server.Port
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// DefaultOptions returns Options populated with safe defaults.
func DefaultOptions() Options {
	return Options{
		ConfigPath:      DefaultConfigPath,
		StartupTimeout:  DefaultStartupTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// resolveOptions applies defaults and validates the options.
func resolveOptions(opts Options) (Options, error) {
	if opts.ConfigPath == "" {
		opts.ConfigPath = DefaultConfigPath
	}
	if opts.StartupTimeout == 0 {
		opts.StartupTimeout = DefaultStartupTimeout
	}
	if opts.StartupTimeout < 0 {
		return Options{}, fmt.Errorf("%w: negative startup timeout", ErrInvalidOptions)
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = DefaultShutdownTimeout
	}
	if opts.ShutdownTimeout < 0 {
		return Options{}, fmt.Errorf("%w: negative shutdown timeout", ErrInvalidOptions)
	}
	return opts, nil
}

// Runtime holds the composed Plomvix runtime state.
type Runtime struct {
	opts    Options
	cfg     config.Config
	log     *slog.Logger
	manager *lifecycle.Manager
}

// New creates a Runtime by loading configuration, constructing all database
// and network components, registering them with the lifecycle manager in
// dependency order, but does NOT start anything.
func New(opts Options) (*Runtime, error) {
	resolved, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(resolved.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	// Apply CLI port override.
	if resolved.PortOverride != 0 {
		cfg.Server.Port = resolved.PortOverride
	}

	baseLog, err := logger.New(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateLogger, err)
	}
	log := logger.WithComponent(baseLog, "runtime")

	// Create boot context for system heaps initialization.
	bootCtx, cancel := context.WithTimeout(context.Background(), resolved.StartupTimeout)
	defer cancel()

	// 1. Initialize System Heaps & Global Catalog.
	if err := os.MkdirAll(cfg.SQL.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("runtime: create data dir: %w", err)
	}
	sysFactory := system.NewFactory(cfg.SQL.DataDir)
	sysTables, sysColumns, sysUsers, err := sysFactory.OpenOrCreateSystemHeaps(bootCtx)
	if err != nil {
		return nil, fmt.Errorf("runtime: init system heaps: %w", err)
	}
	cat := catalog.NewWithStores(sysTables, sysColumns, sysUsers)

	// 2. Initialize Custom Pager & KVStore for User Tables.
	if err := os.MkdirAll(filepath.Dir(cfg.Store.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("runtime: create data dir: %w", err)
	}
	sharedPager := pager.New(cfg.Store.DBPath)
	sharedKV := kv.New(sharedPager)

	// 3. Initialize HeapManager (TableRegistry), TxManager, Vacuum, and RowDecoder.
	heapMgr := sql.NewHeapManager(sharedKV, cfg.SQL.DataDir)
	txMgr := tx.NewManager(1, 1)
	vacMgr, err := vacuum.NewManager(cfg.SQL.VacuumWorkers, cfg.SQL.VacuumQueueSize)
	if err != nil {
		return nil, fmt.Errorf("runtime: create vacuum manager: %w", err)
	}
	planCache := planner.NewPlanCache(128)
	rowDecoder := sql.NewRowDecoder()

	// 4. Initialize SQL Engine & Register it with Catalog.
	sqlEngine, err := sql.NewSQLEngine(sql.SQLEngineConfig{
		Catalog:         cat,
		Versions:        cat,
		TableManager:    heapMgr,
		Decoder:         rowDecoder,
		PlanCache:       planCache,
		TxManager:       txMgr,
		VacuumManager:   vacMgr,
		Logger:          logger.WithComponent(baseLog, "sql-engine"),
		MaxBatchSize:    1000,
		MaxMutationRows: cfg.SQL.MaxMutationRows,
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: create sql engine: %w", err)
	}
	if err := cat.RegisterEngine(sqlEngine); err != nil {
		return nil, fmt.Errorf("runtime: register sql engine: %w", err)
	}

	// 5. Initialize Router & SQL Parser.
	routerService := router.New(cat)
	routerService.RegisterEngine(sqlEngine)
	parserService, err := sqlparser.New()
	if err != nil {
		return nil, fmt.Errorf("runtime: create parser: %w", err)
	}

	// 6. Initialize PG Wire Protocol Server.
	pgServer := srv.New(srv.ServerConfig{
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Router:         routerService,
		Parser:         parserService,
		Logger:         logger.WithComponent(baseLog, "pg-server"),
		MaxConnections: cfg.Server.MaxConnections,
	})

	// 7. Register components with lifecycle manager in dependency order.
	manager := lifecycle.NewManager()
	// Storage layer (start first, stop last).
	if err := manager.Register(&pagerComponent{p: sharedPager}); err != nil {
		return nil, err
	}
	if err := manager.Register(&kvComponent{store: sharedKV}); err != nil {
		return nil, err
	}
	// Catalog (after storage, before engine).
	if err := manager.Register(cat); err != nil {
		return nil, err
	}
	// Vacuum manager.
	if err := manager.Register(vacMgr); err != nil {
		return nil, err
	}
	// PG wire server (last to start, first to stop).
	if err := manager.Register(pgServer); err != nil {
		return nil, err
	}

	return &Runtime{
		opts:    resolved,
		cfg:     cfg,
		log:     log,
		manager: manager,
	}, nil
}

// Start begins the runtime lifecycle.
func (r *Runtime) Start(ctx context.Context) (err error) {
	defer recoverRuntimePanic("start", &err)

	if r == nil {
		return ErrInvalidOptions
	}

	r.log.Info("runtime starting")

	startCtx, cancel := context.WithTimeout(ctx, r.opts.StartupTimeout)
	defer cancel()

	if err := r.manager.Start(startCtx); err != nil {
		return fmt.Errorf("%w: %w", ErrStartLifecycle, err)
	}

	r.log.Info("runtime started")
	return nil
}

// Stop ends the runtime lifecycle (LIFO stop order).
func (r *Runtime) Stop(ctx context.Context) (err error) {
	defer recoverRuntimePanic("stop", &err)

	if r == nil {
		return ErrInvalidOptions
	}

	r.log.Info("runtime stopping")

	stopCtx, cancel := context.WithTimeout(ctx, r.opts.ShutdownTimeout)
	defer cancel()

	if err := r.manager.Stop(stopCtx); err != nil {
		if stopCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%w: %w: %w", ErrStopLifecycle, ErrShutdownTimeout, err)
		}
		return fmt.Errorf("%w: %w", ErrStopLifecycle, err)
	}

	r.log.Info("runtime stopped")
	return nil
}

// State returns the current lifecycle state.
func (r *Runtime) State() lifecycle.State {
	if r == nil {
		return lifecycle.StateFailed
	}
	return r.manager.State()
}

// pagerComponent wraps pager.Pager for lifecycle management.
type pagerComponent struct {
	p pager.Pager
}

func (c *pagerComponent) Name() string                    { return "storage.pager" }
func (c *pagerComponent) Start(ctx context.Context) error { return c.p.Open(ctx) }
func (c *pagerComponent) Stop(ctx context.Context) error  { return c.p.Close(ctx) }

// kvComponent wraps kv.KVStore for lifecycle management.
type kvComponent struct {
	store kv.KVStore
}

func (c *kvComponent) Name() string                    { return "storage.kv" }
func (c *kvComponent) Start(ctx context.Context) error { return c.store.Open(ctx) }
func (c *kvComponent) Stop(ctx context.Context) error  { return c.store.Close(ctx) }

// recoverRuntimePanic recovers from panics and wraps them as ErrRuntimePanic.
func recoverRuntimePanic(operation string, errp *error) {
	if r := recover(); r != nil {
		panicErr := fmt.Errorf("%w: %s panic: %v", ErrRuntimePanic, operation, r)
		if *errp == nil {
			*errp = panicErr
		} else {
			*errp = errors.Join(*errp, panicErr)
		}
	}
}

// Run loads configuration, creates a logger and lifecycle manager, starts the
// lifecycle, and then stops it before returning.
func Run(ctx context.Context, opts Options) (err error) {
	defer recoverRuntimePanic("run", &err)

	rt, err := New(opts)
	if err != nil {
		return err
	}

	if err := rt.Start(ctx); err != nil {
		_ = rt.Stop(ctx)
		return err
	}

	return rt.Stop(ctx)
}
