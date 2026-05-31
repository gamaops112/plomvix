package admin

import (
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/ingestion"
	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/storage/cold"
	hotstore "github.com/plomvix/plomvix/internal/storage/hot"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
	"github.com/plomvix/plomvix/pkg/utils"
)

// Handler handles all Sprint 9 admin endpoints.
type Handler struct {
	wal       *walstore.Manager
	hot       *hotstore.Manager
	cold      *cold.Store
	version   string
	buildTime string
	gitCommit string
	startTime time.Time
}

// NewHandler creates a new admin Handler.
func NewHandler(
	wal *walstore.Manager,
	hot *hotstore.Manager,
	cold *cold.Store,
	version, buildTime, gitCommit string,
	startTime time.Time,
) *Handler {
	return &Handler{
		wal:       wal,
		hot:       hot,
		cold:      cold,
		version:   version,
		buildTime: buildTime,
		gitCommit: gitCommit,
		startTime: startTime,
	}
}

// Stats handles GET /admin/stats.
//
// GET /admin/stats
// Auth: admin only
//
// Responses:
//   200 OK — consolidated system stats
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	walStats := h.wal.Stats()
	hotStats := h.hot.Stats()

	var coldStats cold.TierStats
	if h.cold != nil {
		coldStats = h.cold.Stats()
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	utils.OK(w, r, map[string]interface{}{
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
		"cold": map[string]interface{}{
			"parquet_files": coldStats.TotalParquetFiles,
			"records_moved": coldStats.TotalRecordsMoved,
			"last_flush_at": coldStats.LastFlushAt,
		},
		"runtime": map[string]interface{}{
			"goroutines":  runtime.NumGoroutine(),
			"alloc_bytes": memStats.Alloc,
			"sys_bytes":   memStats.Sys,
			"gc_cycles":   memStats.NumGC,
		},
	})
}

// Info handles GET /admin/info.
//
// GET /admin/info
// Auth: admin only
//
// Responses:
//   200 OK — version and build info
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	utils.OK(w, r, map[string]interface{}{
		"version":        h.version,
		"build_time":     h.buildTime,
		"git_commit":     h.gitCommit,
		"go_version":     runtime.Version(),
		"os_arch":        runtime.GOOS + "/" + runtime.GOARCH,
		"uptime_seconds": int64(time.Since(h.startTime).Seconds()),
	})
}

// WALStats handles GET /admin/wal/stats.
//
// GET /admin/wal/stats
// Auth: admin only
//
// Responses:
//   200 OK — WAL stats
func (h *Handler) WALStats(w http.ResponseWriter, r *http.Request) {
	stats := h.wal.Stats()
	utils.OK(w, r, map[string]interface{}{
		"segment_count":     stats.SegmentCount,
		"active_segment":    stats.ActiveSegment,
		"active_size_bytes": stats.ActiveSizeBytes,
		"total_entries":     stats.TotalEntries,
	})
}

// WALRotate handles POST /admin/wal/rotate.
//
// POST /admin/wal/rotate
// Auth: admin only
//
// Responses:
//   200 OK       — rotation complete, returns new active segment index
//   500 Internal — INTERNAL_ERROR: rotation failed
func (h *Handler) WALRotate(w http.ResponseWriter, r *http.Request) {
	if err := h.wal.Rotate(); err != nil {
		logger.Error("WAL rotation failed", zap.Error(err))
		utils.InternalError(w, r, "WAL rotation failed")
		return
	}
	stats := h.wal.Stats()
	utils.OK(w, r, map[string]interface{}{
		"message":        "WAL segment rotated",
		"active_segment": stats.ActiveSegment,
	})
}

// ColdStats handles GET /admin/cold/stats.
//
// GET /admin/cold/stats
// Auth: admin only
//
// Responses:
//   200 OK — cold tier stats
func (h *Handler) ColdStats(w http.ResponseWriter, r *http.Request) {
	if h.cold == nil {
		utils.InternalError(w, r, "cold tier not available")
		return
	}
	stats := h.cold.Stats()
	utils.OK(w, r, map[string]interface{}{
		"parquet_files":       stats.TotalParquetFiles,
		"records_moved":       stats.TotalRecordsMoved,
		"last_flush_at":       stats.LastFlushAt,
		"last_flush_duration": stats.LastFlushDuration.String(),
	})
}

// SchemaList handles GET /admin/schema.
//
// GET /admin/schema
// Auth: admin only
//
// Responses:
//   200 OK — map of data type → schema
func (h *Handler) SchemaList(w http.ResponseWriter, r *http.Request) {
	types := []string{"logs", "metrics", "json", "kv"}
	result := make(map[string]interface{}, len(types))
	for _, dt := range types {
		schema, err := ingestion.LoadSchema(h.hot, dt)
		if err != nil {
			utils.InternalError(w, r, "failed to load schema for "+dt)
			return
		}
		result[dt] = schema
	}
	utils.OK(w, r, result)
}

// SchemaDelete handles DELETE /admin/schema/{type}.
//
// DELETE /admin/schema/{type}
// Auth: admin only
//
// Path param: type — one of: logs, metrics, json, kv
//
// Responses:
//   200 OK          — schema reset
//   400 Bad Request — VALIDATION_FAILED: unknown data type
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) SchemaDelete(w http.ResponseWriter, r *http.Request) {
	dataType := chi.URLParam(r, "type")
	valid := map[string]bool{"logs": true, "metrics": true, "json": true, "kv": true}
	if !valid[dataType] {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"type must be one of: logs, metrics, json, kv")
		return
	}
	if err := ingestion.DeleteSchema(h.hot, dataType); err != nil {
		utils.InternalError(w, r, "failed to delete schema")
		return
	}
	utils.OK(w, r, map[string]interface{}{
		"message":   "schema reset",
		"data_type": dataType,
	})
}
