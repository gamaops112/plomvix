package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newUIProxyHandler returns an http.Handler that reverse-proxies all requests
// to the target URL. Used in development to proxy /app/* to the Vite dev server.
func newUIProxyHandler(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}
