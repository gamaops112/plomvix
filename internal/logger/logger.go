package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Structured logging field conventions for Plomvix:
//
// Server startup:      zap.Int("port"), zap.String("host"), zap.String("version"), zap.Int("pid"), zap.String("env")
// HTTP request:        zap.String("method"), zap.String("path"), zap.Int("status"), zap.Int64("latency_ms"), zap.String("request_id")
// Storage operation:   zap.String("operation"), zap.String("data_type"), zap.Int64("bytes"), zap.Int64("duration_ms")
// Directory operation: zap.String("path"), zap.String("operation")
// Graceful shutdown:   zap.Int("timeout_seconds"), zap.String("reason")
//
// NEVER log: password, jwt_secret, api_key, or any field containing "secret" or "token".

var globalLogger *zap.Logger

func Init(level, format string) error {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("invalid log level %q: %w", level, err)
	}

	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	l, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	globalLogger = l
	return nil
}

func must() {
	if globalLogger == nil {
		panic("plomvix: logger not initialized — call logger.Init() first")
	}
}

func Debug(msg string, fields ...zap.Field) { must(); globalLogger.Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { must(); globalLogger.Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { must(); globalLogger.Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { must(); globalLogger.Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { must(); globalLogger.Fatal(msg, fields...) }
func Sync()                                 { must(); _ = globalLogger.Sync() }
