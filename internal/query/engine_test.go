package query

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/storage/cold"
	"github.com/plomvix/plomvix/internal/storage/hot"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hot")
	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: dir},
	}
	m, err := hot.Open(dir, cfg)
	if err != nil {
		t.Fatalf("hot.Open failed: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return NewEngine(m, nil)
}

func TestQueryLogsEmpty(t *testing.T) {
	e := newTestEngine(t)
	params := &QueryParams{ToNs: time.Now().UnixNano(), Limit: DefaultLimit}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
	if result.Records == nil {
		t.Error("Records should be non-nil empty slice, not nil")
	}
}

func TestQueryLogsWithData(t *testing.T) {
	e := newTestEngine(t)

	// Write 3 log records directly via hot tier
	base := time.Now().UnixNano()
	payloads := [][]byte{
		[]byte(`{"level":"info","message":"first"}`),
		[]byte(`{"level":"warn","message":"second"}`),
		[]byte(`{"level":"error","message":"third"}`),
	}
	for i, p := range payloads {
		if err := e.store.WriteLog(base+int64(i)*1000, p); err != nil {
			t.Fatalf("WriteLog %d failed: %v", i, err)
		}
	}

	params := &QueryParams{
		FromNs: base - 1,
		ToNs:   base + int64(len(payloads))*1000 + 1,
		Limit:  DefaultLimit,
	}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
}

func TestQueryLogsFilter(t *testing.T) {
	e := newTestEngine(t)

	base := time.Now().UnixNano()
	e.store.WriteLog(base+0, []byte(`{"level":"info","message":"a"}`))
	e.store.WriteLog(base+1000, []byte(`{"level":"warn","message":"b"}`))
	e.store.WriteLog(base+2000, []byte(`{"level":"info","message":"c"}`))

	conditions, _ := ParseFilter("level=info")
	params := &QueryParams{
		FromNs:  base - 1,
		ToNs:    base + 3000,
		Filters: conditions,
		Limit:   DefaultLimit,
	}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs with filter failed: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2 (only info records)", result.Total)
	}
}

func TestQueryLogsPagination(t *testing.T) {
	e := newTestEngine(t)

	base := time.Now().UnixNano()
	for i := 0; i < 5; i++ {
		e.store.WriteLog(base+int64(i)*1000, []byte(`{"level":"info","seq":1}`))
	}

	params := &QueryParams{
		FromNs: base - 1,
		ToNs:   base + 6000,
		Limit:  2,
		Offset: 1,
	}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs pagination failed: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if result.Count != 2 {
		t.Errorf("count = %d, want 2 (limit=2)", result.Count)
	}
}

func TestQueryKVFound(t *testing.T) {
	e := newTestEngine(t)
	e.store.WriteKV("mykey", []byte(`{"key":"mykey","value":"myval"}`))

	result, err := e.QueryKV("mykey")
	if err != nil {
		t.Fatalf("QueryKV failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestQueryKVNotFound(t *testing.T) {
	e := newTestEngine(t)
	result, err := e.QueryKV("doesnotexist")
	if err != nil {
		t.Fatalf("QueryKV failed: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if len(result.Records) != 0 {
		t.Errorf("records len = %d, want 0", len(result.Records))
	}
}

func newTestEngineWithCold(t *testing.T) (*Engine, *hot.Manager) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{Storage: config.StorageConfig{DataDir: dir}}

	hotDir := filepath.Join(dir, "hot")
	m, err := hot.Open(hotDir, cfg)
	if err != nil {
		t.Fatalf("hot.Open failed: %v", err)
	}
	t.Cleanup(func() { m.Close() })

	coldDir := filepath.Join(dir, "cold")
	cs, err := cold.NewStore(coldDir)
	if err != nil {
		t.Fatalf("cold.NewStore failed: %v", err)
	}

	return NewEngine(m, cs), m
}

func TestQueryLogsHotAndCold(t *testing.T) {
	e, hotMgr := newTestEngineWithCold(t)
	base := time.Now().UnixNano()

	// Write 1 record to hot tier directly
	hotMgr.WriteLog(base, []byte(`{"level":"info","message":"hot record","timestamp":1}`))

	// Write 1 record to cold tier directly via engine's cold store
	e.cold.WriteRows(cold.DataTypeLogs, []cold.ParquetRow{
		{TimestampNs: base - 1000, Payload: `{"level":"warn","message":"cold record","timestamp":0}`},
	}, time.Now())

	params := &QueryParams{
		FromNs: base - 2000,
		ToNs:   base + 1000,
		Limit:  DefaultLimit,
	}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2 (1 hot + 1 cold)", result.Total)
	}
}

func TestQueryLogsColdOnly(t *testing.T) {
	e, _ := newTestEngineWithCold(t)
	base := time.Now().UnixNano()

	// Only cold tier has data
	e.cold.WriteRows(cold.DataTypeLogs, []cold.ParquetRow{
		{TimestampNs: base, Payload: `{"level":"info","message":"cold only","timestamp":1}`},
	}, time.Now())

	params := &QueryParams{FromNs: base - 1, ToNs: base + 1, Limit: DefaultLimit}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestQueryLogsNilCold(t *testing.T) {
	// Engine with nil cold store should not panic and return only hot results
	e := newTestEngine(t) // uses nil cold
	params := &QueryParams{ToNs: time.Now().UnixNano(), Limit: DefaultLimit}
	result, err := e.QueryLogs(params)
	if err != nil {
		t.Fatalf("QueryLogs with nil cold failed: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}
