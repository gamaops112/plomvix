package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
)

func TestNewWithWriterTextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggerConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
	l, err := newWithWriter(cfg, &buf)
	if err != nil {
		t.Fatal(err)
	}
	l.Info("hello")
	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("text output should contain msg=hello, got: %s", buf.String())
	}
}

func TestNewWithWriterJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	l, err := newWithWriter(cfg, &buf)
	if err != nil {
		t.Fatal(err)
	}
	l.Info("hello")

	line := strings.SplitN(buf.String(), "\n", 2)[0]
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("first line is not valid JSON: %v\nline: %s", err, line)
	}
	if m["msg"] != "hello" {
		t.Errorf("json msg = %v, want %q", m["msg"], "hello")
	}
}

func TestNewWithWriterDebugLevelPresent(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggerConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	}
	l, err := newWithWriter(cfg, &buf)
	if err != nil {
		t.Fatal(err)
	}
	l.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("debug message should be present, got: %s", buf.String())
	}
}

func TestNewWithWriterInfoLevelFiltersDebug(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggerConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
	l, err := newWithWriter(cfg, &buf)
	if err != nil {
		t.Fatal(err)
	}
	l.Debug("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Error("debug message should not appear at info level")
	}
}

func TestWithComponentAddsField(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggerConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
	base, err := newWithWriter(cfg, &buf)
	if err != nil {
		t.Fatal(err)
	}

	l := WithComponent(base, "config")
	l.Info("hello")
	if !strings.Contains(buf.String(), "component=config") {
		t.Errorf("output should contain component=config, got: %s", buf.String())
	}
}

func TestWithComponentNilBase(t *testing.T) {
	l := WithComponent(nil, "config")
	if l == nil {
		t.Fatal("expected non-nil logger from nil base")
	}
	// Logging should not panic.
	l.Info("should not panic")
}

func TestNewLevelControllerValid(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		c, err := NewLevelController(level)
		if err != nil {
			t.Errorf("NewLevelController(%q) unexpected error: %v", level, err)
		}
		if c == nil {
			t.Errorf("NewLevelController(%q) returned nil", level)
		}
	}
}

func TestNewLevelControllerInvalid(t *testing.T) {
	_, err := NewLevelController("trace")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	if err.Error() != "logger.level must be one of: debug, info, warn, error" {
		t.Errorf("error = %q, want exact error message", err.Error())
	}
}

func TestLevelControllerSetLevelEnablesDebug(t *testing.T) {
	var buf bytes.Buffer

	controller, err := NewLevelController("info")
	if err != nil {
		t.Fatal(err)
	}
	handler := slog.NewTextHandler(&buf, controller.HandlerOptions())
	log := slog.New(handler)

	log.Debug("should not appear")
	if buf.Len() > 0 {
		t.Error("debug should not appear at info level")
	}

	if err := controller.SetLevel("debug"); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	log.Debug("debug visible")
	if !strings.Contains(buf.String(), "debug visible") {
		t.Errorf("debug message should appear after SetLevel(\"debug\"), got: %s", buf.String())
	}
}

func TestLevelControllerSetLevelInvalid(t *testing.T) {
	controller, err := NewLevelController("info")
	if err != nil {
		t.Fatal(err)
	}
	err = controller.SetLevel("trace")
	if err == nil {
		t.Fatal("expected error for invalid SetLevel")
	}
	if err.Error() != "logger.level must be one of: debug, info, warn, error" {
		t.Errorf("error = %q, want exact error message", err.Error())
	}
}
