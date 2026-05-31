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

	"github.com/plomvix/plomvix/internal/auth"
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/ingestion"
	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/pkg/utils"

	walmanager "github.com/plomvix/plomvix/internal/storage/wal"

	hotmanager "github.com/plomvix/plomvix/internal/storage/hot"
)

type Server struct {
	router     *chi.Mux
	cfg        *config.Config
	httpServer *http.Server
	startTime  time.Time
	version    string
	store      *auth.Store
	blacklist  *auth.Blacklist
	wal        *walmanager.Manager
	hotTier    *hotmanager.Manager
}

func New(cfg *config.Config, version string, store *auth.Store,
	blacklist *auth.Blacklist, wal *walmanager.Manager,
	hotTier *hotmanager.Manager) *Server {
	s := &Server{
		router:    chi.NewRouter(),
		cfg:       cfg,
		startTime: time.Now(),
		version:   version,
		store:     store,
		blacklist: blacklist,
		wal:       wal,
		hotTier:   hotTier,
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

	// Protected — auth required
	s.router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
		r.Post("/auth/logout", authHandler.Logout)
		r.Post("/auth/refresh", authHandler.Refresh)
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
	})
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
	})
}
