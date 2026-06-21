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
	if cfg.Store.DBPath == "" {
		t.Error("default Store.DBPath should not be empty")
	}
}

func TestDefaultServerValues(t *testing.T) {
	cfg := config.Default()

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("default Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 5432 {
		t.Errorf("default Server.Port = %d, want %d", cfg.Server.Port, 5432)
	}
}

func TestDefaultDataPath(t *testing.T) {
	cfg := config.Default()

	if cfg.Store.DBPath != "data/plomvix.db" {
		t.Errorf("default Store.DBPath = %q, want %q", cfg.Store.DBPath, "data/plomvix.db")
	}
	if cfg.Store.MetricsDBPath != "data/metrics.db" {
		t.Errorf("default Store.MetricsDBPath = %q, want %q", cfg.Store.MetricsDBPath, "data/metrics.db")
	}
	if cfg.Store.RollupDBPath != "data/metrics_rollups.db" {
		t.Errorf("default Store.RollupDBPath = %q, want %q", cfg.Store.RollupDBPath, "data/metrics_rollups.db")
	}
}

func TestDefaultLoggerValues(t *testing.T) {
	cfg := config.Default()

	if cfg.Logger.Level != "info" {
		t.Errorf("default Logger.Level = %q, want %q", cfg.Logger.Level, "info")
	}
	if cfg.Logger.Format != "text" {
		t.Errorf("default Logger.Format = %q, want %q", cfg.Logger.Format, "text")
	}
	if cfg.Logger.Output != "stdout" {
		t.Errorf("default Logger.Output = %q, want %q", cfg.Logger.Output, "stdout")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		modify     func(cfg *config.Config)
		wantErr    bool
		wantErrMsg string
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
			name: "zero server.port is valid (OS-assigned)",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = 0
			},
			wantErr: false,
		},
		{
			name: "negative server.port",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = -1
			},
			wantErr:    true,
			wantErrMsg: "server.port must be between 0 and 65535",
		},
		{
			name: "server.port above 65535",
			modify: func(cfg *config.Config) {
				cfg.Server.Port = 65536
			},
			wantErr:    true,
			wantErrMsg: "server.port must be between 0 and 65535",
		},
		{
			name: "empty store db_path",
			modify: func(cfg *config.Config) {
				cfg.Store.DBPath = ""
			},
			wantErr:    true,
			wantErrMsg: "storage db_path is required",
		},
		{
			name: "valid logger config",
			modify: func(cfg *config.Config) {
				cfg.Logger.Level = "debug"
				cfg.Logger.Format = "json"
				cfg.Logger.Output = "stderr"
			},
			wantErr: false,
		},
		{
			name: "invalid logger level",
			modify: func(cfg *config.Config) {
				cfg.Logger.Level = "trace"
			},
			wantErr:    true,
			wantErrMsg: "logger.level must be one of: debug, info, warn, error",
		},
		{
			name: "invalid logger format",
			modify: func(cfg *config.Config) {
				cfg.Logger.Format = "console"
			},
			wantErr:    true,
			wantErrMsg: "logger.format must be one of: text, json",
		},
		{
			name: "invalid logger output",
			modify: func(cfg *config.Config) {
				cfg.Logger.Output = "file"
			},
			wantErr:    true,
			wantErrMsg: "logger.output must be one of: stdout, stderr",
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

[sql_engine]
data_dir = "/tmp/plomvix"

[storage]
db_path = "/tmp/plomvix.db"
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
	if cfg.SQL.DataDir != "/tmp/plomvix" {
		t.Errorf("sql data_dir = %q, want %q", cfg.SQL.DataDir, "/tmp/plomvix")
	}
	if cfg.Store.DBPath != "/tmp/plomvix.db" {
		t.Errorf("store db_path = %q, want %q", cfg.Store.DBPath, "/tmp/plomvix.db")
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
	if cfg.SQL.DataDir != "data/sql" {
		t.Errorf("sql data_dir = %q, want %q (default preserved)", cfg.SQL.DataDir, "data/sql")
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

[sql_engine]
data_dir = "./data/../data"

[storage]
db_path = "./data/../data/plomvix.db"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SQL.DataDir != "data" {
		t.Errorf("sql data_dir = %q, want %q", cfg.SQL.DataDir, "data")
	}
	if cfg.Store.DBPath != "data/plomvix.db" {
		t.Errorf("store db_path = %q, want %q", cfg.Store.DBPath, "data/plomvix.db")
	}
}

func TestLoadUnknownTopLevelFieldIsError(t *testing.T) {
	content := `
unknown = true

[server]
host = "127.0.0.1"
port = 8080

[sql_engine]
data_dir = "./data"

[storage]
db_path = "./data/plomvix.db"
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

[sql_engine]
data_dir = "./data"

[storage]
db_path = "./data/plomvix.db"
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

func TestLoadUnknownStorageFieldIsError(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[storage]
db_path = "./data/plomvix.db"
directories = "./other"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for unknown storage field")
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
	if cfg.Server.Port != 5432 {
		t.Errorf("port = %d, want %d", cfg.Server.Port, 5432)
	}
	if cfg.SQL.DataDir != "data/sql" {
		t.Errorf("sql data_dir = %q, want %q", cfg.SQL.DataDir, "data/sql")
	}
	if cfg.Store.DBPath != "data/plomvix.db" {
		t.Errorf("store db_path = %q, want %q", cfg.Store.DBPath, "data/plomvix.db")
	}
}

func TestLoadLoggerSection(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[sql_engine]
data_dir = "./data"

[storage]
db_path = "./data/plomvix.db"

[logger]
level = "debug"
format = "json"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Logger.Level != "debug" {
		t.Errorf("logger level = %q, want %q", cfg.Logger.Level, "debug")
	}
	if cfg.Logger.Format != "json" {
		t.Errorf("logger format = %q, want %q", cfg.Logger.Format, "json")
	}
	if cfg.Logger.Output != "stderr" {
		t.Errorf("logger output = %q, want %q", cfg.Logger.Output, "stderr")
	}
}

func TestLoadOmittedLoggerSectionPreservesDefaults(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[sql_engine]
data_dir = "./data"

[storage]
db_path = "./data/plomvix.db"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Logger.Level != "info" {
		t.Errorf("logger level = %q, want %q (default preserved)", cfg.Logger.Level, "info")
	}
	if cfg.Logger.Format != "text" {
		t.Errorf("logger format = %q, want %q (default preserved)", cfg.Logger.Format, "text")
	}
	if cfg.Logger.Output != "stdout" {
		t.Errorf("logger output = %q, want %q (default preserved)", cfg.Logger.Output, "stdout")
	}
}

func TestLoadUnknownLoggerFieldIsError(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[sql_engine]
data_dir = "./data"

[storage]
db_path = "./data/plomvix.db"

[logger]
color = true
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for unknown logger field")
	}
}
