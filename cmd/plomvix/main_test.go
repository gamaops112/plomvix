package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/runtime"
)

func TestRunValidConfigSucceeds(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "info"
format = "text"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := run(context.Background(), runtime.Options{ConfigPath: p}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMissingConfigReturnsError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.toml")
	err := run(context.Background(), runtime.Options{ConfigPath: missingPath})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}
