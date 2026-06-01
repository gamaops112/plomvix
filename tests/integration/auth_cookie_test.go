package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/plomvix/plomvix/internal/auth"
)

func srvURL(srv *httptest.Server) (*url.URL, error) {
	return url.Parse(srv.URL)
}

func TestBrowserCookieLoginFlow(t *testing.T) {
	srv, cfg, _, _ := setupTestServer(t)

	jar := adminCookieJar(t, srv, cfg)
	client := &http.Client{Jar: jar}

	u, _ := srvURL(srv)
	cookies := jar.Cookies(u)
	var tokenCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.TokenCookieName {
			tokenCookie = c
			break
		}
	}
	if tokenCookie == nil {
		t.Fatal("expected plomvix_token cookie in jar after login")
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/protected", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("protected request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cookie-authenticated protected request, got %d", resp.StatusCode)
	}

	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/logout", nil)
	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", logoutResp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/protected", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("post-logout request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp2.StatusCode)
	}
}

func TestBrowserCookieRefreshFlow(t *testing.T) {
	srv, cfg, _, _ := setupTestServer(t)

	jar := adminCookieJar(t, srv, cfg)
	client := &http.Client{Jar: jar}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/refresh", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cookie refresh, got %d", resp.StatusCode)
	}

	var foundNewCookie bool
	for _, c := range resp.Cookies() {
		if c.Name == auth.TokenCookieName && c.Value != "" {
			foundNewCookie = true
			break
		}
	}
	if !foundNewCookie {
		t.Error("expected new Set-Cookie for plomvix_token after refresh")
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/protected", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("post-refresh protected request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cookie-authenticated request after refresh, got %d", resp2.StatusCode)
	}
}
