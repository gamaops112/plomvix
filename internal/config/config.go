// Package config provides configuration structures and defaults for Plomvix.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config is the top-level configuration structure.
type Config struct {
	Server ServerConfig `toml:"server"`
	Logger LoggerConfig `toml:"logger"`
	SQL    SQLConfig    `toml:"sql_engine"`
	Store  StoreConfig  `toml:"storage"`
}

// ServerConfig holds network-related configuration.
type ServerConfig struct {
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	MaxConnections int64  `toml:"max_connections"`
	SSLEnabled     bool   `toml:"ssl_enabled"`
	SSLCertPath    string `toml:"ssl_cert_path"`
	SSLKeyPath     string `toml:"ssl_key_path"`
	AuthType       string `toml:"auth_type"`
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}

// SQLConfig holds sql_engine configuration.
type SQLConfig struct {
	DataDir         string `toml:"data_dir"`
	MaxMutationRows int    `toml:"max_mutation_rows"`
	VacuumWorkers   int    `toml:"vacuum_workers"`
	VacuumQueueSize int    `toml:"vacuum_queue_size"`
}

// StoreConfig holds storage configuration.
type StoreConfig struct {
	DBPath       string `toml:"db_path"`
	WALPath      string `toml:"wal_path"`
	CacheSizeMB  int    `toml:"cache_size_mb"`
	SyncWrites   bool   `toml:"sync_writes"`
	MaxOpenFiles int    `toml:"max_open_files"`
}

// Default returns a Config populated with sensible default values.
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host:           "127.0.0.1",
			Port:           5432,
			MaxConnections: 100,
			AuthType:       "trust",
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		SQL: SQLConfig{
			DataDir:         "data/sql",
			MaxMutationRows: 1000,
			VacuumWorkers:   2,
			VacuumQueueSize: 100,
		},
		Store: StoreConfig{
			DBPath:       "data/plomvix.db",
			WALPath:      "data/plomvix.wal",
			CacheSizeMB:  64,
			SyncWrites:   true,
			MaxOpenFiles: 256,
		},
	}
}

// Validate checks a Config for correctness and returns an error describing
// any problem found. It returns nil when the config is valid.
func Validate(cfg Config) error {
	if cfg.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 0 and 65535")
	}
	if !validLoggerLevel(cfg.Logger.Level) {
		return fmt.Errorf("logger.level must be one of: debug, info, warn, error")
	}
	if !validLoggerFormat(cfg.Logger.Format) {
		return fmt.Errorf("logger.format must be one of: text, json")
	}
	if !validLoggerOutput(cfg.Logger.Output) {
		return fmt.Errorf("logger.output must be one of: stdout, stderr")
	}
	if cfg.SQL.DataDir == "" {
		return errors.New("sql_engine data_dir is required")
	}
	if cfg.Store.DBPath == "" {
		return errors.New("storage db_path is required")
	}
	return nil
}

func validLoggerLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	}
	return false
}

func validLoggerFormat(format string) bool {
	switch format {
	case "text", "json":
		return true
	}
	return false
}

func validLoggerOutput(output string) bool {
	switch output {
	case "stdout", "stderr":
		return true
	}
	return false
}

// Load reads a TOML configuration file from path, overlays it onto default
// values, validates the result, and returns the final Config. Default values
// are preserved for any keys not present in the file.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	defer file.Close()

	cfg := Default()

	decoder := toml.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	cfg = normalize(cfg)

	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// normalize returns a copy of cfg with paths cleaned via filepath.Clean.
// Empty paths are preserved as empty (not turned into ".").
func normalize(cfg Config) Config {
	if cfg.SQL.DataDir != "" {
		cfg.SQL.DataDir = filepath.Clean(cfg.SQL.DataDir)
	}
	if cfg.Store.DBPath != "" {
		cfg.Store.DBPath = filepath.Clean(cfg.Store.DBPath)
	}
	if cfg.Store.WALPath != "" {
		cfg.Store.WALPath = filepath.Clean(cfg.Store.WALPath)
	}
	return cfg
}
