package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/runtime"
)

func TestRunMissingConfigReturnsError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.toml")
	err := run(runtime.Options{ConfigPath: missingPath})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestRunInvalidConfigReturnsError(t *testing.T) {
	content := `[server]
port = 70000
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	err := run(runtime.Options{ConfigPath: p})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}
