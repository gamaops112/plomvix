package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultThemeWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	store := NewStore(path)

	tm, err := store.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tm.Version != 1 {
		t.Fatalf("expected version 1, got: %d", tm.Version)
	}
	if tm.Mode != "light" {
		t.Fatalf("expected mode light, got: %q", tm.Mode)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected theme.json to be created, got: %v", err)
	}
}

func TestSaveAndLoadTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	store := NewStore(path)

	tm, err := store.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	tm.Mode = "dark"
	tm.DevPanel = false
	tm.Tokens.Colors.Primary = "#ff0000"

	if _, err := store.Save(tm); err != nil {
		t.Fatalf("expected no error on save, got: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if loaded.Mode != "dark" {
		t.Fatalf("expected mode dark, got: %q", loaded.Mode)
	}
	if loaded.DevPanel {
		t.Fatalf("expected dev_panel false, got true")
	}
	if loaded.Tokens.Colors.Primary != "#ff0000" {
		t.Fatalf("expected primary #ff0000, got: %q", loaded.Tokens.Colors.Primary)
	}
}

func TestResetRestoresDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	store := NewStore(path)

	tm, _ := store.Load()
	tm.Mode = "dark"
	tm.DevPanel = false
	_, _ = store.Save(tm)

	reset, err := store.Reset()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if reset.Mode != "light" {
		t.Fatalf("expected mode light after reset, got: %q", reset.Mode)
	}
	if !reset.DevPanel {
		t.Fatalf("expected dev_panel true after reset, got false")
	}
}

func TestExportJSONReturnsValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	store := NewStore(path)

	_, _ = store.Load()
	data, err := store.ExportJSON()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var tm Theme
	if err := json.Unmarshal(data, &tm); err != nil {
		t.Fatalf("expected valid JSON, got: %v", err)
	}
	if tm.Version != 1 {
		t.Fatalf("expected version 1, got: %d", tm.Version)
	}
}

func TestSaveRejectsInvalidTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	store := NewStore(path)

	_, _ = store.Load()
	tm := DefaultTheme()
	tm.Version = 99

	if _, err := store.Save(tm); err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
}
