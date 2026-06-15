package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
)

func TestDefaultExists(t *testing.T) {
	cfg := config.Default()

	if cfg.Server.Host == "" {
		t.Error("default Server.Host should not be empty")
	}
	if cfg.Server.Port <= 0 {
		t.Errorf("default Server.Port should be > 0, got %d", cfg.Server.Port)
	}
	if cfg.Data.Path == "" {
		t.Error("default Data.Path should not be empty")
	}
}

func TestDefaultServerValues(t *testing.T) {
	cfg := config.Default()

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("default Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("default Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
}

func TestDefaultDataPath(t *testing.T) {
	cfg := config.Default()

	if cfg.Data.Path != "./data" {
		t.Errorf("default Data.Path = %q, want %q", cfg.Data.Path, "./data")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(cfg *config.Config)
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name:    "default config is valid",
			modify:  nil,
			wantErr: false,
		},
		{
			name: "empty server.host",
			modify: func(cfg *config.Config) {
				cfg.Server.Host = ""
			},
			wantErr:    true,
			wantErrMsg: "server.host is required",
		},
		{
			name: "zero server.port",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = 0
			},
			wantErr:    true,
			wantErrMsg: "server.port must be between 1 and 65535",
		},
		{
			name: "negative server.port",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = -1
			},
			wantErr:    true,
			wantErrMsg: "server.port must be between 1 and 65535",
		},
		{
			name: "server.port above 65535",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = 65536
			},
			wantErr:    true,
			wantErrMsg: "server.port must be between 1 and 65535",
		},
		{
			name: "empty data.path",
			modify: func(cfg *config.Config) {
				cfg.Data.Path = ""
			},
			wantErr:    true,
			wantErrMsg: "data.path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			if tt.modify != nil {
				tt.modify(&cfg)
			}

			err := config.Validate(cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestLoadValidConfig(t *testing.T) {
	content := `
[server]
host = "0.0.0.0"
port = 9090

[data]
path = "/tmp/plomvix"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Data.Path != "/tmp/plomvix" {
		t.Errorf("data path = %q, want %q", cfg.Data.Path, "/tmp/plomvix")
	}
}

func TestLoadPartialConfigPreservesDefaults(t *testing.T) {
	content := `
[server]
port = 9090
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("host = %q, want %q (default preserved)", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Data.Path != "data" {
		t.Errorf("data path = %q, want %q (default preserved)", cfg.Data.Path, "data")
	}
}

func TestLoadEmptyPathIsError(t *testing.T) {
	_, err := config.Load("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestLoadMissingFileIsError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nonexistent.toml")
	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadMalformedTOMLIsError(t *testing.T) {
	content := `this is not valid toml {{{`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for malformed TOML")
	}
}

func TestLoadInvalidConfigIsError(t *testing.T) {
	content := `
[server]
port = 70000
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for invalid config (port 70000)")
	}
}

func TestLoadNormalizesDataPath(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data/../data"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Data.Path != "data" {
		t.Errorf("data path = %q, want %q", cfg.Data.Path, "data")
	}
}

func TestLoadUnknownTopLevelFieldIsError(t *testing.T) {
	content := `
unknown = true

[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for unknown top-level field")
	}
}

func TestLoadUnknownServerFieldIsError(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
prt = 8080

[data]
path = "./data"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for unknown server field")
	}
}

func TestLoadUnknownDataFieldIsError(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
directory = "./other"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for unknown data field")
	}
}

func TestLoadExampleConfigIsValid(t *testing.T) {
	cfg, err := config.Load("../../config.example.toml")
	if err != nil {
		t.Fatalf("example config should be valid, got error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Data.Path != "data" {
		t.Errorf("data path = %q, want %q", cfg.Data.Path, "data")
	}
}
