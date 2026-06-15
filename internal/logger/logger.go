// Package logger provides a production-grade logging foundation for Plomvix
// built on Go's standard library log/slog.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/plomvix/plomvix/internal/config"
)

// Standard structured logging field names.
const (
	FieldComponent = "component"
	FieldError     = "error"
	FieldPath      = "path"
	FieldDuration  = "duration_ms"
)

// RedactedValue is the replacement string for sensitive log values.
const RedactedValue = "[REDACTED]"

// sensitiveKeys holds attribute keys whose values must be redacted.
// Matching is case-insensitive. This map is initialized once and must
// never be modified after package init.
var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"secret":        {},
	"token":         {},
	"api_key":       {},
	"apikey":        {},
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
}

// New returns a configured *slog.Logger based on cfg. It does not set the
// global default logger and does not emit any logs during construction.
func New(cfg config.LoggerConfig) (*slog.Logger, error) {
	var w io.Writer
	switch cfg.Output {
	case "stdout":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	default:
		return nil, fmt.Errorf("logger.output must be one of: stdout, stderr")
	}
	return newWithWriter(cfg, w)
}

// newWithWriter builds a slog.Logger writing to w according to cfg.
func newWithWriter(cfg config.LoggerConfig, w io.Writer) (*slog.Logger, error) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("logger.level must be one of: debug, info, warn, error")
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("logger.format must be one of: text, json")
	}

	return slog.New(handler), nil
}

// WithComponent returns a child logger with the standard component field set.
// If base is nil, a safe discard logger is returned.
func WithComponent(base *slog.Logger, component string) *slog.Logger {
	if base == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return base.With(FieldComponent, component)
}

// ErrorAttr returns a slog.Attr for logging an error consistently.
// A nil error produces an attribute with an empty string value.
func ErrorAttr(err error) slog.Attr {
	if err != nil {
		return slog.String(FieldError, err.Error())
	}
	return slog.String(FieldError, "")
}

// RedactAttr returns a copy of attr with its value replaced by RedactedValue
// if the key matches a known sensitive key. Matching is case-insensitive.
// Non-sensitive attributes are returned unchanged.
func RedactAttr(attr slog.Attr) slog.Attr {
	if attr.Key == "" {
		return attr
	}
	for k := range sensitiveKeys {
		if strings.EqualFold(attr.Key, k) {
			return slog.String(attr.Key, RedactedValue)
		}
	}
	return attr
}

// LevelController wraps slog.LevelVar to provide runtime-mutable log levels.
// It is a standalone foundation for future config reload; it is not wired
// into New(cfg) in this plan.
type LevelController struct {
	level slog.LevelVar
}

// NewLevelController creates a LevelController initialized to the named level.
// Valid levels are: debug, info, warn, error.
func NewLevelController(level string) (*LevelController, error) {
	c := &LevelController{}
	if err := c.SetLevel(level); err != nil {
		return nil, err
	}
	return c, nil
}

// HandlerOptions returns slog.HandlerOptions that use the controller's level.
func (c *LevelController) HandlerOptions() *slog.HandlerOptions {
	return &slog.HandlerOptions{Level: &c.level}
}

// SetLevel updates the controller's level at runtime.
// Valid levels are: debug, info, warn, error.
func (c *LevelController) SetLevel(level string) error {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		return fmt.Errorf("logger.level must be one of: debug, info, warn, error")
	}
	c.level.Set(l)
	return nil
}
