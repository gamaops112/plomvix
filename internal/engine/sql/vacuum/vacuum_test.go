package vacuum

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager_Valid(t *testing.T) {
	m, err := NewManager(2, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m == nil {
		t.Fatal("nil manager")
	}
}

func TestNewManager_InvalidWorkers(t *testing.T) {
	_, err := NewManager(0, 10)
	if err != ErrInvalidWorkerCount {
		t.Errorf("got %v, want ErrInvalidWorkerCount", err)
	}
}

func TestNewManager_InvalidQueueSize(t *testing.T) {
	_, err := NewManager(2, 0)
	if err != ErrInvalidQueueSize {
		t.Errorf("got %v, want ErrInvalidQueueSize", err)
	}
}

func TestScheduleDeletion_SystemTableForbidden(t *testing.T) {
	m, err := NewManager(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Start(ctx)

	err = m.ScheduleDeletion(1, "/tmp/test.db")
	if err != ErrSystemTableDeletionForbidden {
		t.Errorf("got %v, want ErrSystemTableDeletionForbidden", err)
	}
}

func TestScheduleDeletion_NotStarted(t *testing.T) {
	m, err := NewManager(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	err = m.ScheduleDeletion(1000, "/tmp/test.db")
	if err != ErrVacuumNotStarted {
		t.Errorf("got %v, want ErrVacuumNotStarted", err)
	}
}

func TestScheduleDeletion_Stopped(t *testing.T) {
	m, err := NewManager(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Start(ctx)
	_ = m.Stop(ctx)

	err = m.ScheduleDeletion(1000, "/tmp/test.db")
	if err != ErrVacuumStopped {
		t.Errorf("got %v, want ErrVacuumStopped", err)
	}
}

func TestScheduleDeletion_AlreadyStarted(t *testing.T) {
	m, err := NewManager(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Second start should fail.
	if err := m.Start(ctx); err != ErrVacuumAlreadyStarted {
		t.Errorf("got %v, want ErrVacuumAlreadyStarted", err)
	}
	_ = m.Stop(ctx)
}

func TestScheduleDeletion_QueueFull(t *testing.T) {
	m, err := NewManager(1, 1) // queue size 1
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Start(ctx)
	defer m.Stop(ctx)

	// Fill the queue (worker not started, so items stay).
	_ = m.ScheduleDeletion(1000, "/tmp/a.db")
	err = m.ScheduleDeletion(1001, "/tmp/b.db")
	if err != ErrVacuumQueueFull {
		t.Errorf("got %v, want ErrVacuumQueueFull", err)
	}
}

func TestDrain_Empty(t *testing.T) {
	m, err := NewManager(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Drain(ctx); err != nil {
		t.Errorf("Drain on empty: %v", err)
	}
}

func TestEndToEnd_FileDeleted(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test_heap.db")
	// Create a dummy file.
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if err := m.ScheduleDeletion(1000, testFile); err != nil {
		t.Fatal(err)
	}

	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.Drain(drainCtx); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
	_ = m.Stop(ctx)
}

func TestStop_DrainsQueue(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, fmt.Sprintf("file_%d.db", i))
		os.WriteFile(f, []byte("data"), 0644)
		files = append(files, f)
	}

	m, err := NewManager(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Start(ctx)

	for _, f := range files {
		_ = m.ScheduleDeletion(1000, f)
	}

	// Stop should drain.
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("file %s should be deleted", f)
		}
	}
}

func TestName(t *testing.T) {
	m, _ := NewManager(1, 1)
	if m.Name() != "vacuum" {
		t.Errorf("got %q, want \"vacuum\"", m.Name())
	}
}
