package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
)

type WALStats struct {
	SegmentCount    int
	ActiveSegment   uint64
	ActiveSizeBytes int64
	TotalEntries    int64
}

type Manager struct {
	writer       *Writer
	dir          string
	totalEntries atomic.Int64
}

func Open(dir string, cfg *config.Config) (*Manager, error) {
	if err := utils.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	writer, err := NewWriter(dir, cfg.Storage.WALFlushThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL writer: %w", err)
	}

	return &Manager{
		writer: writer,
		dir:    dir,
	}, nil
}

func (m *Manager) Write(dataType DataType, payload []byte) (*Entry, error) {
	entry, err := m.writer.Write(dataType, payload)
	if err != nil {
		return nil, err
	}
	m.totalEntries.Add(1)
	return entry, nil
}

func (m *Manager) Recover() ([]*Entry, error) {
	r, err := NewReader(m.dir)
	if err != nil {
		return nil, err
	}
	return r.ReadAll()
}

func (m *Manager) DeleteSegment(index uint64) error {
	path := filepath.Join(m.dir, SegmentFileName(index))
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete WAL segment %s: %w", SegmentFileName(index), err)
	}
	return nil
}

// Rotate forces the WAL to close the current segment and open a new one,
// regardless of the current segment size.
//
// Important: Writer.rotate() requires the caller to hold writer.mu.
// Manager.Rotate is the public safe wrapper used by admin APIs.
func (m *Manager) Rotate() error {
	if m == nil || m.writer == nil {
		return fmt.Errorf("WAL writer not initialized")
	}

	m.writer.mu.Lock()
	defer m.writer.mu.Unlock()

	if m.writer.currentFile == nil {
		return fmt.Errorf("WAL writer is closed")
	}
	return m.writer.rotate()
}

func (m *Manager) Close() error {
	return m.writer.Close()
}

func (m *Manager) Stats() WALStats {
	segments, _ := listSegments(m.dir)
	return WALStats{
		SegmentCount:    len(segments),
		ActiveSegment:   m.writer.CurrentSegmentIndex(),
		ActiveSizeBytes: m.writer.CurrentSize(),
		TotalEntries:    m.totalEntries.Load(),
	}
}
