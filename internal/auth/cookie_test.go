package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/config"
)

func TestNewTokenCookie_SetsName(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if c.Name != TokenCookieName {
		t.Errorf("expected cookie name %q, got %q", TokenCookieName, c.Name)
	}
}

func TestNewTokenCookie_HttpOnly(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if !c.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
}

func TestNewTokenCookie_Path(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if c.Path != "/" {
		t.Errorf("expected Path /, got %q", c.Path)
	}
}

func TestNewTokenCookie_SameSiteLax(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite Lax, got %v", c.SameSite)
	}
}

func TestNewTokenCookie_SecureFalseInDevelopment(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if c.Secure {
		t.Error("expected Secure to be false in development")
	}
}

func TestNewTokenCookie_SecureTrueInProduction(t *testing.T) {
	cfg := &config.Config{Env: "production", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	expires := time.Now().Add(time.Hour)
	c := NewTokenCookie("test-jwt", expires, cfg)
	if !c.Secure {
		t.Error("expected Secure to be true in production")
	}
}

func TestNewClearTokenCookie_MaxAgeMinusOne(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	c := NewClearTokenCookie(cfg)
	if c.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", c.MaxAge)
	}
}

func TestNewClearTokenCookie_EmptyValue(t *testing.T) {
	cfg := &config.Config{Env: "development", Auth: config.AuthConfig{JWTExpirySeconds: 3600}}
	c := NewClearTokenCookie(cfg)
	if c.Value != "" {
		t.Errorf("expected empty value, got %q", c.Value)
	}
}
