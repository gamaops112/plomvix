// Package vacuum provides a background file-deletion manager for the SQL
// engine. It implements lifecycle.Component and deletes orphaned physical
// heap files after DROP TABLE.
package vacuum

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/plomvix/plomvix/internal/systemids"
)

// State models the vacuum manager's lifecycle.
type State int

const (
	StateNew State = iota
	StateStarted
	StateStopped
)

// Sentinel errors.
var (
	ErrInvalidWorkerCount           = errors.New("vacuum: workers must be >= 1")
	ErrInvalidQueueSize             = errors.New("vacuum: queue size must be >= 1")
	ErrSystemTableDeletionForbidden = errors.New("vacuum: cannot delete reserved system table")
	ErrVacuumNotStarted             = errors.New("vacuum: manager not started")
	ErrVacuumAlreadyStarted         = errors.New("vacuum: manager already started")
	ErrVacuumStopped                = errors.New("vacuum: manager is stopped")
	ErrVacuumQueueFull              = errors.New("vacuum: deletion queue is full")
)

// DeletionRequest describes a file to remove.
type DeletionRequest struct {
	TableID  uint64
	FilePath string
}

// Manager runs background workers that delete heap files.
// It satisfies lifecycle.Component.
type Manager struct {
	mu      sync.Mutex
	state   State
	pending chan DeletionRequest

	pendingCount int64 // protected by mu
	workersWg    sync.WaitGroup
	workers      int
	stopCh       chan struct{}
}

// NewManager creates a vacuum Manager. Returns error if workers < 1 or queueSize < 1.
func NewManager(workers int, queueSize int) (*Manager, error) {
	if workers < 1 {
		return nil, ErrInvalidWorkerCount
	}
	if queueSize < 1 {
		return nil, ErrInvalidQueueSize
	}
	return &Manager{
		state:   StateNew,
		pending: make(chan DeletionRequest, queueSize),
		workers: workers,
		stopCh:  make(chan struct{}),
	}, nil
}

// Name returns the component name for lifecycle.Manager.
func (m *Manager) Name() string { return "vacuum" }

// Start launches background workers. Idempotent if already started.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStarted {
		return ErrVacuumAlreadyStarted
	}
	if m.state == StateStopped {
		return ErrVacuumStopped
	}
	for i := 0; i < m.workers; i++ {
		m.workersWg.Add(1)
		go m.worker()
	}
	m.state = StateStarted
	return nil
}

// Stop gracefully shuts down workers, draining the queue before returning.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.state != StateStarted {
		m.mu.Unlock()
		return ErrVacuumNotStarted
	}
	m.state = StateStopped
	m.mu.Unlock()

	close(m.stopCh)

	done := make(chan struct{})
	go func() {
		m.workersWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ScheduleDeletion adds a file to the deletion queue. Non-blocking: returns
// ErrVacuumQueueFull if the channel is full. Rejects system table IDs.
func (m *Manager) ScheduleDeletion(tableID uint64, filePath string) error {
	if tableID >= systemids.SystemTableMinID && tableID <= systemids.SystemTableMaxID {
		return ErrSystemTableDeletionForbidden
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateStarted {
		if m.state == StateNew {
			return ErrVacuumNotStarted
		}
		return ErrVacuumStopped
	}

	select {
	case m.pending <- DeletionRequest{TableID: tableID, FilePath: filePath}:
		m.pendingCount++
		return nil
	default:
		return ErrVacuumQueueFull
	}
}

// Drain blocks until all enqueued jobs are processed or ctx is cancelled.
// Test-only: uses polling to avoid sync.Cond and helper goroutine leaks.
func (m *Manager) Drain(ctx context.Context) error {
	for {
		m.mu.Lock()
		count := m.pendingCount
		m.mu.Unlock()

		if count == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (m *Manager) worker() {
	defer m.workersWg.Done()
	for {
		select {
		case req := <-m.pending:
			m.process(req)
		case <-m.stopCh:
			// Drain remaining items before exiting.
			for {
				select {
				case req := <-m.pending:
					m.process(req)
				default:
					return
				}
			}
		}
	}
}

func (m *Manager) process(req DeletionRequest) {
	_ = os.Remove(req.FilePath)
	m.mu.Lock()
	m.pendingCount--
	m.mu.Unlock()
}

// PendingCount returns the current number of enqueued deletion requests.
func (m *Manager) PendingCount() int64 {
	return atomic.LoadInt64(&m.pendingCount)
}

var _ interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
} = (*Manager)(nil)
