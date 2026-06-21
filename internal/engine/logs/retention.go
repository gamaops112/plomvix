// Package logs provides a pluggable logs engine for Plomvix.
// retention.go implements the background log retention worker that
// periodically sweeps and frees expired page blocks older than
// the configured retention window.
package logs

import (
	"context"
	"sync"
	"time"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

// BlockDirectoryEntry tracks metadata for a compressed log block.
type BlockDirectoryEntry struct {
	StartPageID  uint64
	PageCount    uint32
	MinTimestamp int64
	MaxTimestamp int64
}

// RetentionConfig holds configuration for the retention worker.
type RetentionConfig struct {
	RetentionDays   int
	CleanupInterval time.Duration
}

// DefaultRetentionConfig returns sensible defaults.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RetentionDays:   7,
		CleanupInterval: 24 * time.Hour,
	}
}

// RetentionWorker periodically sweeps log pages older than the configured
// retention window. It uses the block directory to identify expired blocks
// and frees their backing pages via the Pager API.
type RetentionWorker struct {
	cfg           RetentionConfig
	store         *LogsStore
	pgr           pager.Pager
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	lastSweepTime int64
}

// NewRetentionWorker creates a new retention worker.
func NewRetentionWorker(cfg RetentionConfig, store *LogsStore, p pager.Pager) *RetentionWorker {
	return &RetentionWorker{
		cfg:   cfg,
		store: store,
		pgr:   p,
		done:  make(chan struct{}),
	}
}

// Name returns the component name for lifecycle management.
func (w *RetentionWorker) Name() string { return "logs.retention" }

// Start begins the background retention loop.
func (w *RetentionWorker) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	go func() {
		defer close(w.done)
		ticker := time.NewTicker(w.cfg.CleanupInterval)
		defer ticker.Stop()

		// Run an initial sweep on startup.
		if err := w.sweep(w.ctx); err != nil {
			// Logging would go here in production.
			_ = err
		}

		for {
			select {
			case <-ticker.C:
				if err := w.sweep(w.ctx); err != nil {
					_ = err
				}
			case <-w.ctx.Done():
				return
			}
		}
	}()
	return nil
}

// Stop signals the retention worker to stop and waits for completion.
func (w *RetentionWorker) Stop(ctx context.Context) error {
	if w.cancel != nil {
		w.cancel()
	}
	if w.done != nil {
		select {
		case <-w.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// sweep performs a single retention pass.
func (w *RetentionWorker) sweep(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -w.cfg.RetentionDays).Unix()

	w.store.mu.Lock()
	defer w.store.mu.Unlock()

	var activeBlocks []BlockDirectoryEntry
	for _, entry := range w.store.blockDirectory {
		if entry.MaxTimestamp < cutoff {
			// Free all physical pages spanned by this block.
			for i := uint32(0); i < entry.PageCount; i++ {
				pageID := entry.StartPageID + uint64(i)
				if err := w.pgr.FreePage(ctx, pageID); err != nil {
					// Tombstone tolerance: pages may already be freed or invalid.
					// Only return hard errors for non-page-level issues.
					_ = err
				}
			}
		} else {
			activeBlocks = append(activeBlocks, entry)
		}
	}

	w.store.blockDirectory = activeBlocks
	w.lastSweepTime = time.Now().Unix()

	// Also sweep the token index of expired entries.
	if w.store.tokenIndex != nil {
		w.store.tokenIndex.Sweep(cutoff)
	}

	return nil
}

// Sweep runs a single retention pass (exported for testing).
func (w *RetentionWorker) Sweep(ctx context.Context) error {
	return w.sweep(ctx)
}

// LastSweepTime returns the Unix timestamp of the last completed sweep.
func (w *RetentionWorker) LastSweepTime() int64 {
	w.store.mu.RLock()
	defer w.store.mu.RUnlock()
	return w.lastSweepTime
}

// runRetentionLoop is the background goroutine entry point.
func (w *RetentionWorker) runRetentionLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = w.sweep(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// sweepWithDirectory is a helper that lets external callers trigger
// a sweep against a given directory and pager (used for testing).
func sweepWithDirectory(ctx context.Context, dir *[]BlockDirectoryEntry, mu *sync.RWMutex, p pager.Pager, tokenIdx *TokenIndex, cutoff int64) error {
	mu.Lock()
	defer mu.Unlock()

	var active []BlockDirectoryEntry
	for _, entry := range *dir {
		if entry.MaxTimestamp < cutoff {
			for i := uint32(0); i < entry.PageCount; i++ {
				pageID := entry.StartPageID + uint64(i)
				_ = p.FreePage(ctx, pageID)
			}
		} else {
			active = append(active, entry)
		}
	}
	*dir = active

	if tokenIdx != nil {
		tokenIdx.Sweep(cutoff)
	}
	return nil
}
