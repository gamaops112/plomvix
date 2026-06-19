package sql

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
	"github.com/plomvix/plomvix/internal/engine/sql/tx"
	"github.com/plomvix/plomvix/internal/sqlparser"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

type testEngine struct{ name string }

func (e testEngine) Name() string                  { return e.name }
func (e testEngine) ValidateSchema(b []byte) error { return nil }

// newTestSQLEngine creates a fully wired SQLEngine for testing.
// Returns the engine, catalog, and the admin UserInfo for DDL operations.
func newTestSQLEngine(t *testing.T) (*SQLEngine, Catalog, catalog.UserInfo, func()) {
	t.Helper()
	dir := t.TempDir()
	pg := pager.New(dir + "/ddl_test.pager")
	ctx := context.Background()
	if err := pg.Open(ctx); err != nil {
		t.Fatalf("pager.Open: %v", err)
	}
	store := kv.New(pg)
	if err := store.Open(ctx); err != nil {
		t.Fatalf("kv.Open: %v", err)
	}
	h := heap.New(store)
	cat := catalog.New(h)
	cat.RegisterEngine(testEngine{"sql"})
	if err := cat.Start(ctx); err != nil {
		t.Fatalf("cat.Start: %v", err)
	}
	// Create admin user for DDL permissions.
	if err := cat.CreateUser(ctx, "admin", "password", true); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	adminUI, err := cat.Authenticate(ctx, "admin", "password")
	if err != nil {
		t.Fatalf("Authenticate admin: %v", err)
	}
	tm := NewHeapManager(store)
	txm := tx.NewManager(1, 1)
	dec := NewRowDecoder()
	pc := planner.NewPlanCache(16)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	eng, err := NewSQLEngine(cat, cat, tm, dec, pc, txm, log)
	if err != nil {
		t.Fatalf("NewSQLEngine: %v", err)
	}
	return eng, cat, adminUI, func() { store.Close(ctx); pg.Close(ctx) }
}

func parseStmt(t *testing.T, sql string) sqlparser.Statement {
	t.Helper()
	p, err := sqlparser.New()
	if err != nil {
		t.Fatalf("sqlparser.New: %v", err)
	}
	stmt, err := p.Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", sql, err)
	}
	return stmt
}

func parseStmtErr(t *testing.T, sql string) error {
	t.Helper()
	p, err := sqlparser.New()
	if err != nil {
		t.Fatalf("sqlparser.New: %v", err)
	}
	_, err = p.Parse(sql)
	return err
}

func TestDDL_CreateTable(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	stmt := parseStmt(t, "CREATE TABLE users (id bigint, name varchar(255))")
	result, err := eng.Execute(ctx, &engine.Request{
		Stmt:   stmt,
		UserID: adminUI.UserID,
	})
	if err != nil {
		t.Fatalf("Execute CREATE TABLE: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Stream != nil {
		t.Error("DDL should not return a stream")
	}
	if !strings.Contains(result.Message, "CREATE TABLE") {
		t.Errorf("message should contain CREATE TABLE, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "table_id=") {
		t.Errorf("message should contain table_id, got %q", result.Message)
	}
}

func TestDDL_DropTable(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	createStmt := parseStmt(t, "CREATE TABLE test_drop (id bigint)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: createStmt, UserID: adminUI.UserID})
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	dropStmt := parseStmt(t, "DROP TABLE test_drop")
	result, err := eng.Execute(ctx, &engine.Request{Stmt: dropStmt, UserID: adminUI.UserID})
	if err != nil {
		t.Fatalf("DROP TABLE: %v", err)
	}
	if !strings.Contains(result.Message, "DROP TABLE") {
		t.Errorf("message should contain DROP TABLE, got %q", result.Message)
	}
}

func TestDDL_DropTable_NotFound(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	stmt := parseStmt(t, "DROP TABLE nonexistent")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err == nil {
		t.Fatal("expected error for dropping nonexistent table")
	}
}

func TestDDL_CreateTable_EmptySchema(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	// Vitess rejects zero-column tables at parse time.
	err := parseStmtErr(t, "CREATE TABLE empty_t ()")
	if err == nil {
		t.Fatal("expected parse error for zero-column table")
	}
	if !strings.Contains(err.Error(), "syntax") {
		t.Errorf("got %v, want syntax error", err)
	}
	_ = eng
	_ = adminUI
	_ = ctx
}

func TestDDL_CreateTable_Duplicate(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	stmt := parseStmt(t, "CREATE TABLE dup (id bigint)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err != nil {
		t.Fatalf("first CREATE: %v", err)
	}
	_, err = eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err == nil {
		t.Fatal("expected error for duplicate table")
	}
}

func TestDDL_Select_PassesThrough(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	createStmt := parseStmt(t, "CREATE TABLE sel_test (id bigint, name varchar(100))")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: createStmt, UserID: adminUI.UserID})
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	selectStmt := parseStmt(t, "SELECT id, name FROM sel_test")
	result, err := eng.Execute(ctx, &engine.Request{Stmt: selectStmt, UserID: adminUI.UserID})
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if result.Stream == nil {
		t.Fatal("SELECT should return a stream")
	}
	defer result.Stream.Close()
	schema := result.Stream.Schema()
	if len(schema.Columns) != 2 {
		t.Errorf("want 2 columns, got %d", len(schema.Columns))
	}
}

func TestDDL_UnsupportedStatement(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	stmt := parseStmt(t, "INSERT INTO t VALUES (1)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err == nil {
		t.Fatal("expected error for unsupported statement")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("got %v, want unsupported error", err)
	}
}

func TestDDL_UnsupportedColumnType(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	// DECIMAL is not in the basic tier type map.
	stmt := parseStmt(t, "CREATE TABLE bad_col (id bigint, price decimal(10,2))")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err == nil {
		t.Fatal("expected error for unsupported column type")
	}
	if !strings.Contains(err.Error(), "unsupported column type") {
		t.Errorf("got %v, want unsupported column type error", err)
	}
}

func TestDDL_DuplicateColumn(t *testing.T) {
	eng, _, adminUI, cleanup := newTestSQLEngine(t)
	defer cleanup()

	ctx := context.Background()
	stmt := parseStmt(t, "CREATE TABLE dup_col (id bigint, id int)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: adminUI.UserID})
	if err == nil {
		t.Fatal("expected error for duplicate column")
	}
	if !strings.Contains(err.Error(), "duplicate column") {
		t.Errorf("got %v, want duplicate column error", err)
	}
}

func TestDDL_PermissionDenied(t *testing.T) {
	dir := t.TempDir()
	pg := pager.New(dir + "/perm_test.pager")
	ctx := context.Background()
	if err := pg.Open(ctx); err != nil {
		t.Fatalf("pager.Open: %v", err)
	}
	store := kv.New(pg)
	if err := store.Open(ctx); err != nil {
		t.Fatalf("kv.Open: %v", err)
	}
	defer func() { store.Close(ctx); pg.Close(ctx) }()

	h := heap.New(store)
	cat := catalog.New(h)
	cat.RegisterEngine(testEngine{"sql"})
	if err := cat.Start(ctx); err != nil {
		t.Fatalf("cat.Start: %v", err)
	}
	// Create a non-admin user.
	if err := cat.CreateUser(ctx, "user", "pass", false); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Authenticate to get user ID 1 (admin is 0).
	ui, err := cat.Authenticate(ctx, "user", "pass")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	tm := NewHeapManager(store)
	txm := tx.NewManager(1, 1)
	dec := NewRowDecoder()
	pc := planner.NewPlanCache(16)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	eng, err := NewSQLEngine(cat, cat, tm, dec, pc, txm, log)
	if err != nil {
		t.Fatalf("NewSQLEngine: %v", err)
	}

	stmt := parseStmt(t, "CREATE TABLE secret (id bigint)")
	// UserID = 1 (non-admin, no DDL grant).
	_, err = eng.Execute(ctx, &engine.Request{Stmt: stmt, UserID: ui.UserID})
	if err == nil {
		t.Fatal("expected permission denied for non-admin DDL")
	}
}

// Catalog type alias for test setup.
type Catalog = catalog.Catalog
