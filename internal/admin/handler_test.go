package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/storage/cold"
	hotstore "github.com/plomvix/plomvix/internal/storage/hot"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()

	walDir := filepath.Join(dir, "wal")
	walCfg := &config.Config{Storage: config.StorageConfig{
		DataDir: dir, WALFlushThreshold: 64 * 1024 * 1024,
	}}
	wal, err := walstore.Open(walDir, walCfg)
	if err != nil {
		t.Fatalf("wal.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = wal.Close() })

	hotDir := filepath.Join(dir, "hot")
	hotCfg := &config.Config{Storage: config.StorageConfig{DataDir: hotDir}}
	hot, err := hotstore.Open(hotDir, hotCfg)
	if err != nil {
		t.Fatalf("hot.Open failed: %v", err)
	}
	t.Cleanup(func() { hot.Close() })

	coldDir := filepath.Join(dir, "cold")
	cs, err := cold.NewStore(coldDir)
	if err != nil {
		t.Fatalf("cold.NewStore failed: %v", err)
	}

	return NewHandler(wal, hot, cs, "test", "2024-01-01", "abc1234", time.Now())
}

func getJSON(t *testing.T, handler http.HandlerFunc, path string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	return resp
}

func TestStats(t *testing.T) {
	h := newTestHandler(t)
	resp := getJSON(t, h.Stats, "/admin/stats")
	data := resp["data"].(map[string]interface{})
	if _, ok := data["wal"]; !ok {
		t.Error("stats response missing 'wal' key")
	}
	if _, ok := data["hot"]; !ok {
		t.Error("stats response missing 'hot' key")
	}
	if _, ok := data["cold"]; !ok {
		t.Error("stats response missing 'cold' key")
	}
	if _, ok := data["runtime"]; !ok {
		t.Error("stats response missing 'runtime' key")
	}
}

func TestInfo(t *testing.T) {
	h := newTestHandler(t)
	resp := getJSON(t, h.Info, "/admin/info")
	data := resp["data"].(map[string]interface{})
	if data["version"] != "test" {
		t.Errorf("version = %v, want test", data["version"])
	}
	if data["git_commit"] != "abc1234" {
		t.Errorf("git_commit = %v, want abc1234", data["git_commit"])
	}
	if _, ok := data["uptime_seconds"]; !ok {
		t.Error("info response missing 'uptime_seconds'")
	}
}

func TestWALStats(t *testing.T) {
	h := newTestHandler(t)
	resp := getJSON(t, h.WALStats, "/admin/wal/stats")
	data := resp["data"].(map[string]interface{})
	if _, ok := data["segment_count"]; !ok {
		t.Error("wal stats missing 'segment_count'")
	}
}

func TestWALRotate(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/wal/rotate", nil)
	w := httptest.NewRecorder()
	h.WALRotate(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("WALRotate status = %d, want 200", w.Code)
	}
}

func TestColdStats(t *testing.T) {
	h := newTestHandler(t)
	resp := getJSON(t, h.ColdStats, "/admin/cold/stats")
	data := resp["data"].(map[string]interface{})
	if _, ok := data["parquet_files"]; !ok {
		t.Error("cold stats missing 'parquet_files'")
	}
}

func TestSchemaList(t *testing.T) {
	h := newTestHandler(t)
	resp := getJSON(t, h.SchemaList, "/admin/schema")
	data := resp["data"].(map[string]interface{})
	// All four types should be present even if empty
	for _, dt := range []string{"logs", "metrics", "json", "kv"} {
		if _, ok := data[dt]; !ok {
			t.Errorf("schema list missing data type %q", dt)
		}
	}
}

func TestSchemaDelete(t *testing.T) {
	h := newTestHandler(t)

	// Use chi router to handle URL param
	r := chi.NewRouter()
	r.Delete("/admin/schema/{type}", h.SchemaDelete)

	req := httptest.NewRequest(http.MethodDelete, "/admin/schema/logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("SchemaDelete status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestSchemaDeleteUnknownType(t *testing.T) {
	h := newTestHandler(t)

	r := chi.NewRouter()
	r.Delete("/admin/schema/{type}", h.SchemaDelete)

	req := httptest.NewRequest(http.MethodDelete, "/admin/schema/unknown", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("SchemaDelete unknown type status = %d, want 400", w.Code)
	}
}
