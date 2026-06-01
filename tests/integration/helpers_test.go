package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/plomvix/plomvix/internal/auth"
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
)

func init() {
	_ = logger.Init("error", "pretty")
}

func testConfig() *config.Config {
	return &config.Config{
		Env: "development",
		Auth: config.AuthConfig{
			DefaultAdminUsername: "admin",
			DefaultAdminPassword: "integration-test-pass",
			JWTSecret:            "integration-test-secret-do-not-use-in-prod",
			JWTExpirySeconds:     3600,
			APIKeyLength:         32,
		},
	}
}

func setupTestServer(t *testing.T) (*httptest.Server, *config.Config, *auth.Store, *auth.Blacklist) {
	t.Helper()
	cfg := testConfig()

	store, err := auth.NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	blacklist := auth.NewBlacklist()
	t.Cleanup(func() { blacklist.Stop() })

	if err := auth.BootstrapAdminUser(store, cfg); err != nil {
		t.Fatalf("failed to bootstrap admin user: %v", err)
	}

	handler := auth.NewHandler(store, blacklist, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", handler.Login)

	mux.Handle("/auth/logout", auth.Middleware(store, blacklist, cfg)(http.HandlerFunc(handler.Logout)))
	mux.Handle("/auth/refresh", auth.Middleware(store, blacklist, cfg)(http.HandlerFunc(handler.Refresh)))

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","data":{"protected":true}}`))
	})
	mux.Handle("/api/protected", auth.Middleware(store, blacklist, cfg)(protectedHandler))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, cfg, store, blacklist
}

func adminCookieJar(t *testing.T, srv *httptest.Server, cfg *config.Config) http.CookieJar {
	t.Helper()
	jar := newTestCookieJar()
	client := &http.Client{Jar: jar}

	body := `{"username":"` + cfg.Auth.DefaultAdminUsername + `","password":"` + cfg.Auth.DefaultAdminPassword + `"}`
	resp, err := client.Post(srv.URL+"/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login returned %d", resp.StatusCode)
	}
	return jar
}

type testCookieJar struct {
	mu      sync.Mutex
	cookies map[string][]*http.Cookie
}

func newTestCookieJar() *testCookieJar {
	return &testCookieJar{cookies: make(map[string][]*http.Cookie)}
}

func (j *testCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range cookies {
		existing := j.cookies[u.Host]
		replaced := false
		for i, ec := range existing {
			if ec.Name == c.Name && ec.Path == c.Path {
				existing[i] = c
				replaced = true
				break
			}
		}
		if !replaced {
			j.cookies[u.Host] = append(existing, c)
		}
	}
}

func (j *testCookieJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cookies[u.Host]
}
