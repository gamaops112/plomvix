package query

import (
	"encoding/json"
	"time"

	"github.com/plomvix/plomvix/internal/ingestion"
	"github.com/plomvix/plomvix/internal/storage/hot"
)

// DecodePayload unmarshals a raw JSON byte slice into a map.
// Returns nil if decoding fails — callers must check for nil.
func DecodePayload(raw []byte) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// Engine executes queries against the hot tier.
type Engine struct {
	store *hot.Manager
}

// NewEngine creates a new query Engine.
func NewEngine(store *hot.Manager) *Engine {
	return &Engine{store: store}
}

// QueryLogs scans the logs column family and returns matching records.
func (e *Engine) QueryLogs(params *QueryParams) (*QueryResult, error) {
	return e.queryTimeSeries(hot.CFLogs, "logs", params)
}

// QueryJSON scans the json column family and returns matching records.
func (e *Engine) QueryJSON(params *QueryParams) (*QueryResult, error) {
	return e.queryTimeSeries(hot.CFJSON, "json", params)
}

// QueryMetrics scans the metrics column family and returns matching records.
// If params.MetricName is set (non-empty), only records with that metric name are returned.
func (e *Engine) QueryMetrics(params *QueryParams) (*QueryResult, error) {
	return e.queryTimeSeries(hot.CFMetrics, "metrics", params)
}

// QueryKV retrieves a single key-value record by key.
// Returns a QueryResult with 0 or 1 records.
func (e *Engine) QueryKV(key string) (*QueryResult, error) {
	start := time.Now()

	raw, err := e.store.GetKV(key)
	if err != nil {
		return nil, err
	}

	result := &QueryResult{
		DataType: "kv",
		Limit:    1,
		Offset:   0,
	}

	if raw == nil {
		result.Records = []map[string]interface{}{}
		result.QueryMs = time.Since(start).Milliseconds()
		return result, nil
	}

	record := DecodePayload(raw)
	if record == nil {
		result.Records = []map[string]interface{}{}
	} else {
		result.Records = []map[string]interface{}{record}
		result.Count = 1
		result.Total = 1
	}
	result.QueryMs = time.Since(start).Milliseconds()
	return result, nil
}

// QuerySchema returns the inferred schema for a data type.
// dataType must be one of: "logs", "metrics", "json", "kv".
func (e *Engine) QuerySchema(dataType string) (*ingestion.Schema, error) {
	return ingestion.LoadSchema(e.store, dataType)
}

// queryTimeSeries is the shared implementation for logs, json, and metrics queries.
func (e *Engine) queryTimeSeries(cf, dataType string, params *QueryParams) (*QueryResult, error) {
	start := time.Now()

	// Collect all matching records
	var all []map[string]interface{}

	err := e.store.ScanCF(cf, params.FromNs, params.ToNs, func(raw []byte) bool {
		record := DecodePayload(raw)
		if record == nil {
			return true // skip unparseable records, continue scanning
		}
		if ApplyFilters(record, params.Filters) {
			all = append(all, record)
		}
		return true // always continue scanning — collect all matching
	})
	if err != nil {
		return nil, err
	}

	total := len(all)

	// Apply pagination
	start2 := params.Offset
	if start2 > total {
		start2 = total
	}
	end := start2 + params.Limit
	if end > total {
		end = total
	}
	page := all[start2:end]
	if page == nil {
		page = []map[string]interface{}{}
	}

	return &QueryResult{
		Records:  page,
		Count:    len(page),
		Total:    total,
		Limit:    params.Limit,
		Offset:   params.Offset,
		QueryMs:  time.Since(start).Milliseconds(),
		DataType: dataType,
	}, nil
}
