package router

import (
	"context"
	"testing"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

// mockEngine implements engine.Engine for testing.
type mockEngine struct {
	name    string
	execute func(ctx context.Context, req *engine.Request) (*engine.Result, error)
}

func (m *mockEngine) Name() string { return m.name }
func (m *mockEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	return m.execute(ctx, req)
}

type mockCatalog struct {
	tables  map[string]catalog.TableInfo
	permOk  bool
	permErr error
}

func (m *mockCatalog) Start(ctx context.Context) error       { return nil }
func (m *mockCatalog) Stop(ctx context.Context) error        { return nil }
func (m *mockCatalog) Name() string                          { return "mock" }
func (m *mockCatalog) RegisterEngine(e catalog.Engine) error { return nil }
func (m *mockCatalog) CreateTable(ctx context.Context, eng, name string, schema []byte) error {
	return nil
}
func (m *mockCatalog) DropTable(ctx context.Context, name string) error { return nil }
func (m *mockCatalog) GetTable(ctx context.Context, name string) (catalog.TableInfo, error) {
	ti, ok := m.tables[name]
	if !ok {
		return catalog.TableInfo{}, catalog.ErrTableNotFound
	}
	return ti, nil
}
func (m *mockCatalog) CreateUser(ctx context.Context, u, p string, a bool) error { return nil }
func (m *mockCatalog) Authenticate(ctx context.Context, u, p string) (catalog.UserInfo, error) {
	return catalog.UserInfo{}, nil
}
func (m *mockCatalog) AllocateTableID(ctx context.Context) (uint64, error) { return 100, nil }
func (m *mockCatalog) RegisterTable(ctx context.Context, tableID uint64, eng, name string, payload []byte) error {
	return nil
}
func (m *mockCatalog) CheckGlobalPermission(ctx context.Context, uid uint64, a catalog.Action) (bool, error) {
	return m.permOk, m.permErr
}
func (m *mockCatalog) CreateRole(ctx context.Context, name string) error               { return nil }
func (m *mockCatalog) DropRole(ctx context.Context, name string) error                 { return nil }
func (m *mockCatalog) AssignRole(ctx context.Context, u, r string) error               { return nil }
func (m *mockCatalog) RevokeRole(ctx context.Context, u, r string) error               { return nil }
func (m *mockCatalog) Grant(ctx context.Context, r, t string, a catalog.Action) error  { return nil }
func (m *mockCatalog) Revoke(ctx context.Context, r, t string, a catalog.Action) error { return nil }
func (m *mockCatalog) CheckPermission(ctx context.Context, uid, tid uint64, a catalog.Action) (bool, error) {
	if m.permErr != nil {
		return false, m.permErr
	}
	return m.permOk, nil
}
func (m *mockCatalog) GetSchemaHistory(ctx context.Context, name string) ([]catalog.SchemaHistoryEntry, error) {
	return nil, nil
}
func (m *mockCatalog) SchemaVersion() uint64 { return 1 }

func TestRoute_Select(t *testing.T) {
	cat := &mockCatalog{
		tables: map[string]catalog.TableInfo{"users": {TableID: 1, EngineName: "sql"}},
		permOk: true,
	}
	r := New(cat)
	called := false
	r.RegisterEngine(&mockEngine{name: "sql", execute: func(ctx context.Context, req *engine.Request) (*engine.Result, error) {
		called = true
		return &engine.Result{}, nil
	}})

	ctx := context.Background()
	p, _ := sqlparser.New()
	stmt, _ := p.Parse("SELECT * FROM users")
	_, err := r.Route(ctx, 1, stmt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if !called {
		t.Error("engine not called")
	}
}

func TestRoute_Unsupported(t *testing.T) {
	r := New(&mockCatalog{tables: map[string]catalog.TableInfo{}, permOk: true})
	p, _ := sqlparser.New()
	// ALTER TABLE is still unsupported.
	stmt, _ := p.Parse("ALTER TABLE users ADD COLUMN x int")
	_, err := r.Route(context.Background(), 1, stmt)
	if err != ErrUnsupportedStatement {
		t.Errorf("got %v, want ErrUnsupportedStatement", err)
	}
}

func TestRoute_NoTargetTable(t *testing.T) {
	r := New(&mockCatalog{tables: map[string]catalog.TableInfo{}, permOk: true})
	p, _ := sqlparser.New()
	stmt, _ := p.Parse("SELECT 1")
	_, err := r.Route(context.Background(), 1, stmt)
	if err != ErrNoTargetTable {
		t.Errorf("got %v, want ErrNoTargetTable", err)
	}
}

func TestRoute_PermissionDenied(t *testing.T) {
	cat := &mockCatalog{
		tables: map[string]catalog.TableInfo{"users": {TableID: 1, EngineName: "sql"}},
		permOk: false,
	}
	r := New(cat)
	p, _ := sqlparser.New()
	stmt, _ := p.Parse("SELECT * FROM users")
	_, err := r.Route(context.Background(), 1, stmt)
	if err != ErrPermissionDenied {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

func TestRoute_CrossEngine(t *testing.T) {
	cat := &mockCatalog{
		tables: map[string]catalog.TableInfo{
			"users":  {TableID: 1, EngineName: "sql"},
			"orders": {TableID: 2, EngineName: "mongo"},
		},
		permOk: true,
	}
	r := New(cat)
	p, _ := sqlparser.New()
	stmt, _ := p.Parse("SELECT * FROM users JOIN orders")
	_, err := r.Route(context.Background(), 1, stmt)
	if err != ErrCrossEngineJoinNotSupported {
		t.Errorf("got %v, want ErrCrossEngineJoinNotSupported", err)
	}
}
