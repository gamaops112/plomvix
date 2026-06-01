package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// newSPAHandler returns an http.Handler that serves a Vite-built React SPA
// from distDir. It handles three cases:
//  1. index.html missing → 503 with instructions to run make ui-build
//  2. real asset file (e.g. /app/assets/main.js) → serve the file
//  3. any other /app/* route → serve index.html (client-side routing)
func newSPAHandler(distDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "Plomvix UI is not built. Run make ui-build.", http.StatusServiceUnavailable)
			return
		}

		// Non-/app routes (login, logout, dev/design): serve index.html for client-side routing
		if !strings.HasPrefix(r.URL.Path, "/app") {
			http.ServeFile(w, r, indexPath)
			return
		}

		// Strip the /app prefix to get the path relative to distDir
		trimmed := strings.TrimPrefix(r.URL.Path, "/app")
		if trimmed == "" || trimmed == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}

		// Path traversal guard — reject any path that escapes distDir
		cleanDist := filepath.Clean(distDir)
		requestedPath := filepath.Join(cleanDist, filepath.Clean(trimmed))
		if requestedPath != cleanDist &&
			!strings.HasPrefix(requestedPath, cleanDist+string(os.PathSeparator)) {
			http.NotFound(w, r)
			return
		}

		// Serve real files that exist (e.g. assets/main.js, assets/main.css)
		if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
			http.StripPrefix("/app", fileServer).ServeHTTP(w, r)
			return
		}

		// Assets path with no matching file → 404 (do not fall through to index.html)
		if strings.HasPrefix(trimmed, "/assets/") {
			http.NotFound(w, r)
			return
		}

		// All other routes → serve index.html for client-side routing
		http.ServeFile(w, r, indexPath)
	})
}
