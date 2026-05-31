package ingestion

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/logger"
	"github.com/plomvix/plomvix/internal/parser"
	"github.com/plomvix/plomvix/internal/storage/hot"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
	"github.com/plomvix/plomvix/pkg/utils"
)

// Handler handles all ingestion HTTP endpoints.
type Handler struct {
	hot     *hot.Manager
	wal     *walstore.Manager
	parsers *parser.Registry
}

// NewHandler creates a new ingestion Handler.
func NewHandler(h *hot.Manager, w *walstore.Manager) *Handler {
	return &Handler{hot: h, wal: w, parsers: parser.NewRegistry()}
}

func (h *Handler) parseRequest(w http.ResponseWriter, r *http.Request, allowed map[string]bool) ([]map[string]interface{}, bool) {
	ct := parser.NormalizeContentType(r.Header.Get("Content-Type"))
	if ct == "" {
		ct = parser.ContentTypeJSON
	}
	if !allowed[ct] {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			fmt.Sprintf("unsupported content type for this endpoint: %s", ct))
		return nil, false
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		utils.InternalError(w, r, "failed to read request body")
		return nil, false
	}

	records, err := h.parsers.Get(ct).Parse(data)
	if err != nil {
		if errors.Is(err, parser.ErrEmptyInput) {
			utils.BadRequest(w, r, utils.CodeValidationFailed,
				"request body is empty or contains no parseable records")
			return nil, false
		}
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			fmt.Sprintf("failed to parse request body: %v", err))
		return nil, false
	}
	return records, true
}

func ensureTimestampNs(record map[string]interface{}) int64 {
	if v, ok := record["timestamp_ns"]; ok {
		if ts := numericToInt64(v); ts > 0 {
			record["timestamp"] = float64(ts)
			return ts
		}
	}
	if v, ok := record["timestamp"]; ok {
		if ts := numericToInt64(v); ts > 0 {
			record["timestamp"] = float64(ts)
			return ts
		}
	}
	ts := time.Now().UnixNano()
	record["timestamp"] = float64(ts)
	return ts
}

func numericToInt64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
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
	records, ok := h.parseRequest(w, r, map[string]bool{
		parser.ContentTypeJSON:   true,
		parser.ContentTypeCSV:    true,
		parser.ContentTypeLogfmt: true,
		parser.ContentTypeSyslog: true,
	})
	if !ok {
		return
	}

	count := 0
	var schemaRecords []map[string]interface{}
	for _, record := range records {
		tsNs := ensureTimestampNs(record)
		payload, err := json.Marshal(record)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}
		if _, err := h.wal.Write(walstore.DataTypeLog, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}
		if err := h.hot.WriteLog(tsNs, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}
		count++
		schemaRecords = append(schemaRecords, record)
	}
	if err := UpdateSchema(h.hot, "logs", schemaRecords); err != nil {
		logger.Warn("schema update failed", zap.String("data_type", "logs"), zap.Error(err))
	}
	utils.Created(w, r, IngestResponse{Ingested: count, RequestID: r.Header.Get("X-Request-ID")})
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
	records, ok := h.parseRequest(w, r, map[string]bool{
		parser.ContentTypeJSON: true,
		parser.ContentTypeCSV:  true,
	})
	if !ok {
		return
	}

	count := 0
	var schemaRecords []map[string]interface{}
	for _, record := range records {
		tsNs := ensureTimestampNs(record)
		payload, err := json.Marshal(record)
		if err != nil {
			utils.InternalError(w, r, "failed to serialize record")
			return
		}
		if _, err := h.wal.Write(walstore.DataTypeJSON, payload); err != nil {
			logger.Error("WAL write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to WAL")
			return
		}
		if err := h.hot.WriteJSON(tsNs, payload); err != nil {
			logger.Error("hot tier write failed", zap.Error(err))
			utils.InternalError(w, r, "failed to write to hot tier")
			return
		}
		count++
		schemaRecords = append(schemaRecords, record)
	}
	if err := UpdateSchema(h.hot, "json", schemaRecords); err != nil {
		logger.Warn("schema update failed", zap.String("data_type", "json"), zap.Error(err))
	}
	utils.Created(w, r, IngestResponse{Ingested: count, RequestID: r.Header.Get("X-Request-ID")})
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
