package runtime_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/lifecycle"
	"github.com/plomvix/plomvix/internal/runtime"
)

func TestDefaultConfigPath(t *testing.T) {
	if runtime.DefaultConfigPath != "config.toml" {
		t.Errorf("DefaultConfigPath = %q, want %q", runtime.DefaultConfigPath, "config.toml")
	}
}

func TestDefaultOptionsConfigPath(t *testing.T) {
	opts := runtime.DefaultOptions()
	if opts.ConfigPath != runtime.DefaultConfigPath {
		t.Errorf("opts.ConfigPath = %q, want %q", opts.ConfigPath, runtime.DefaultConfigPath)
	}
}

// These values are part of the stable runtime API.
func TestEnterpriseErrors(t *testing.T) {
	if runtime.ErrInvalidOptions.Error() != "runtime: invalid options" {
		t.Errorf("ErrInvalidOptions = %q", runtime.ErrInvalidOptions.Error())
	}
	if runtime.ErrLoadConfig.Error() != "runtime: load config" {
		t.Errorf("ErrLoadConfig = %q", runtime.ErrLoadConfig.Error())
	}
	if runtime.ErrCreateLogger.Error() != "runtime: create logger" {
		t.Errorf("ErrCreateLogger = %q", runtime.ErrCreateLogger.Error())
	}
	if runtime.ErrStartLifecycle.Error() != "runtime: start lifecycle" {
		t.Errorf("ErrStartLifecycle = %q", runtime.ErrStartLifecycle.Error())
	}
	if runtime.ErrStopLifecycle.Error() != "runtime: stop lifecycle" {
		t.Errorf("ErrStopLifecycle = %q", runtime.ErrStopLifecycle.Error())
	}
	if runtime.ErrRuntimePanic.Error() != "runtime: panic" {
		t.Errorf("ErrRuntimePanic = %q", runtime.ErrRuntimePanic.Error())
	}
}

func TestDefaultStartupTimeout(t *testing.T) {
	if runtime.DefaultStartupTimeout != 30*time.Second {
		t.Errorf("DefaultStartupTimeout = %v, want 30s", runtime.DefaultStartupTimeout)
	}
}

func TestDefaultShutdownTimeout(t *testing.T) {
	if runtime.DefaultShutdownTimeout != 30*time.Second {
		t.Errorf("DefaultShutdownTimeout = %v, want 30s", runtime.DefaultShutdownTimeout)
	}
}

func TestDefaultOptionsTimeouts(t *testing.T) {
	opts := runtime.DefaultOptions()
	if opts.StartupTimeout != runtime.DefaultStartupTimeout {
		t.Errorf("StartupTimeout = %v, want %v", opts.StartupTimeout, runtime.DefaultStartupTimeout)
	}
	if opts.ShutdownTimeout != runtime.DefaultShutdownTimeout {
		t.Errorf("ShutdownTimeout = %v, want %v", opts.ShutdownTimeout, runtime.DefaultShutdownTimeout)
	}
}

func TestNegativeStartupTimeoutReturnsInvalidOptions(t *testing.T) {
	err := runtime.Run(context.Background(), runtime.Options{
		ConfigPath:     "nonexistent.toml", // Use valid-ish path to bypass config error
		StartupTimeout: -1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestNegativeShutdownTimeoutReturnsInvalidOptions(t *testing.T) {
	err := runtime.Run(context.Background(), runtime.Options{
		ConfigPath:      "nonexistent.toml",
		ShutdownTimeout: -1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestRunStubSucceeds(t *testing.T) {
	// Stub no longer applies — replaced by real config tests below.
}

func TestNewValidOptionsReturnsRuntime(t *testing.T) {
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
	rt, err := runtime.New(runtime.Options{ConfigPath: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
}

func TestNewMissingConfigReturnsLoadConfig(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.toml")
	_, err := runtime.New(runtime.Options{ConfigPath: missingPath})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestNewMalformedConfigReturnsLoadConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("{{{"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.New(runtime.Options{ConfigPath: p})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestNewInvalidLoggerConfigReturnsLoadConfig(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "trace"
format = "text"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.New(runtime.Options{ConfigPath: p})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestNewNegativeStartupTimeoutReturnsInvalidOptions(t *testing.T) {
	_, err := runtime.New(runtime.Options{
		ConfigPath:     "nonexistent.toml",
		StartupTimeout: -1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestNewNegativeShutdownTimeoutReturnsInvalidOptions(t *testing.T) {
	_, err := runtime.New(runtime.Options{
		ConfigPath:      "nonexistent.toml",
		ShutdownTimeout: -1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestRunValidConfig(t *testing.T) {
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

	if err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMissingConfigReturnsError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.toml")
	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: missingPath})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("error should contain 'load config': %v", err)
	}
}

func TestRunInvalidConfigReturnsError(t *testing.T) {
	content := `
[server]
port = 70000
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("error should contain 'load config': %v", err)
	}
}

func TestRunInvalidLoggerConfigReturnsError(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "trace"
format = "text"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p})
	if err == nil {
		t.Fatal("expected error for invalid logger config")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("error should contain 'load config': %v", err)
	}
}

func TestRunValidJSONLoggerConfig(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "debug"
format = "json"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunEmptyConfigPathUsesDefault(t *testing.T) {
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
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	if err := runtime.Run(context.Background(), runtime.Options{}); err != nil {
		t.Fatalf("unexpected error with empty Options: %v", err)
	}
}

func TestRuntimeDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../docs/runtime.md")
	if err != nil {
		t.Fatalf("docs/runtime.md not found: %v", err)
	}
	content := string(data)

	required := []string{
		"# Plomvix Runtime",
		"runtime setup",
		"core composition",
		"enterprise runtime hardening",
		"runtime object",
		"runtime options",
		"startup timeout",
		"shutdown timeout",
		"classified runtime errors",
		"panic recovery",
		"config loading",
		"logger creation",
		"lifecycle manager",
		"zero-component lifecycle",
		"config.toml",
		"no CLI flags",
		"no environment overrides",
		"signal handling",
		"WAL",
		"storage",
		"query engine",
		"API server",
		"UI",
	}
	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("docs/runtime.md missing required string: %q", s)
		}
	}
}

func TestRuntimeStateNewAfterConstruction(t *testing.T) {
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
	rt, err := runtime.New(runtime.Options{ConfigPath: p})
	if err != nil {
		t.Fatal(err)
	}
	if rt.State() != lifecycle.StateNew {
		t.Errorf("state = %q, want StateNew", rt.State())
	}
}

func TestRuntimeStartTransitionsToStarted(t *testing.T) {
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
	rt, err := runtime.New(runtime.Options{ConfigPath: p})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rt.State() != lifecycle.StateStarted {
		t.Errorf("state = %q, want StateStarted", rt.State())
	}
}

func TestRuntimeStopTransitionsToStopped(t *testing.T) {
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
	rt, err := runtime.New(runtime.Options{ConfigPath: p})
	if err != nil {
		t.Fatal(err)
	}
	rt.Start(context.Background())
	if err := rt.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rt.State() != lifecycle.StateStopped {
		t.Errorf("state = %q, want StateStopped", rt.State())
	}
}

func TestRuntimeRepeatedStopReturnsNil(t *testing.T) {
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
	rt, err := runtime.New(runtime.Options{ConfigPath: p})
	if err != nil {
		t.Fatal(err)
	}
	rt.Start(context.Background())
	rt.Stop(context.Background())
	if err := rt.Stop(context.Background()); err != nil {
		t.Errorf("repeated stop should return nil, got: %v", err)
	}
}

func TestNilRuntimeStateReturnsFailed(t *testing.T) {
	var rt *runtime.Runtime
	if rt.State() != lifecycle.StateFailed {
		t.Errorf("nil runtime state = %q, want StateFailed", rt.State())
	}
}

func TestNilRuntimeStartReturnsInvalidOptions(t *testing.T) {
	var rt *runtime.Runtime
	err := rt.Start(context.Background())
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestNilRuntimeStopReturnsInvalidOptions(t *testing.T) {
	var rt *runtime.Runtime
	err := rt.Stop(context.Background())
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestRunMissingConfigMatchesErrLoadConfig(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.toml")
	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: missingPath})
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestRunMalformedConfigMatchesErrLoadConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("{{{"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p})
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestRunInvalidConfigMatchesErrLoadConfig(t *testing.T) {
	content := `
[server]
port = 70000
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p})
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestRunInvalidLoggerConfigMatchesErrLoadConfig(t *testing.T) {
	content := `
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "trace"
format = "text"
output = "stderr"
`
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	err := runtime.Run(context.Background(), runtime.Options{ConfigPath: p})
	if !errors.Is(err, runtime.ErrLoadConfig) {
		t.Errorf("error = %v, want ErrLoadConfig", err)
	}
}

func TestRunNegativeStartupTimeoutMatchesErrInvalidOptions(t *testing.T) {
	err := runtime.Run(context.Background(), runtime.Options{
		ConfigPath:     "nonexistent.toml",
		StartupTimeout: -1,
	})
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestRunNegativeShutdownTimeoutMatchesErrInvalidOptions(t *testing.T) {
	err := runtime.Run(context.Background(), runtime.Options{
		ConfigPath:      "nonexistent.toml",
		ShutdownTimeout: -1,
	})
	if !errors.Is(err, runtime.ErrInvalidOptions) {
		t.Errorf("error = %v, want ErrInvalidOptions", err)
	}
}
