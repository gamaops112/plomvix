package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	adminpkg "github.com/plomvix/plomvix/internal/admin"
	"github.com/plomvix/plomvix/internal/auth"
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/ingestion"
	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/query"
	themestore "github.com/plomvix/plomvix/internal/theme"
	"github.com/plomvix/plomvix/pkg/utils"

	walmanager "github.com/plomvix/plomvix/internal/storage/wal"

	hotmanager "github.com/plomvix/plomvix/internal/storage/hot"

	coldstore "github.com/plomvix/plomvix/internal/storage/cold"
)

type Server struct {
	router       *chi.Mux
	cfg          *config.Config
	httpServer   *http.Server
	startTime    time.Time
	version      string
	store        *auth.Store
	blacklist    *auth.Blacklist
	wal          *walmanager.Manager
	hotTier      *hotmanager.Manager
	cold         *coldstore.Store
	tierEngine   *coldstore.TieringEngine
	adminHandler *adminpkg.Handler
	themeStore   *themestore.Store
}

func New(cfg *config.Config, version, buildTime, gitCommit string,
	store *auth.Store, blacklist *auth.Blacklist,
	wal *walmanager.Manager, hotTier *hotmanager.Manager,
	cold *coldstore.Store, tierEngine *coldstore.TieringEngine,
	themeStore *themestore.Store) *Server {
	s := &Server{
		router:     chi.NewRouter(),
		cfg:        cfg,
		startTime:  time.Now(),
		version:    version,
		store:      store,
		blacklist:  blacklist,
		wal:        wal,
		hotTier:    hotTier,
		cold:       cold,
		tierEngine: tierEngine,
		adminHandler: adminpkg.NewHandler(
			wal, hotTier, cold,
			version, buildTime, gitCommit,
			time.Now(),
		),
		themeStore: themeStore,
	}
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}
	s.setupMiddleware()
	s.setupRoutes()
	return s
}

func (s *Server) Start() error {
	logger.Info("http server listening", zap.String("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) setupMiddleware() {
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := utils.NewRequestID()
			r.Header.Set("X-Request-ID", requestID)
			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r)
		})
	})

	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			logger.Info("request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int64("latency_ms", time.Since(start).Milliseconds()),
				zap.String("request_id", r.Header.Get("X-Request-ID")),
			)
		})
	})

	s.router.Use(middleware.Recoverer)

	s.router.Use(middleware.Timeout(
		time.Duration(s.cfg.Server.WriteTimeout) * time.Second,
	))

	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})
}

func (s *Server) setupRoutes() {
	authHandler := auth.NewHandler(s.store, s.blacklist, s.cfg)
	userHandler := auth.NewUserHandler(s.store, s.cfg)
	apiKeyHandler := auth.NewAPIKeyHandler(s.store, s.cfg)

	// Public — no auth
	s.router.Get("/health", s.handleHealth)
	s.router.Post("/auth/login", authHandler.Login)
	s.router.Post("/auth/refresh", authHandler.Refresh)
	s.router.Get("/openapi.json", s.handleOpenAPISpec)
	s.router.Get("/docs", s.handleDocs)

	themeHandler := themestore.NewHandler(s.themeStore)
	s.router.Get("/api/theme", themeHandler.GetTheme)

	// Protected — auth required
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Post("/auth/logout", authHandler.Logout)
	})

	// Ingestion — auth required
	ingestHandler := ingestion.NewHandler(s.hotTier, s.wal)
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Post("/ingest/logs", ingestHandler.IngestLogs)
		r.Post("/ingest/metrics", ingestHandler.IngestMetrics)
		r.Post("/ingest/json", ingestHandler.IngestJSON)
		r.Post("/ingest/kv", ingestHandler.IngestKV)
	})

	// Query — auth required
	queryEngine := query.NewEngine(s.hotTier, s.cold)
	queryHandler := query.NewHandler(queryEngine)
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Get("/query/logs", queryHandler.QueryLogs)
		r.Get("/query/metrics", queryHandler.QueryMetrics)
		r.Get("/query/json", queryHandler.QueryJSON)
		r.Get("/query/kv/{key}", queryHandler.QueryKV)
		r.Get("/query/schema/{type}", queryHandler.QuerySchema)
	})

	// Admin only — auth + admin role
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Use(auth.RequireAdmin())
		r.Post("/admin/users", userHandler.Create)
		r.Get("/admin/users", userHandler.List)
		r.Get("/admin/users/{id}", userHandler.Get)
		r.Patch("/admin/users/{id}", userHandler.Update)
		r.Delete("/admin/users/{id}", userHandler.Delete)
		r.Post("/admin/users/{id}/apikey", apiKeyHandler.Generate)
		r.Delete("/admin/users/{id}/apikey", apiKeyHandler.Revoke)
		r.Get("/admin/users/{id}/apikey/status", apiKeyHandler.Status)
		r.Post("/admin/tier/flush", s.handleTierFlush)
		r.Get("/admin/stats", s.adminHandler.Stats)
		r.Get("/admin/info", s.adminHandler.Info)
		r.Get("/admin/wal/stats", s.adminHandler.WALStats)
		r.Post("/admin/wal/rotate", s.adminHandler.WALRotate)
		r.Get("/admin/cold/stats", s.adminHandler.ColdStats)
		r.Get("/admin/schema", s.adminHandler.SchemaList)
		r.Delete("/admin/schema/{type}", s.adminHandler.SchemaDelete)
	})

	// Theme mutation — admin only
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Use(auth.RequireAdmin())
		r.Put("/api/theme", themeHandler.UpdateTheme)
		r.Post("/api/theme/reset", themeHandler.ResetTheme)
		r.Get("/api/theme/export", themeHandler.ExportTheme)
	})

	// UI routes — served last so they cannot shadow API routes
	if s.cfg.UI.Enabled {
		if s.cfg.UI.DevMode {
			uiProxy, err := newUIProxyHandler("http://localhost:3000")
			if err != nil {
				// misconfigured target URL — fail at startup, not at request time
				panic(fmt.Sprintf("failed to create UI proxy handler: %v", err))
			}
			s.router.Handle("/app", uiProxy)
			s.router.Handle("/app/*", uiProxy)
			s.router.Handle("/login", http.RedirectHandler("/app/login", http.StatusMovedPermanently))
			s.router.Handle("/logout", http.RedirectHandler("/app/logout", http.StatusMovedPermanently))
			s.router.Handle("/forgot-password", http.RedirectHandler("/app/forgot-password", http.StatusMovedPermanently))
		} else {
			uiHandler := newSPAHandler("obs_theme/dist")
			s.router.Handle("/app", uiHandler)
			s.router.Handle("/app/*", uiHandler)
			s.router.Handle("/login", http.RedirectHandler("/app/login", http.StatusMovedPermanently))
			s.router.Handle("/logout", http.RedirectHandler("/app/logout", http.StatusMovedPermanently))
			s.router.Handle("/forgot-password", http.RedirectHandler("/app/forgot-password", http.StatusMovedPermanently))
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dataDirs := []string{
		filepath.Join(s.cfg.Storage.DataDir, "wal"),
		filepath.Join(s.cfg.Storage.DataDir, "hot"),
		filepath.Join(s.cfg.Storage.DataDir, "cold", "logs"),
		filepath.Join(s.cfg.Storage.DataDir, "cold", "metrics"),
		filepath.Join(s.cfg.Storage.DataDir, "cold", "json"),
		filepath.Join(s.cfg.Storage.DataDir, "cold", "kv"),
	}

	var failures []string
	for _, dir := range dataDirs {
		if !utils.IsWritable(dir) {
			failures = append(failures,
				fmt.Sprintf("data directory not writable: %s", dir))
		}
	}

	if len(failures) > 0 {
		utils.ServiceUnavailable(w, r,
			utils.CodeHealthCheckFailed,
			"One or more health checks failed",
			failures...,
		)
		return
	}

	walStats := s.wal.Stats()
	hotStats := s.hotTier.Stats()
	var coldData map[string]interface{}
	if s.cold != nil {
		cs := s.cold.Stats()
		coldData = map[string]interface{}{
			"parquet_files": cs.TotalParquetFiles,
			"records_moved": cs.TotalRecordsMoved,
			"last_flush_at": cs.LastFlushAt,
		}
	}
	utils.OK(w, r, map[string]interface{}{
		"version":        s.version,
		"env":            s.cfg.Env,
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
		"pid":            os.Getpid(),
		"go_version":     utils.GetGoVersion(),
		"os_arch":        utils.GetOSArch(),
		"wal": map[string]interface{}{
			"segment_count":     walStats.SegmentCount,
			"active_segment":    walStats.ActiveSegment,
			"active_size_bytes": walStats.ActiveSizeBytes,
			"total_entries":     walStats.TotalEntries,
		},
		"hot": map[string]interface{}{
			"total_writes":      hotStats.TotalWrites,
			"total_data_writes": hotStats.TotalDataWrites,
			"data_dir":          hotStats.DataDir,
		},
		"cold": coldData,
	})
}

// handleTierFlush handles POST /admin/tier/flush.
//
// POST /admin/tier/flush
// Auth: admin only
//
// Responses:
//
//	200 OK       — flush complete, returns stats
//	500 Internal — INTERNAL_ERROR: flush failed
func (s *Server) handleTierFlush(w http.ResponseWriter, r *http.Request) {
	if s.tierEngine == nil || s.cold == nil {
		utils.InternalError(w, r, "tier engine not available")
		return
	}
	if err := s.tierEngine.Flush(); err != nil {
		utils.InternalError(w, r, "tier flush failed")
		return
	}
	stats := s.cold.Stats()
	utils.OK(w, r, map[string]interface{}{
		"message":        "tier flush complete",
		"records_moved":  stats.TotalRecordsMoved,
		"parquet_files":  stats.TotalParquetFiles,
		"last_flush_at":  stats.LastFlushAt,
		"flush_duration": stats.LastFlushDuration.String(),
	})
}

// handleOpenAPISpec serves the OpenAPI 3.0 JSON specification.
//
// GET /openapi.json
// Auth: none (public)
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec, err := os.ReadFile("api/openapi.json")
	if err != nil {
		utils.InternalError(w, r, "openapi spec not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(spec)
}

// docsHTML is the Stoplight Elements API documentation page.
// Elements is loaded via CDN — no build step required.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plomvix API Docs</title>
  <script src="https://unpkg.com/@stoplight/elements@7.16.6/web-components.min.js"></script>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements@7.16.6/styles.min.css">
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    elements-api { display: block; height: 100%; }
    #fallback { display: none; padding: 2rem; font-family: system-ui, sans-serif; }
    #fallback a { color: #2563eb; }
  </style>
</head>
<body>
  <elements-api id="api" router="hash" layout="sidebar"></elements-api>
  <div id="fallback">
    <h2>API Documentation</h2>
    <p>Could not load the OpenAPI specification.</p>
    <p>Try:</p>
    <ul>
      <li>Disabling ad blockers / tracking protection for this page</li>
      <li>Ensuring <code>unpkg.com</code> is reachable from your network</li>
      <li>Downloading the raw <a href="/openapi.json">OpenAPI spec</a></li>
    </ul>
  </div>
  <script>
    fetch('/openapi.json')
      .then(function(r) { return r.json(); })
      .then(function(spec) {
        var el = document.getElementById('api');
        if (el) el.apiDescriptionDocument = spec;
      })
      .catch(function() {
        document.getElementById('fallback').style.display = 'block';
      });
  </script>
</body>
</html>`

// handleDocs serves the Stoplight Elements interactive API documentation UI.
//
// GET /docs
// Auth: none (public)
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(docsHTML))
}
