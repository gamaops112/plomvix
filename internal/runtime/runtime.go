// Package runtime composes Plomvix core foundations (config, logger, lifecycle)
// into a minimal runnable entrypoint. It does not own database engines, storage,
// WAL, query execution, API servers, or UI.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/lifecycle"
	"github.com/plomvix/plomvix/internal/logger"
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

// New creates a Runtime by loading configuration, creating a logger, and
// initializing a lifecycle manager. It does not start the lifecycle.
func New(opts Options) (*Runtime, error) {
	resolved, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(resolved.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	baseLog, err := logger.New(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateLogger, err)
	}

	log := logger.WithComponent(baseLog, "runtime")

	return &Runtime{
		opts:    resolved,
		cfg:     cfg,
		log:     log,
		manager: lifecycle.NewManager(),
	}, nil
}

// Start begins the runtime lifecycle. It creates a context with the startup
// timeout and starts the lifecycle manager.
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

// Stop ends the runtime lifecycle. It creates a context with the shutdown
// timeout and stops the lifecycle manager. Stop is safe to call after a
// failed start.
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
// lifecycle, and then stops it before returning. No production components are
// registered in this minimal composition.
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
