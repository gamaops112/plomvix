package theme

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(dir + "/theme.json")
}

func TestGetThemeReturnsDefault(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/theme", nil)
	rec := httptest.NewRecorder()
	handler.GetTheme(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var env struct {
		Data Theme `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if env.Data.Version != 1 {
		t.Fatalf("expected version 1, got %d", env.Data.Version)
	}
}

func TestPutThemeSavesValidTheme(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)

	tm := DefaultTheme()
	tm.Mode = "dark"
	body, _ := json.Marshal(tm)

	req := httptest.NewRequest(http.MethodPut, "/api/theme", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.UpdateTheme(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var env struct {
		Data Theme `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.Mode != "dark" {
		t.Fatalf("expected mode dark, got %q", env.Data.Mode)
	}
}

func TestPutThemeRejectsInvalidTheme(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)

	tm := DefaultTheme()
	tm.Version = 99
	body, _ := json.Marshal(tm)

	req := httptest.NewRequest(http.MethodPut, "/api/theme", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.UpdateTheme(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestResetThemeRestoresDefaults(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)

	// First change the theme
	tm := DefaultTheme()
	tm.Mode = "dark"
	body, _ := json.Marshal(tm)
	req := httptest.NewRequest(http.MethodPut, "/api/theme", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.UpdateTheme(rec, req)

	// Now reset
	req2 := httptest.NewRequest(http.MethodPost, "/api/theme/reset", nil)
	rec2 := httptest.NewRecorder()
	handler.ResetTheme(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	var env struct {
		Data Theme `json:"data"`
	}
	json.Unmarshal(rec2.Body.Bytes(), &env)
	if env.Data.Mode != "light" {
		t.Fatalf("expected mode light after reset, got %q", env.Data.Mode)
	}
}

func TestExportThemeReturnsJSONWithDisposition(t *testing.T) {
	store := newTestStore(t)
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/theme/export", nil)
	rec := httptest.NewRecorder()
	handler.ExportTheme(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "plomvix-theme.json") {
		t.Fatalf("expected content-disposition with plomvix-theme.json, got: %q", disp)
	}

	var tm Theme
	if err := json.Unmarshal(rec.Body.Bytes(), &tm); err != nil {
		t.Fatalf("expected valid JSON, got: %v", err)
	}
	if tm.Version != 1 {
		t.Fatalf("expected version 1, got %d", tm.Version)
	}
}
