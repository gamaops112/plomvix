package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

// mockRouter implements Router for testing.
type mockRouter struct {
	route func(ctx context.Context, userID uint64, stmt sqlparser.Statement) (*engine.Result, error)
}

func (m *mockRouter) Route(ctx context.Context, userID uint64, stmt sqlparser.Statement) (*engine.Result, error) {
	return m.route(ctx, userID, stmt)
}

func TestServer_StartStop(t *testing.T) {
	p, err := sqlparser.New()
	if err != nil {
		t.Fatal(err)
	}
	router := &mockRouter{
		route: func(_ context.Context, _ uint64, _ sqlparser.Statement) (*engine.Result, error) {
			return &engine.Result{}, nil
		},
	}
	srv := New(ServerConfig{Addr: "127.0.0.1:0", Router: router, Parser: p})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if srv.Name() != "pg-wire-server" {
		t.Errorf("Name: got %q, want \"pg-wire-server\"", srv.Name())
	}
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestServer_ClientConnect(t *testing.T) {
	p, err := sqlparser.New()
	if err != nil {
		t.Fatal(err)
	}
	router := &mockRouter{
		route: func(_ context.Context, _ uint64, stmt sqlparser.Statement) (*engine.Result, error) {
			return &engine.Result{
				Stream: &fakeRowStream{},
			}, nil
		},
	}

	srv := New(ServerConfig{Addr: "127.0.0.1:0", Router: router, Parser: p})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(ctx)

	addr := srv.Addr()
	if addr == nil {
		t.Fatal("server address is nil")
	}

	// Connect a raw client.
	conn, err := net.DialTimeout("tcp", addr.String(), 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send startup message: 4-byte length (big-endian) + 4-byte protocol version 196608
	// 196608 = 0x00030000 = 3 << 16
	startup := make([]byte, 8)
	// Length = 8 (big-endian): 00 00 00 08
	startup[0] = 0
	startup[1] = 0
	startup[2] = 0
	startup[3] = 8
	// Version = 196608 (big-endian): 00 03 00 00
	startup[4] = 0
	startup[5] = 3
	startup[6] = 0
	startup[7] = 0
	conn.Write(startup)

	// Read response.
	resp := make([]byte, 1024)
	n, _ := conn.Read(resp)
	if n < 9 || resp[0] != 'R' {
		t.Errorf("expected AuthOK ('R'), got %v", resp[:n])
	}
}

// fakeRowStream implements engine.RowStream for testing.
type fakeRowStream struct{}

func (s *fakeRowStream) Next(_ context.Context) (engine.Row, error) {
	return engine.Row{Datums: []engine.Datum{
		{Type: engine.TypeInt64, Value: int64(42)},
	}}, nil
}
func (s *fakeRowStream) Schema() engine.Schema {
	return engine.Schema{Columns: []engine.Column{
		{Name: "x", Type: engine.TypeInt64},
	}}
}
func (s *fakeRowStream) Close() error { return nil }
