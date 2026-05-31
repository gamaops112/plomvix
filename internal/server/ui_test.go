package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandlerServesIndexForAppRoute(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/explore", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Plomvix Test UI") {
		t.Fatalf("expected index.html content, got %q", rec.Body.String())
	}
}

func TestSPAHandlerServesAsset(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("expected JS content, got %q", rec.Body.String())
	}
}

func TestSPAHandlerReturnsNotFoundForMissingAsset(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/assets/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSPAHandlerReturns503WhenUINotBuilt(t *testing.T) {
	distDir := t.TempDir() // empty — no index.html
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Run make ui-build") {
		t.Fatalf("expected build instruction in body, got %q", rec.Body.String())
	}
}

func TestSPAHandlerRejectsPathTraversal(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Must not return 200 for a path traversal attempt
	if rec.Code == http.StatusOK {
		t.Fatal("path traversal should not return 200")
	}
}

// createTestUIDist creates a minimal fake ui/dist/ for testing.
func createTestUIDist(t *testing.T) string {
	t.Helper()
	distDir := t.TempDir()

	assetsDir := filepath.Join(distDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(distDir, "index.html"),
		[]byte("<!doctype html><html><body>Plomvix Test UI</body></html>"),
		0644,
	); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(assetsDir, "app.js"),
		[]byte("console.log('plomvix');"),
		0644,
	); err != nil {
		t.Fatalf("failed to write app.js: %v", err)
	}

	return distDir
}
