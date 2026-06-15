package logger_test

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
)

// These values are part of the stable logger API.
func TestFieldConstants(t *testing.T) {
	if logger.FieldComponent != "component" {
		t.Errorf("FieldComponent = %q, want %q", logger.FieldComponent, "component")
	}
	if logger.FieldError != "error" {
		t.Errorf("FieldError = %q, want %q", logger.FieldError, "error")
	}
	if logger.FieldPath != "path" {
		t.Errorf("FieldPath = %q, want %q", logger.FieldPath, "path")
	}
	if logger.FieldDuration != "duration_ms" {
		t.Errorf("FieldDuration = %q, want %q", logger.FieldDuration, "duration_ms")
	}
}

func TestNewValidCombinations(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
		output string
	}{
		{name: "debug/text/stdout", level: "debug", format: "text", output: "stdout"},
		{name: "info/json/stdout", level: "info", format: "json", output: "stdout"},
		{name: "warn/text/stderr", level: "warn", format: "text", output: "stderr"},
		{name: "error/json/stderr", level: "error", format: "json", output: "stderr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.LoggerConfig{
				Level:  tt.level,
				Format: tt.format,
				Output: tt.output,
			}
			l, err := logger.New(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if l == nil {
				t.Error("expected non-nil logger")
			}
		})
	}
}

func TestNewInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		format  string
		output  string
		wantErr string
	}{
		{
			name:    "invalid level",
			level:   "trace",
			format:  "text",
			output:  "stdout",
			wantErr: "logger.level must be one of: debug, info, warn, error",
		},
		{
			name:    "invalid format",
			level:   "info",
			format:  "console",
			output:  "stdout",
			wantErr: "logger.format must be one of: text, json",
		},
		{
			name:    "invalid output",
			level:   "info",
			format:  "text",
			output:  "file",
			wantErr: "logger.output must be one of: stdout, stderr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.LoggerConfig{
				Level:  tt.level,
				Format: tt.format,
				Output: tt.output,
			}
			_, err := logger.New(cfg)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestErrorAttrNonNil(t *testing.T) {
	attr := logger.ErrorAttr(errors.New("boom"))
	if attr.Key != "error" {
		t.Errorf("attr.Key = %q, want %q", attr.Key, "error")
	}
	if attr.Value.String() != "boom" {
		t.Errorf("attr.Value.String() = %q, want %q", attr.Value.String(), "boom")
	}
}

func TestErrorAttrNil(t *testing.T) {
	attr := logger.ErrorAttr(nil)
	if attr.Key != "error" {
		t.Errorf("attr.Key = %q, want %q", attr.Key, "error")
	}
	if attr.Value.String() != "" {
		t.Errorf("attr.Value.String() = %q, want %q", attr.Value.String(), "")
	}
}

func TestRedactAttr(t *testing.T) {
	redacted := logger.RedactedValue

	tests := []struct {
		name      string
		key       string
		value     string
		wantValue string
	}{
		// Sensitive keys — should be redacted.
		{name: "password", key: "password", value: "abc", wantValue: redacted},
		{name: "passwd", key: "passwd", value: "abc", wantValue: redacted},
		{name: "secret", key: "secret", value: "abc", wantValue: redacted},
		{name: "token", key: "token", value: "abc", wantValue: redacted},
		{name: "api_key", key: "api_key", value: "abc", wantValue: redacted},
		{name: "apikey", key: "apikey", value: "abc", wantValue: redacted},
		{name: "authorization", key: "authorization", value: "abc", wantValue: redacted},
		{name: "cookie", key: "cookie", value: "abc", wantValue: redacted},
		{name: "set-cookie", key: "set-cookie", value: "abc", wantValue: redacted},
		// Case-insensitive variants.
		{name: "Password", key: "Password", value: "abc", wantValue: redacted},
		{name: "AUTHORIZATION", key: "AUTHORIZATION", value: "abc", wantValue: redacted},
		{name: "Set-Cookie", key: "Set-Cookie", value: "abc", wantValue: redacted},
		// Non-sensitive keys — should not be redacted.
		{name: "path", key: "path", value: "/tmp", wantValue: "/tmp"},
		{name: "component", key: "component", value: "config", wantValue: "config"},
		{name: "duration_ms", key: "duration_ms", value: "42", wantValue: "42"},
		{name: "message", key: "message", value: "hello", wantValue: "hello"},
		// Empty key should not redact.
		{name: "empty key", key: "", value: "val", wantValue: "val"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := logger.RedactAttr(slog.String(tt.key, tt.value))
			if attr.Value.String() != tt.wantValue {
				t.Errorf("value = %q, want %q", attr.Value.String(), tt.wantValue)
			}
		})
	}
}

func TestLoggingDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../docs/logging.md")
	if err != nil {
		t.Fatalf("docs/logging.md not found: %v", err)
	}
	content := string(data)

	required := []string{
		"structured logging",
		"component-scoped loggers",
		"ErrorAttr",
		"RedactAttr",
		"stdout",
		"stderr",
		"text",
		"json",
		"unsupported outputs",
		"stdout is an output",
		"text and json are formats",
	}
	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("docs/logging.md missing required string: %q", s)
		}
	}
}
