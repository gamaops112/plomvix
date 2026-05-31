package ingestion

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
	hotstore "github.com/plomvix/plomvix/internal/storage/hot"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

func newTestHot(t *testing.T) *hotstore.Manager {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hot")
	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: dir},
	}
	m, err := hotstore.Open(dir, cfg)
	if err != nil {
		t.Fatalf("hot.Open failed: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func newTestWAL(t *testing.T) *walstore.Manager {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "wal")
	cfg := &config.Config{
		Storage: config.StorageConfig{
			DataDir:           dir,
			WALFlushThreshold: 64 * 1024 * 1024,
		},
	}
	m, err := walstore.Open(dir, cfg)
	if err != nil {
		t.Fatalf("wal.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(newTestHot(t), newTestWAL(t))
}

func postJSON(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestIngestLogsSuccess(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"level": "info", "message": "hello", "timestamp": 0},
		},
	}
	w := postJSON(t, h.IngestLogs, body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["ingested"].(float64) != 1 {
		t.Errorf("ingested = %v, want 1", data["ingested"])
	}
}

func TestIngestLogsEmptyRecords(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{"records": []interface{}{}}
	w := postJSON(t, h.IngestLogs, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestMetricsMissingName(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"value": 42.0}, // name is missing
		},
	}
	w := postJSON(t, h.IngestMetrics, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestMetricsSuccess(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"name": "cpu.usage", "value": 87.5},
		},
	}
	w := postJSON(t, h.IngestMetrics, body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestIngestJSONMissingData(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"timestamp": 0}, // data field is nil/missing
		},
	}
	w := postJSON(t, h.IngestJSON, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestJSONSuccess(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"data": map[string]interface{}{"event": "order_placed", "amount": 99.99}},
		},
	}
	w := postJSON(t, h.IngestJSON, body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestIngestKVMissingKey(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"value": "somevalue"}, // key is missing
		},
	}
	w := postJSON(t, h.IngestKV, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestKVSuccess(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"key": "user:alice", "value": "active"},
		},
	}
	w := postJSON(t, h.IngestKV, body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestIngestBatchMultipleRecords(t *testing.T) {
	h := newTestHandler(t)
	body := map[string]interface{}{
		"records": []map[string]interface{}{
			{"level": "info", "message": "first"},
			{"level": "warn", "message": "second"},
			{"level": "error", "message": "third"},
		},
	}
	w := postJSON(t, h.IngestLogs, body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["ingested"].(float64) != 3 {
		t.Errorf("ingested = %v, want 3", data["ingested"])
	}
}
