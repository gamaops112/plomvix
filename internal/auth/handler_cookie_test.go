package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/plomvix/plomvix/internal/config"
)

func setupTestHandler(t *testing.T) (*httptest.Server, *config.Config, string, string) {
	t.Helper()

	cfg := testConfig()
	store := newTestStore(t)
	blacklist := NewBlacklist()
	t.Cleanup(func() { blacklist.Stop() })

	password := "test-password-123"
	username := "cookietest"

	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         RoleAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewHandler(store, blacklist, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", handler.Login)
	mux.Handle("/auth/logout", Middleware(store, blacklist, cfg)(http.HandlerFunc(handler.Logout)))
	mux.Handle("/auth/refresh", Middleware(store, blacklist, cfg)(http.HandlerFunc(handler.Refresh)))

	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/api/", Middleware(store, blacklist, cfg)(protectedMux))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, cfg, username, password
}

func TestLoginSetsCookie(t *testing.T) {
	srv, _, username, password := setupTestHandler(t)

	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/login", body)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cookies := resp.Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == TokenCookieName {
			found = true
			if !c.HttpOnly {
				t.Error("expected cookie to be HttpOnly")
			}
			if c.MaxAge <= 0 {
				t.Error("expected positive MaxAge")
			}
			break
		}
	}
	if !found {
		t.Error("expected Set-Cookie header with plomvix_token")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	srv, _, username, password := setupTestHandler(t)

	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == TokenCookieName {
			cookie = c
			break
		}
	}
	resp.Body.Close()
	if cookie == nil {
		t.Fatal("login did not set cookie")
	}

	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/logout", nil)
	req2.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var clearCookie *http.Cookie
	for _, c := range resp2.Cookies() {
		if c.Name == TokenCookieName {
			clearCookie = c
			break
		}
	}
	if clearCookie == nil {
		t.Error("expected clear-cookie Set-Cookie header")
	}
	if clearCookie != nil && clearCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1 in clear cookie, got %d", clearCookie.MaxAge)
	}
}

func TestRefreshWithCookie(t *testing.T) {
	srv, _, username, password := setupTestHandler(t)

	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == TokenCookieName {
			cookie = c
			break
		}
	}
	resp.Body.Close()
	if cookie == nil {
		t.Fatal("login did not set cookie")
	}

	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/refresh", nil)
	req2.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var newCookie *http.Cookie
	for _, c := range resp2.Cookies() {
		if c.Name == TokenCookieName && c.Value != "" {
			newCookie = c
			break
		}
	}
	if newCookie == nil {
		t.Error("expected a new Set-Cookie for plomvix_token after refresh")
	}
}

func TestCookieAuthenticatedRequest(t *testing.T) {
	srv, _, username, password := setupTestHandler(t)

	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == TokenCookieName {
			cookie = c
			break
		}
	}
	resp.Body.Close()
	if cookie == nil {
		t.Fatal("login did not set cookie")
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/protected", nil)
	req2.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("protected request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cookie-authenticated request, got %d", resp2.StatusCode)
	}
}
