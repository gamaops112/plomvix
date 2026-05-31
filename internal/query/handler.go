package query

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/plomvix/plomvix/pkg/utils"
)

// Handler handles all query HTTP endpoints.
type Handler struct {
	engine *Engine
}

// NewHandler creates a new query Handler.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// QueryLogs handles GET /query/logs.
//
// GET /query/logs
// Auth: JWT or API key
//
// Query params:
//   from:   Unix nanoseconds start (default: 0)
//   to:     Unix nanoseconds end (default: now)
//   filter: filter expression (e.g. "level=info AND value>50")
//   limit:  max records (default 100, max 10000)
//   offset: skip N records (default 0)
//
// Responses:
//   200 OK          — query results
//   400 Bad Request — VALIDATION_FAILED: invalid params or filter
//   500 Internal    — INTERNAL_ERROR: scan failed
func (h *Handler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	params, err := ParseQueryParams(r)
	if err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
		return
	}
	result, err := h.engine.QueryLogs(params)
	if err != nil {
		utils.InternalError(w, r, "query failed")
		return
	}
	utils.OK(w, r, result)
}

// QueryMetrics handles GET /query/metrics.
//
// GET /query/metrics
// Auth: JWT or API key
//
// Query params:
//   from, to, filter, limit, offset (same as /query/logs)
//   name: optional metric name filter
//
// Responses:
//   200 OK          — query results
//   400 Bad Request — VALIDATION_FAILED
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QueryMetrics(w http.ResponseWriter, r *http.Request) {
	params, err := ParseQueryParams(r)
	if err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
		return
	}
	// Metric name filter — add as extra filter condition if provided
	if name := r.URL.Query().Get("name"); name != "" {
		params.MetricName = name
		params.Filters = append(params.Filters, FilterCondition{
			Field: "name",
			Op:    FilterOpEq,
			Value: name,
		})
	}
	result, err := h.engine.QueryMetrics(params)
	if err != nil {
		utils.InternalError(w, r, "query failed")
		return
	}
	utils.OK(w, r, result)
}

// QueryJSON handles GET /query/json.
//
// GET /query/json
// Auth: JWT or API key
//
// Query params: from, to, filter, limit, offset
//
// Responses:
//   200 OK / 400 Bad Request / 500 Internal
func (h *Handler) QueryJSON(w http.ResponseWriter, r *http.Request) {
	params, err := ParseQueryParams(r)
	if err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
		return
	}
	result, err := h.engine.QueryJSON(params)
	if err != nil {
		utils.InternalError(w, r, "query failed")
		return
	}
	utils.OK(w, r, result)
}

// QueryKV handles GET /query/kv/{key}.
//
// GET /query/kv/{key}
// Auth: JWT or API key
//
// Path param: key — the KV key to look up
//
// Responses:
//   200 OK          — record found (count=1) or not found (count=0, records=[])
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QueryKV(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	result, err := h.engine.QueryKV(key)
	if err != nil {
		utils.InternalError(w, r, "query failed")
		return
	}
	utils.OK(w, r, result)
}

// QuerySchema handles GET /query/schema/{type}.
//
// GET /query/schema/{type}
// Auth: JWT or API key
//
// Path param: type — one of: logs, metrics, json, kv
//
// Responses:
//   200 OK          — schema for the data type
//   400 Bad Request — VALIDATION_FAILED: unknown data type
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QuerySchema(w http.ResponseWriter, r *http.Request) {
	dataType := chi.URLParam(r, "type")
	valid := map[string]bool{"logs": true, "metrics": true, "json": true, "kv": true}
	if !valid[dataType] {
		utils.BadRequest(w, r, utils.CodeValidationFailed,
			"type must be one of: logs, metrics, json, kv")
		return
	}
	schema, err := h.engine.QuerySchema(dataType)
	if err != nil {
		utils.InternalError(w, r, "failed to load schema")
		return
	}
	utils.OK(w, r, schema)
}
