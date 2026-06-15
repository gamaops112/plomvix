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
	Data   DataConfig   `toml:"data"`
	Logger LoggerConfig `toml:"logger"`
}

// ServerConfig holds network-related configuration.
type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// DataConfig holds data storage configuration.
type DataConfig struct {
	Path string `toml:"path"`
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}

// Default returns a Config populated with sensible default values.
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Data: DataConfig{
			Path: "./data",
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

// Validate checks a Config for correctness and returns an error describing
// any problem found. It returns nil when the config is valid.
func Validate(cfg Config) error {
	if cfg.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if cfg.Data.Path == "" {
		return errors.New("data.path is required")
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
// It does not mutate the input.
func normalize(cfg Config) Config {
	cfg.Data.Path = filepath.Clean(cfg.Data.Path)
	return cfg
}
