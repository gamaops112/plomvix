package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			DefaultAdminUsername: "admin",
			DefaultAdminPassword: "changeme",
			JWTSecret:            "test-secret-key-minimum-length",
			JWTExpirySeconds:     3600,
			APIKeyLength:         32,
		},
	}
}

func testUser() *User {
	return &User{
		ID:       "test-user-id",
		Username: "testuser",
		Role:     RoleAdmin,
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("password")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if !strings.HasPrefix(hash, "$2a$12$") {
		t.Fatalf("expected hash to start with $2a$12$, got: %s", hash)
	}

	hash2, _ := HashPassword("password")
	if hash == hash2 {
		t.Fatal("expected different hashes due to bcrypt salting")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("password")
	if err := CheckPassword("password", hash); err != nil {
		t.Fatalf("expected match, got error: %v", err)
	}
	if err := CheckPassword("wrong", hash); err == nil {
		t.Fatal("expected mismatch, got nil error")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(testUser(), testConfig())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got: %d", len(parts))
	}

	token2, _ := GenerateToken(testUser(), testConfig())
	if token == token2 {
		t.Fatal("expected different tokens (different JTI)")
	}

	claims, err := ParseToken(token, testConfig())
	if err != nil {
		t.Fatalf("expected parse success, got: %v", err)
	}
	if claims.UserID != testUser().ID {
		t.Fatalf("expected UserID=%s, got=%s", testUser().ID, claims.UserID)
	}
	if claims.Role != RoleAdmin {
		t.Fatalf("expected Role=admin, got=%s", claims.Role)
	}
	if claims.JTI == "" {
		t.Fatal("expected non-empty JTI")
	}
}

func TestParseToken(t *testing.T) {
	valid, _ := GenerateToken(testUser(), testConfig())
	claims, err := ParseToken(valid, testConfig())
	if err != nil {
		t.Fatalf("expected parse success, got: %v", err)
	}
	if claims == nil {
		t.Fatal("expected non-nil claims")
	}
	if claims.UserID != "test-user-id" {
		t.Fatalf("expected UserID=test-user-id, got=%s", claims.UserID)
	}
	if claims.JTI == "" {
		t.Fatal("expected non-empty JTI")
	}

	if _, err := ParseToken("", testConfig()); err == nil {
		t.Fatal("expected error for empty token")
	}
	if _, err := ParseToken("not.a.token", testConfig()); err == nil {
		t.Fatal("expected error for invalid token")
	}

	wrongCfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:        "wrong-secret-key-minimum-length",
			JWTExpirySeconds: 3600,
		},
	}
	if _, err := ParseToken(valid, wrongCfg); err == nil {
		t.Fatal("expected error for wrong secret")
	}

	expiredCfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:        "test-secret-key-minimum-length",
			JWTExpirySeconds: -3600,
		},
	}
	expiredToken, _ := GenerateToken(testUser(), expiredCfg)
	if _, err := ParseToken(expiredToken, expiredCfg); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestBlacklist(t *testing.T) {
	b := NewBlacklist()
	defer b.Stop()

	if b.IsBlacklisted("unknown-jti") {
		t.Fatal("expected unknown JTI to not be blacklisted")
	}

	b.Add("jti-1", time.Now().Add(1*time.Hour))
	if !b.IsBlacklisted("jti-1") {
		t.Fatal("expected jti-1 to be blacklisted")
	}

	b.Add("jti-2", time.Now().Add(-1*time.Hour))
	if b.IsBlacklisted("jti-2") {
		t.Fatal("expected expired jti-2 to not be blacklisted")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	plaintext, hash, err := GenerateAPIKey(testConfig())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected non-empty plaintext")
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if plaintext == hash {
		t.Fatal("expected plaintext != hash")
	}
	if err := CheckAPIKey(plaintext, hash); err != nil {
		t.Fatalf("expected match, got error: %v", err)
	}
	if err := CheckAPIKey("wrong", hash); err == nil {
		t.Fatal("expected mismatch for wrong key")
	}

	plaintext2, _, _ := GenerateAPIKey(testConfig())
	if plaintext == plaintext2 {
		t.Fatal("expected different plaintexts")
	}
}
