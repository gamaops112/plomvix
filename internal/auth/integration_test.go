package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/plomvix/plomvix/internal/logger"
)

func init() {
	_ = logger.Init("error", "pretty")
}

func newTestRouter(t *testing.T) (http.Handler, *Store) {
	t.Helper()
	store := newTestStore(t)
	cfg := testConfig()
	blacklist := NewBlacklist()
	t.Cleanup(func() { blacklist.Stop() })

	_ = logger.Init("error", "pretty")

	if err := BootstrapAdminUser(store, cfg); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	r := chi.NewRouter()
	authHandler := NewHandler(store, blacklist, cfg)
	userHandler := NewUserHandler(store, cfg)

	r.Post("/auth/login", authHandler.Login)
	r.Group(func(r chi.Router) {
		r.Use(Middleware(store, blacklist, cfg))
		r.Post("/auth/logout", authHandler.Logout)
		r.Post("/auth/refresh", authHandler.Refresh)
	})
	r.Group(func(r chi.Router) {
		r.Use(Middleware(store, blacklist, cfg))
		r.Use(RequireAdmin())
		r.Post("/admin/users", userHandler.Create)
		r.Get("/admin/users", userHandler.List)
	})
	return r, store
}

func loginAs(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username, "password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: status %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	return resp["data"].(map[string]interface{})["token"].(string)
}

func TestLoginSuccess(t *testing.T) {
	router, _ := newTestRouter(t)
	body, _ := json.Marshal(map[string]string{
		"username": "admin", "password": "changeme",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field in response")
	}
	if _, ok := data["token"]; !ok {
		t.Fatal("expected token in data")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	router, _ := newTestRouter(t)
	body, _ := json.Marshal(map[string]string{
		"username": "admin", "password": "wrong",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got: %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["message"] != "invalid username or password" {
		t.Fatalf("expected 'invalid username or password', got: %s", errObj["message"])
	}
}

func TestLoginWrongUsername(t *testing.T) {
	router, _ := newTestRouter(t)
	body, _ := json.Marshal(map[string]string{
		"username": "nobody", "password": "changeme",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got: %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["message"] != "invalid username or password" {
		t.Fatalf("expected 'invalid username or password', got: %s", errObj["message"])
	}
}

func TestLoginMissingFields(t *testing.T) {
	router, _ := newTestRouter(t)
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got: %s", errObj["code"])
	}
}

func TestProtectedRouteWithJWT(t *testing.T) {
	router, _ := newTestRouter(t)
	token := loginAs(t, router, "admin", "changeme")

	body, _ := json.Marshal(map[string]string{
		"username": "newuser", "password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestProtectedRouteNoAuth(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got: %d", w.Code)
	}
}

func TestProtectedRouteWithAPIKey(t *testing.T) {
	router, store := newTestRouter(t)

	plaintext, hash, _ := GenerateAPIKey(testConfig())
	admin, _ := store.GetUserByUsername("admin")
	admin.APIKeyHash = hash
	store.UpdateUser(admin)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("X-API-Key", plaintext)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", w.Code)
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	router, _ := newTestRouter(t)
	token := loginAs(t, router, "admin", "changeme")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got: %d", w.Code)
	}
}

func TestRefreshIssuesNewToken(t *testing.T) {
	router, _ := newTestRouter(t)
	tokenA := loginAs(t, router, "admin", "changeme")

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	tokenB := resp["data"].(map[string]interface{})["token"].(string)

	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected tokenA blacklisted (401), got: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected tokenB valid (200), got: %d", w.Code)
	}
}

func TestInvalidAPIKey(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("X-API-Key", "wrongkey")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid API key, got: %d", w.Code)
	}
}
