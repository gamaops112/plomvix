package cold

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
	hotstore "github.com/plomvix/plomvix/internal/storage/hot"
)

// TieringEngine moves aged data from the hot tier to the cold tier.
// Only logs, metrics, and json are eligible — KV is excluded.
type TieringEngine struct {
	hot  *hotstore.Manager
	cold *Store
	cfg  *config.Config
	done chan struct{}
	once sync.Once // guards Stop() — prevents double-close panic
}

// NewTieringEngine creates a TieringEngine.
func NewTieringEngine(hot *hotstore.Manager, cold *Store, cfg *config.Config) *TieringEngine {
	return &TieringEngine{
		hot:  hot,
		cold: cold,
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

// Start launches the background tiering goroutine.
// Runs Flush every hour. Call Stop() to shut it down.
func (e *TieringEngine) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := e.Flush(); err != nil {
					logger.Error("background tiering failed", zap.Error(err))
				}
			case <-e.done:
				return
			}
		}
	}()
}

// Stop signals the background goroutine to exit.
// Safe to call multiple times — uses sync.Once to prevent double-close panic.
func (e *TieringEngine) Stop() {
	e.once.Do(func() { close(e.done) })
}

// Flush moves all eligible hot tier data (logs, metrics, json) to the cold tier.
// Eligibility: timestamp < now - retention_days.
// Deletion failure is treated as a flush error to prevent hot+cold duplicates.
func (e *TieringEngine) Flush() error {
	start := time.Now()

	cutoffNs := time.Now().Add(
		-time.Duration(e.cfg.Storage.RetentionDays) * 24 * time.Hour,
	).UnixNano()

	dataTypes := []struct {
		cf       string
		dataType string
	}{
		{hotstore.CFLogs, DataTypeLogs},
		{hotstore.CFMetrics, DataTypeMetrics},
		{hotstore.CFJSON, DataTypeJSON},
		// KV excluded: KV keys have no timestamp prefix
	}

	totalMoved := int64(0)
	for _, dt := range dataTypes {
		moved, err := e.flushDataType(dt.cf, dt.dataType, cutoffNs)
		if err != nil {
			return fmt.Errorf("flush failed for %s: %w", dt.dataType, err)
		}
		totalMoved += moved
	}

	dur := time.Since(start)
	e.cold.SetLastFlush(start, dur)
	e.cold.AddRecordsMoved(totalMoved)

	if totalMoved > 0 {
		logger.Info("tiering flush complete",
			zap.Int64("records_moved", totalMoved),
			zap.Duration("duration", dur),
		)
	} else {
		logger.Debug("tiering flush: no eligible records")
	}
	return nil
}

// flushDataType moves eligible records from one CF to cold storage.
// Computes refTs from the OLDEST row's timestamp for correct date partitioning.
// Deletion failure aborts the flush and returns an error.
func (e *TieringEngine) flushDataType(cf, dataType string, cutoffNs int64) (int64, error) {
	var rows []ParquetRow
	var keysToDelete [][]byte

	err := e.hot.ScanCFWithKeys(cf, 0, cutoffNs, func(key, payload []byte) bool {
		if len(key) < 8 {
			logger.Warn("skipping malformed hot tier key",
				zap.String("cf", cf),
				zap.Int("key_len", len(key)),
			)
			return true
		}

		rows = append(rows, ParquetRow{
			TimestampNs: int64(bigEndianUint64(key[:8])),
			Payload:     string(payload),
		})

		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		keysToDelete = append(keysToDelete, keyCopy)
		return true
	})
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// Use the OLDEST row's timestamp for date partitioning — not the cutoff date.
	oldestNs := rows[0].TimestampNs
	for _, row := range rows[1:] {
		if row.TimestampNs < oldestNs {
			oldestNs = row.TimestampNs
		}
	}
	refTs := time.Unix(0, oldestNs).UTC()

	if err := e.cold.WriteRows(dataType, rows, refTs); err != nil {
		return 0, fmt.Errorf("failed to write %s to cold tier: %w", dataType, err)
	}

	// Delete from hot tier — failure is an error, not a warning.
	// This prevents the same record appearing in both tiers.
	for _, key := range keysToDelete {
		if err := e.hot.DeleteFromCF(cf, key); err != nil {
			return 0, fmt.Errorf("failed to delete tiered record from hot tier cf=%s: %w", cf, err)
		}
	}

	return int64(len(rows)), nil
}

// bigEndianUint64 decodes a big-endian uint64 from the first 8 bytes of b.
func bigEndianUint64(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}
