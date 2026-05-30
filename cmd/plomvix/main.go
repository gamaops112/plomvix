package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	figure "github.com/common-nighthawk/go-figure"
	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/auth"
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/server"
	"github.com/plomvix/plomvix/pkg/utils"

	walmanager "github.com/plomvix/plomvix/internal/storage/wal"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Plomvix %s (built: %s, commit: %s, %s)\n",
			Version, BuildTime, GitCommit, utils.GetOSArch())
		return
	}

	fig := figure.NewFigure("Plomvix", "", true)
	fig.Print()
	fmt.Println()
	fmt.Println("  The Indian-built observability database.")
	fmt.Printf("  Version: %s\n\n", Version)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Logging.Level, cfg.Logging.Format); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("plomvix starting",
		zap.String("version", Version),
		zap.Int("pid", os.Getpid()),
		zap.String("env", cfg.Env),
		zap.String("build_time", BuildTime),
		zap.String("git_commit", GitCommit),
	)

	if err := bootstrapDataDirs(cfg); err != nil {
		logger.Error("failed to bootstrap data directories", zap.Error(err))
		os.Exit(1)
	}

	// Open user store
	store, err := auth.NewStore(
		filepath.Join(cfg.Storage.DataDir, "system", "auth.db"))
	if err != nil {
		logger.Error("failed to open user store", zap.Error(err))
		os.Exit(1)
	}

	// Bootstrap admin user
	if err := auth.BootstrapAdminUser(store, cfg); err != nil {
		store.Close()
		logger.Error("failed to bootstrap admin user", zap.Error(err))
		os.Exit(1)
	}
	defer store.Close()

	// Create blacklist
	blacklist := auth.NewBlacklist()
	defer blacklist.Stop()

	// Open WAL
	wal, err := walmanager.Open(
		filepath.Join(cfg.Storage.DataDir, "wal"), cfg)
	if err != nil {
		logger.Error("failed to open WAL", zap.Error(err))
		os.Exit(1)
	}

	// Run recovery — logs entries found, does not replay in Sprint 3
	entries, err := wal.Recover()
	if err != nil {
		wal.Close()
		logger.Error("WAL recovery failed", zap.Error(err))
		os.Exit(1)
	}
	defer wal.Close()

	logger.Info("WAL recovery complete",
		zap.Int("entries_recovered", len(entries)),
		zap.Int("segments_found", wal.Stats().SegmentCount),
	)
	_ = entries // suppress unused variable warning

	srv := server.New(cfg, Version, store, blacklist, wal)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	logger.Info("plomvix ready",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
		zap.String("env", cfg.Env),
	)

	select {
	case sig := <-quit:
		logger.Info("shutdown signal received",
			zap.String("signal", sig.String()))
	case err := <-serverErr:
		if err != nil {
			logger.Error("server error", zap.Error(err))
		}
	}

	logger.Info("shutting down plomvix", zap.Int("timeout_seconds", 30))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	cancel()

	logger.Info("plomvix stopped cleanly")
	logger.Sync()
}

func bootstrapDataDirs(cfg *config.Config) error {
	dirs := []string{
		filepath.Join(cfg.Storage.DataDir, "wal"),
		filepath.Join(cfg.Storage.DataDir, "hot"),
		filepath.Join(cfg.Storage.DataDir, "cold", "logs"),
		filepath.Join(cfg.Storage.DataDir, "cold", "metrics"),
		filepath.Join(cfg.Storage.DataDir, "cold", "json"),
		filepath.Join(cfg.Storage.DataDir, "cold", "kv"),
		filepath.Join(cfg.Storage.DataDir, "system"),
	}
	for _, dir := range dirs {
		if err := utils.EnsureDir(dir); err != nil {
			return fmt.Errorf("failed to create data directory %q: %w", dir, err)
		}
		logger.Debug("data directory ready", zap.String("path", dir))
	}
	return nil
}
