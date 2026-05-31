package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Env         string            `mapstructure:"env"`
	Server      ServerConfig      `mapstructure:"server"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Compression CompressionConfig `mapstructure:"compression"`
	Indexing    IndexingConfig    `mapstructure:"indexing"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

type StorageConfig struct {
	DataDir           string `mapstructure:"data_dir"`
	WALFlushThreshold int64  `mapstructure:"wal_flush_threshold"`
	HotTierMaxSize    int64  `mapstructure:"hot_tier_max_size"`
	RetentionDays     int    `mapstructure:"retention_days"`
}

type CompressionConfig struct {
	HotTier  string `mapstructure:"hot_tier"`
	ColdTier string `mapstructure:"cold_tier"`
}

type IndexingConfig struct {
	AutoIndexTimestamp bool `mapstructure:"auto_index_timestamp"`
}

type AuthConfig struct {
	DefaultAdminUsername string `mapstructure:"default_admin_username"`
	DefaultAdminPassword string `mapstructure:"default_admin_password"`
	JWTSecret            string `mapstructure:"jwt_secret"`
	JWTExpirySeconds     int    `mapstructure:"jwt_expiry_seconds"`
	APIKeyLength         int    `mapstructure:"api_key_length"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

var (
	instance *Config
	once     sync.Once
	loadErr  error
)

func Load(path string) (*Config, error) {
	once.Do(func() {
		v := viper.New()
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		v.SetEnvPrefix("PLOMVIX")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		if err := v.ReadInConfig(); err != nil {
			loadErr = fmt.Errorf("failed to read config file %q: %w", path, err)
			return
		}

		var cfg Config
		if err := v.Unmarshal(&cfg); err != nil {
			loadErr = fmt.Errorf("failed to parse config: %w", err)
			return
		}

		if err := validate(&cfg); err != nil {
			loadErr = err
			return
		}

		instance = &cfg
	})
	return instance, loadErr
}

func Get() *Config {
	if instance == nil {
		panic("plomvix: config not loaded — call config.Load() first")
	}
	return instance
}

func (c *Config) IsDevelopment() bool { return c.Env == "development" }
func (c *Config) IsProduction() bool  { return c.Env == "production" }

func validate(c *Config) error {
	var errs []string

	if c.Env != "development" && c.Env != "production" {
		errs = append(errs, fmt.Sprintf(`env must be "development" or "production", got: %q`, c.Env))
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be between 1 and 65535, got: %d", c.Server.Port))
	}
	if c.Server.Host == "" {
		errs = append(errs, "server.host must not be empty")
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, "server.read_timeout must be greater than 0")
	}
	if c.Server.WriteTimeout <= 0 {
		errs = append(errs, "server.write_timeout must be greater than 0")
	}
	if c.Server.IdleTimeout <= 0 {
		errs = append(errs, "server.idle_timeout must be greater than 0")
	}
	if c.Storage.DataDir == "" {
		errs = append(errs, "storage.data_dir must not be empty")
	}
	if c.Storage.WALFlushThreshold <= 0 {
		errs = append(errs, "storage.wal_flush_threshold must be greater than 0")
	}
	if c.Storage.HotTierMaxSize <= 0 {
		errs = append(errs, "storage.hot_tier_max_size must be greater than 0")
	}
	if c.Storage.RetentionDays < 0 {
		errs = append(errs, "storage.retention_days must not be negative")
	}

	validHot := map[string]bool{"snappy": true, "lz4": true, "none": true}
	if !validHot[c.Compression.HotTier] {
		errs = append(errs, fmt.Sprintf(`compression.hot_tier must be one of [snappy lz4 none], got: %q`, c.Compression.HotTier))
	}
	validCold := map[string]bool{"zstd": true, "snappy": true, "none": true}
	if !validCold[c.Compression.ColdTier] {
		errs = append(errs, fmt.Sprintf(`compression.cold_tier must be one of [zstd snappy none], got: %q`, c.Compression.ColdTier))
	}

	if c.Auth.DefaultAdminUsername == "" {
		errs = append(errs, "auth.default_admin_username must not be empty")
	}
	if c.Auth.DefaultAdminPassword == "" {
		errs = append(errs, "auth.default_admin_password must not be empty")
	}
	if c.Auth.JWTSecret == "" {
		errs = append(errs, "auth.jwt_secret must not be empty")
	}
	if c.Env == "production" && c.Auth.JWTSecret == "plomvix-change-in-prod" {
		errs = append(errs, "auth.jwt_secret must be changed from default in production mode")
	}
	if c.Env == "production" && c.Auth.DefaultAdminPassword == "changeme" {
		errs = append(errs, "auth.default_admin_password must be changed from default in production mode")
	}
	if c.Auth.JWTExpirySeconds <= 0 {
		errs = append(errs, "auth.jwt_expiry_seconds must be greater than 0")
	}
	if c.Auth.APIKeyLength < 16 || c.Auth.APIKeyLength > 64 {
		errs = append(errs, fmt.Sprintf("auth.api_key_length must be between 16 and 64, got: %d", c.Auth.APIKeyLength))
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		errs = append(errs, fmt.Sprintf(`logging.level must be one of [debug info warn error], got: %q`, c.Logging.Level))
	}
	validFormats := map[string]bool{"json": true, "pretty": true}
	if !validFormats[c.Logging.Format] {
		errs = append(errs, fmt.Sprintf(`logging.format must be one of [json pretty], got: %q`, c.Logging.Format))
	}

	if len(errs) > 0 {
		return fmt.Errorf("plomvix config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
