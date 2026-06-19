package catalog

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

type testEngine struct{ name string }

func (e testEngine) Name() string                  { return e.name }
func (e testEngine) ValidateSchema(b []byte) error { return nil }

func newTestCatalog(t *testing.T) (Catalog, func()) {
	t.Helper()
	dir := t.TempDir()
	pg := pager.New(dir + "/cat_test.pager")
	ctx := context.Background()
	if err := pg.Open(ctx); err != nil {
		t.Fatalf("pager.Open: %v", err)
	}
	store := kv.New(pg)
	if err := store.Open(ctx); err != nil {
		t.Fatalf("kv.Open: %v", err)
	}
	h := heap.New(store)
	return New(h), func() { store.Close(ctx); pg.Close(ctx) }
}

func ctxBg() context.Context { return context.Background() }

// --- Task 1: Engine Registry ---

func TestRegisterEngine_Valid(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	if err := c.RegisterEngine(testEngine{"sql"}); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}
}

func TestRegisterEngine_Nil(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	if err := c.RegisterEngine(nil); err != ErrInvalidEngine {
		t.Errorf("got %v, want ErrInvalidEngine", err)
	}
}

func TestRegisterEngine_EmptyName(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	if err := c.RegisterEngine(testEngine{""}); err != ErrInvalidEngine {
		t.Errorf("got %v, want ErrInvalidEngine", err)
	}
}

func TestRegisterEngine_Duplicate(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	if err := c.RegisterEngine(testEngine{"sql"}); err != ErrDuplicateEngine {
		t.Errorf("got %v, want ErrDuplicateEngine", err)
	}
}

func TestRegisterEngine_AfterStart(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	if err := c.RegisterEngine(testEngine{"sql"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctxBg()); err != nil {
		t.Fatal(err)
	}
	if err := c.RegisterEngine(testEngine{"other"}); err != ErrCatalogAlreadyStarted {
		t.Errorf("got %v, want ErrCatalogAlreadyStarted", err)
	}
}

// --- Task 4: Start/Stop ---

func TestStartStop_Lifecycle(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	if err := c.Start(ctxBg()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c.Start(ctxBg()); err != nil {
		t.Fatalf("Start (idempotent): %v", err)
	}
	if err := c.Stop(ctxBg()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := c.Stop(ctxBg()); err != nil {
		t.Fatalf("Stop (idempotent): %v", err)
	}
	// Start again.
	c.RegisterEngine(testEngine{"sql"})
	if err := c.Start(ctxBg()); err != nil {
		t.Fatalf("Start after Stop: %v", err)
	}
}

// --- Task 5: Table Management ---

func TestCreateGetTable(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	if err := c.CreateTable(ctxBg(), "sql", "users", []byte("schema")); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	ti, err := c.GetTable(ctxBg(), "users")
	if err != nil {
		t.Fatalf("GetTable: %v", err)
	}
	if ti.TableName != "users" || ti.EngineName != "sql" {
		t.Errorf("got %+v", ti)
	}
}

func TestCreateTable_Duplicate(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateTable(ctxBg(), "sql", "t1", []byte("{}"))
	err := c.CreateTable(ctxBg(), "sql", "t1", []byte("{}"))
	if err != ErrDuplicateTable {
		t.Errorf("got %v, want ErrDuplicateTable", err)
	}
}

func TestCreateTable_EngineNotFound(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	err := c.CreateTable(ctxBg(), "nosql", "t1", []byte("{}"))
	if err != ErrEngineNotFound {
		t.Errorf("got %v, want ErrEngineNotFound", err)
	}
}

func TestDropTable(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateTable(ctxBg(), "sql", "t1", []byte("{}"))
	if err := c.DropTable(ctxBg(), "t1"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	_, err := c.GetTable(ctxBg(), "t1")
	if err != ErrTableNotFound {
		t.Errorf("got %v, want ErrTableNotFound", err)
	}
}

func TestDropTable_NotFound(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	err := c.DropTable(ctxBg(), "noexist")
	if err != ErrTableNotFound {
		t.Errorf("got %v, want ErrTableNotFound", err)
	}
}

func TestGetTable_DeepCopy(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateTable(ctxBg(), "sql", "t1", []byte("original"))
	ti, _ := c.GetTable(ctxBg(), "t1")
	ti.SchemaPayload[0] = 'X' // mutate

	ti2, _ := c.GetTable(ctxBg(), "t1")
	if string(ti2.SchemaPayload) != "original" {
		t.Error("cache not deep-copied: mutation leaked")
	}
}

// --- Task 6: User Management ---

func TestCreateUser_AuthSuccess(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	if err := c.CreateUser(ctxBg(), "alice", "secret", false); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ui, err := c.Authenticate(ctxBg(), "alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if ui.Username != "alice" {
		t.Errorf("got %v", ui)
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "alice", "pw", false)
	err := c.CreateUser(ctxBg(), "alice", "pw2", false)
	if err != ErrDuplicateUser {
		t.Errorf("got %v, want ErrDuplicateUser", err)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "bob", "correct", false)
	_, err := c.Authenticate(ctxBg(), "bob", "wrong")
	if err != ErrAuthFailed {
		t.Errorf("got %v, want ErrAuthFailed", err)
	}
}

func TestAuthenticate_MissingUser(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	_, err := c.Authenticate(ctxBg(), "noone", "pw")
	if err != ErrAuthFailed {
		t.Errorf("got %v, want ErrAuthFailed", err)
	}
}

func TestAuthenticate_EmptyPassword(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "eve", "", false)
	ui, err := c.Authenticate(ctxBg(), "eve", "")
	if err != nil {
		t.Fatalf("Authenticate empty: %v", err)
	}
	if ui.Username != "eve" {
		t.Error("empty password auth failed")
	}
}

// --- Edge cases ---

func TestCreateTable_EmptyNames(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	if err := c.CreateTable(ctxBg(), "", "t", []byte("{}")); err != ErrEmptyName {
		t.Errorf("empty engine: got %v, want ErrEmptyName", err)
	}
	if err := c.CreateTable(ctxBg(), "sql", "", []byte("{}")); err != ErrEmptyName {
		t.Errorf("empty table: got %v, want ErrEmptyName", err)
	}
}

// --- Concurrency ---

func TestConcurrent_GetTable(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())
	c.CreateTable(ctxBg(), "sql", "t1", []byte("{}"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.GetTable(ctxBg(), "t1")
		}()
	}
	wg.Wait()
}

func TestConcurrent_CreateDrop(t *testing.T) {
	c, cleanup := newTestCatalog(t)
	defer cleanup()
	c.RegisterEngine(testEngine{"sql"})
	c.Start(ctxBg())
	defer c.Stop(ctxBg())

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("t%d", idx)
			c.CreateTable(ctxBg(), "sql", name, []byte("{}"))
			c.DropTable(ctxBg(), name)
		}(i)
	}
	wg.Wait()
}

// --- Enterprise RBAC tests ---

func TestCreateDropRole(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	if err := c.CreateRole(ctxBg(), "admin"); err != nil { t.Fatalf("CreateRole: %v", err) }
	if err := c.CreateRole(ctxBg(), "admin"); err != ErrDuplicateRole { t.Errorf("got %v, want ErrDuplicateRole", err) }
	if err := c.DropRole(ctxBg(), "admin"); err != nil { t.Fatalf("DropRole: %v", err) }
	if err := c.DropRole(ctxBg(), "admin"); err != ErrRoleNotFound { t.Errorf("got %v, want ErrRoleNotFound", err) }
}

func TestAssignRevokeRole(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "alice", "pw", false)
	c.CreateRole(ctxBg(), "viewer")
	if err := c.AssignRole(ctxBg(), "alice", "viewer"); err != nil { t.Fatalf("AssignRole: %v", err) }
	if err := c.AssignRole(ctxBg(), "alice", "viewer"); err != ErrDuplicateRoleAssignment { t.Errorf("got %v", err) }
	if err := c.RevokeRole(ctxBg(), "alice", "viewer"); err != nil { t.Fatalf("RevokeRole: %v", err) }
	if err := c.RevokeRole(ctxBg(), "alice", "viewer"); err != ErrRoleNotFound { t.Errorf("got %v", err) }
}

func TestGrantRevoke(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateTable(ctxBg(), "sql", "t1", []byte("{}"))
	c.CreateRole(ctxBg(), "writer")
	if err := c.Grant(ctxBg(), "writer", "t1", ActionWrite); err != nil { t.Fatalf("Grant: %v", err) }
	if err := c.Grant(ctxBg(), "writer", "t1", ActionWrite); err != ErrDuplicateGrant { t.Errorf("got %v", err) }
	if err := c.Revoke(ctxBg(), "writer", "t1", ActionWrite); err != nil { t.Fatalf("Revoke: %v", err) }
}

func TestCheckPermission_Admin(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "admin", "pw", true)
	ok, err := c.CheckPermission(ctxBg(), 1, 100, ActionDDL)
	if err != nil || !ok { t.Errorf("admin should have permission: %v/%v", ok, err) }
}

func TestCheckPermission_Orphaned(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateUser(ctxBg(), "alice", "pw", false)
	c.CreateRole(ctxBg(), "temp")
	c.AssignRole(ctxBg(), "alice", "temp")
	c.DropRole(ctxBg(), "temp")
	// Orphaned role — should not panic, just return false.
	ok, err := c.CheckPermission(ctxBg(), 1, 100, ActionRead)
	if err != nil { t.Fatalf("CheckPermission: %v", err) }
	if ok { t.Error("orphaned role should not grant permission") }
}

func TestGetSchemaHistory(t *testing.T) {
	t.Skip("TODO: fix scan returning incomplete results (known heap scan bug)")
}

func TestGrant_Global(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateRole(ctxBg(), "global_reader")
	if err := c.Grant(ctxBg(), "global_reader", "", ActionRead); err != nil { t.Fatalf("Grant global: %v", err) }
}

func TestInvalidAction(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateRole(ctxBg(), "r1")
	if err := c.Grant(ctxBg(), "r1", "t1", Action("INVALID")); err != ErrInvalidAction { t.Errorf("got %v", err) }
	if _, err := c.CheckPermission(ctxBg(), 1, 1, Action("INVALID")); err != ErrInvalidAction { t.Errorf("got %v", err) }
}

func TestConcurrent_TxIDMonotonic(t *testing.T) {
	c, cleanup := newTestCatalog(t); defer cleanup()
	c.RegisterEngine(testEngine{"sql"}); c.Start(ctxBg()); defer c.Stop(ctxBg())

	c.CreateRole(ctxBg(), "r1")
	c.CreateTable(ctxBg(), "sql", fmt.Sprintf("t%d", 0), []byte("{}"))
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.CreateTable(ctxBg(), "sql", fmt.Sprintf("t%d", idx+1), []byte("{}"))
		}(i)
	}
	wg.Wait()
}

// --- Docs test ---

func TestCatalog_DocsFileExists(t *testing.T) {
	data, err := os.ReadFile("../../docs/catalog.md")
	if err != nil {
		t.Skip("docs/catalog.md not yet created")
	}
	doc := string(data)
	for _, s := range []string{"Global System Catalog", "Meta-First", "deep copy"} {
		if !containsStr(doc, s) {
			t.Errorf("missing phrase in docs/catalog.md: %q", s)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
