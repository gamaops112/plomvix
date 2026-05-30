package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirExists(t *testing.T) {
	if !DirExists(os.TempDir()) {
		t.Error("expected DirExists to return true for TempDir")
	}
	if DirExists("/nonexistent/path/abc123xyz") {
		t.Error("expected DirExists to return false for nonexistent path")
	}

	tmpFile, err := os.CreateTemp("", "plomvix_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()
	if DirExists(tmpFile.Name()) {
		t.Error("expected DirExists to return false for a file")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "plomvix_test_ensure")
	defer os.RemoveAll(dir)
	if err := EnsureDir(dir); err != nil {
		t.Errorf("EnsureDir failed: %v", err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Errorf("EnsureDir should be idempotent: %v", err)
	}
}

func TestIsWritable(t *testing.T) {
	if !IsWritable(os.TempDir()) {
		t.Error("expected IsWritable to return true for TempDir")
	}

	checkFile := filepath.Join(os.TempDir(), ".plomvix_write_check")
	if _, err := os.Stat(checkFile); err == nil {
		t.Error("IsWritable should have cleaned up temp file")
	}

	if os.Getuid() == 0 {
		t.Skip("skipping readonly dir test: chmod restrictions do not apply to root")
	}
	readonlyDir := filepath.Join(os.TempDir(), "plomvix_test_readonly")
	if err := os.MkdirAll(readonlyDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(readonlyDir, 0755)
		_ = os.RemoveAll(readonlyDir)
	}()
	if err := os.Chmod(readonlyDir, 0000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	if IsWritable(readonlyDir) {
		t.Error("expected IsWritable to return false for readonly dir")
	}
}

func TestBytesToHuman(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{67108864, "64.0 MB"},
		{10737418240, "10.0 GB"},
	}
	for _, tt := range tests {
		got := BytesToHuman(tt.input)
		if got != tt.expected {
			t.Errorf("BytesToHuman(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetGoVersion(t *testing.T) {
	v := GetGoVersion()
	if !strings.HasPrefix(v, "go") {
		t.Errorf("GetGoVersion = %q, expected to start with 'go'", v)
	}
}

func TestGetOSArch(t *testing.T) {
	s := GetOSArch()
	if s == "" {
		t.Error("GetOSArch returned empty string")
	}
	if c := strings.Count(s, "/"); c != 1 {
		t.Errorf("GetOSArch = %q, expected exactly one '/'", s)
	}
}

func TestNewRequestID(t *testing.T) {
	id1 := NewRequestID()
	if len(id1) != 36 {
		t.Errorf("NewRequestID length = %d, want 36", len(id1))
	}
	if c := strings.Count(id1, "-"); c != 4 {
		t.Errorf("NewRequestID has %d hyphens, want 4", c)
	}
	parts := strings.Split(id1, "-")
	expectedLens := []int{8, 4, 4, 4, 12}
	for i, l := range expectedLens {
		if len(parts[i]) != l {
			t.Errorf("NewRequestID part %d length = %d, want %d", i, len(parts[i]), l)
		}
	}

	id2 := NewRequestID()
	if id1 == id2 {
		t.Error("two NewRequestID calls returned the same value")
	}
}
