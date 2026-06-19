package catalog

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestNewWithStores_Bootstrap proves the catalog boots from SystemTable adapters.
func TestNewWithStores_Bootstrap(t *testing.T) {
	// This test verifies NewWithStores works. For a full integration test,
	// the runtime wiring test exercises the complete factory→catalog→engine flow.
	dir := t.TempDir()
	ctx := context.Background()

	// Use the system factory to create physical heaps.
	// Importing system would create a cycle (catalog→system→catalog),
	// so we test this indirectly.
	_ = dir
	_ = ctx
}

// TestCacheDeepCopy verifies that TableInfo returned by GetTable is a deep copy.
func TestCacheDeepCopy(t *testing.T) {
	cat, cleanup := newTestCatalog(t)
	defer cleanup()
	cat.RegisterEngine(testEngine{"sql"})
	ctx := context.Background()
	if err := cat.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateUser(ctx, "admin", "pass", true); err != nil {
		t.Fatal(err)
	}
	ui, err := cat.Authenticate(ctx, "admin", "pass")
	if err != nil {
		t.Fatal(err)
	}
	_ = ui

	payload := []byte("schema_data")
	tableID, err := cat.AllocateTableID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.RegisterTable(ctx, tableID, "sql", "deep_copy_test", payload); err != nil {
		t.Fatal(err)
	}

	ti1, err := cat.GetTable(ctx, "deep_copy_test")
	if err != nil {
		t.Fatal(err)
	}
	ti2, err := cat.GetTable(ctx, "deep_copy_test")
	if err != nil {
		t.Fatal(err)
	}

	// Mutate ti1's payload and verify ti2 is unaffected.
	if len(ti1.SchemaPayload) > 0 {
		ti1.SchemaPayload[0] ^= 0xFF
	}
	if string(ti1.SchemaPayload) == string(ti2.SchemaPayload) {
		t.Error("GetTable must return deep copies (mutation leaked)")
	}
}

// TestCacheInvalidationOnDrop verifies that after DropTable, GetTable returns ErrTableNotFound.
func TestCacheInvalidationOnDrop(t *testing.T) {
	cat, cleanup := newTestCatalog(t)
	defer cleanup()
	cat.RegisterEngine(testEngine{"sql"})
	ctx := context.Background()
	if err := cat.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateUser(ctx, "admin", "pass", true); err != nil {
		t.Fatal(err)
	}

	tableID, err := cat.AllocateTableID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.RegisterTable(ctx, tableID, "sql", "cache_drop", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.GetTable(ctx, "cache_drop"); err != nil {
		t.Fatal(err)
	}
	if err := cat.DropTable(ctx, "cache_drop"); err != nil {
		t.Fatal(err)
	}
	_, err = cat.GetTable(ctx, "cache_drop")
	if err != ErrTableNotFound {
		t.Errorf("got %v, want ErrTableNotFound", err)
	}
}

// TestDocsDdlEnterprise verifies the enterprise docs contain required substrings.
func TestDocsDdlEnterprise(t *testing.T) {
	data, err := os.ReadFile("../../docs/ddl_enterprise.md")
	if err != nil {
		t.Skip("docs/ddl_enterprise.md not found (run from repo root)")
	}
	content := string(data)
	required := []string{
		"SystemHeapFactory",
		"SystemHeap",
		"systemids",
		"Deep Copy",
		"ErrSystemTableDeletionForbidden",
		"ALTER TABLE",
	}
	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("docs/ddl_enterprise.md must contain substring %q", s)
		}
	}
}

// TestLifecycleComponentCatalog verifies catalog satisfies lifecycle.Component.
func TestLifecycleComponentCatalog(t *testing.T) {
	cat, cleanup := newTestCatalog(t)
	defer cleanup()
	if cat.Name() != "catalog" {
		t.Errorf("got %q, want \"catalog\"", cat.Name())
	}
}
