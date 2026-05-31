package ingestion

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/storage/hot"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
	"github.com/plomvix/plomvix/pkg/utils"
)

// Handler handles all ingestion HTTP endpoints.
type Handler struct {
	hot *hot.Manager
	wal *walstore.Manager
}

// NewHandler creates a new ingestion Handler.
func NewHandler(h *hot.Manager, w *walstore.Manager) *Handler {
	return &Handler{hot: h, wal: w}
}

// IngestLogs handles POST /ingest/logs.
//
// POST /ingest/logs
// Auth: JWT or API key
//
// Request body: {"records": [{...}]}
//
// Responses:
//
//	201 Created      — ingested N records
//	400 Bad Request  — VALIDATION_FAILED: missing or empty records
//	500 Internal     — INTERNAL_ERROR: WAL or hot tier write failed
func (h *Handler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest[LogRecord]
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}
	if len(req.Records) == 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"records array is required and must not be empty")
		return
	}

	count := 0
	var rawRecords []map[string]interface{}

	for _, rec := range req.Records {
		if rec.Timestamp == 0 {
			rec.Timestamp = time.Now().UnixNano()
		}

		payload, err := json.Marshal(rec)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}

		// Write to WAL first — durability guarantee
		if _, err := h.wal.Write(walstore.DataTypeLog, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}

		// Write to hot tier
		if err := h.hot.WriteLog(rec.Timestamp, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}

		count++

		// Collect for schema inference
		var raw map[string]interface{}
		if err := json.Unmarshal(payload, &raw); err == nil {
			rawRecords = append(rawRecords, raw)
		}
	}

	// Update schema — non-fatal if it fails
	if err := UpdateSchema(h.hot, "logs", rawRecords); err != nil {
		logger.Warn("schema update failed", zap.String("data_type", "logs"), zap.Error(err))
	}

	utils.Created(w, r, IngestResponse{
		Ingested:  count,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

// IngestMetrics handles POST /ingest/metrics.
//
// POST /ingest/metrics
// Auth: JWT or API key
//
// Request body: {"records": [{...}]}
//
// Responses:
//
//	201 Created     — ingested N records
//	400 Bad Request — VALIDATION_FAILED
//	500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestMetrics(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest[MetricRecord]
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}
	if len(req.Records) == 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"records array is required and must not be empty")
		return
	}

	count := 0
	var rawRecords []map[string]interface{}

	for _, rec := range req.Records {
		if rec.Name == "" {
			utils.BadRequest(w, r, utils.CodeValidationFailed,
				"each metric record must have a non-empty name field")
			return
		}
		if rec.Timestamp == 0 {
			rec.Timestamp = time.Now().UnixNano()
		}

		payload, err := json.Marshal(rec)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}

		if _, err := h.wal.Write(walstore.DataTypeMetric, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}

		if err := h.hot.WriteMetric(rec.Timestamp, rec.Name, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}

		count++

		var raw map[string]interface{}
		if err := json.Unmarshal(payload, &raw); err == nil {
			rawRecords = append(rawRecords, raw)
		}
	}

	if err := UpdateSchema(h.hot, "metrics", rawRecords); err != nil {
		logger.Warn("schema update failed", zap.String("data_type", "metrics"), zap.Error(err))
	}

	utils.Created(w, r, IngestResponse{
		Ingested:  count,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

// IngestJSON handles POST /ingest/json.
//
// POST /ingest/json
// Auth: JWT or API key
//
// Responses:
//
//	201 Created     — ingested N records
//	400 Bad Request — VALIDATION_FAILED
//	500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestJSON(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest[JSONRecord]
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}
	if len(req.Records) == 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"records array is required and must not be empty")
		return
	}

	count := 0
	var rawRecords []map[string]interface{}

	for _, rec := range req.Records {
		if rec.Data == nil {
			utils.BadRequest(w, r, utils.CodeValidationFailed,
				"each json record must have a non-null data field")
			return
		}
		if rec.Timestamp == 0 {
			rec.Timestamp = time.Now().UnixNano()
		}

		payload, err := json.Marshal(rec)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}

		if _, err := h.wal.Write(walstore.DataTypeJSON, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}

		if err := h.hot.WriteJSON(rec.Timestamp, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}

		count++
		rawRecords = append(rawRecords, rec.Data)
	}

	if err := UpdateSchema(h.hot, "json", rawRecords); err != nil {
		logger.Warn("schema update failed", zap.String("data_type", "json"), zap.Error(err))
	}

	utils.Created(w, r, IngestResponse{
		Ingested:  count,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

// IngestKV handles POST /ingest/kv.
//
// POST /ingest/kv
// Auth: JWT or API key
//
// Responses:
//
//	201 Created     — ingested N records
//	400 Bad Request — VALIDATION_FAILED
//	500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestKV(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest[KVRecord]
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}
	if len(req.Records) == 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"records array is required and must not be empty")
		return
	}

	count := 0

	for _, rec := range req.Records {
		if rec.Key == "" {
			utils.BadRequest(w, r, utils.CodeValidationFailed,
				"each kv record must have a non-empty key field")
			return
		}

		payload, err := json.Marshal(rec)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}

		if _, err := h.wal.Write(walstore.DataTypeKV, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}

		if err := h.hot.WriteKV(rec.Key, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}

		count++
	}

	// KV has no schema inference — keys are user-defined strings
	utils.Created(w, r, IngestResponse{
		Ingested:  count,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}
