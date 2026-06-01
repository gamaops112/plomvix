package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenFromRequest_BearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer my.jwt.token")
	token, source := TokenFromRequest(req)
	if token != "my.jwt.token" {
		t.Errorf("expected token 'my.jwt.token', got %q", token)
	}
	if source != TokenSourceBearer {
		t.Errorf("expected TokenSourceBearer, got %q", source)
	}
}

func TestTokenFromRequest_CookieToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: "cookie-jwt"})
	token, source := TokenFromRequest(req)
	if token != "cookie-jwt" {
		t.Errorf("expected token 'cookie-jwt', got %q", token)
	}
	if source != TokenSourceCookie {
		t.Errorf("expected TokenSourceCookie, got %q", source)
	}
}

func TestTokenFromRequest_BearerWinsOverCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bearer-token")
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: "cookie-token"})
	token, source := TokenFromRequest(req)
	if token != "bearer-token" {
		t.Errorf("expected bearer token to win, got %q", token)
	}
	if source != TokenSourceBearer {
		t.Errorf("expected TokenSourceBearer, got %q", source)
	}
}

func TestTokenFromRequest_EmptyCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: ""})
	_, source := TokenFromRequest(req)
	if source != TokenSourceNone {
		t.Errorf("expected TokenSourceNone for empty cookie, got %q", source)
	}
}

func TestTokenFromRequest_NoAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	token, source := TokenFromRequest(req)
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if source != TokenSourceNone {
		t.Errorf("expected TokenSourceNone, got %q", source)
	}
}

func TestTokenFromRequest_EmptyBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	_, source := TokenFromRequest(req)
	if source != TokenSourceNone {
		t.Errorf("expected TokenSourceNone for empty bearer, got %q", source)
	}
}
