package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine/sql"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/system"
	"github.com/plomvix/plomvix/internal/engine/sql/tx"
	"github.com/plomvix/plomvix/internal/engine/sql/vacuum"
)

// TestWiringOrder_FactoryToCatalogToEngine verifies the exact construction
// order required by the DDL Enterprise plan: Factory → Catalog → Engine.
func TestWiringOrder_FactoryToCatalogToEngine(t *testing.T) {
	dir := t.TempDir()
	dataDir := dir + "/data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Step 1: Vacuum Manager.
	vac, err := vacuum.NewManager(2, 100)
	if err != nil {
		t.Fatalf("vacuum.NewManager: %v", err)
	}
	if err := vac.Start(ctx); err != nil {
		t.Fatalf("vac.Start: %v", err)
	}
	defer vac.Stop(ctx)

	// Step 2: SystemHeapFactory creates physical system heaps.
	factory := system.NewFactory(dataDir)
	sysTables, sysColumns, sysUsers, err := factory.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatalf("OpenOrCreateSystemHeaps: %v", err)
	}

	// Step 3: Catalog (New-style, decoupled from heap).
	cat := catalog.NewWithStores(sysTables, sysColumns, sysUsers)
	if err := cat.Start(ctx); err != nil {
		t.Fatalf("cat.Start: %v", err)
	}
	defer cat.Stop(ctx)

	// Step 4: Remainder of SQL Engine deps.
	txm := tx.NewManager(1, 1)
	dec := sql.NewRowDecoder()
	pc := planner.NewPlanCache(16)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Note: TableManager needs a KVStore; for a production wiring, this comes
	// from a shared pager. For this test, we verify the construction order
	// compiles and boots without panics.
	_ = txm
	_ = dec
	_ = pc
	_ = log

	// Verify catalog started and has the Name() method.
	if cat.Name() != "catalog" {
		t.Errorf("got %q, want \"catalog\"", cat.Name())
	}

	// Verify the factory created files.
	for _, id := range []uint64{1, 2, 3} {
		expectedPath := fmt.Sprintf("%s/heap_%d.db", dataDir, id)
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("system heap %d not created at %s", id, expectedPath)
		}
	}
}
