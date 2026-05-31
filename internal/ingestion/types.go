package ingestion

import "time"

// FieldType represents the inferred type of a JSON field.
type FieldType string

const (
	FieldTypeBool    FieldType = "bool"
	FieldTypeInt64   FieldType = "int64"
	FieldTypeFloat64 FieldType = "float64"
	FieldTypeString  FieldType = "string"
	FieldTypeNull    FieldType = "null"
	FieldTypeObject  FieldType = "object"
	FieldTypeArray   FieldType = "array"
	FieldTypeMixed   FieldType = "mixed" // field seen with conflicting types
)

// Schema represents the inferred schema for a data type.
type Schema struct {
	DataType    string               `json:"data_type"`
	Fields      map[string]FieldType `json:"fields"`
	UpdatedAt   time.Time            `json:"updated_at"`
	RecordCount int64                `json:"record_count"`
}

// LogRecord is the expected shape of an ingested log entry.
type LogRecord struct {
	Timestamp int64                  `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// MetricRecord is the expected shape of an ingested metric.
type MetricRecord struct {
	Timestamp int64             `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
	Name      string            `json:"name"`      // metric name, required
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// JSONRecord is a free-form JSON document.
// The entire record is stored as-is; schema is inferred from top-level fields.
type JSONRecord struct {
	Timestamp int64                  `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
	Data      map[string]interface{} `json:"data"`
}

// KVRecord is a key-value pair.
type KVRecord struct {
	Key   string `json:"key"`   // required, non-empty
	Value string `json:"value"` // stored as raw string
}

// IngestRequest wraps a single record or batch for all ingest endpoints.
// Either Records (batch) or a single record field is populated.
type IngestRequest[T any] struct {
	Records []T `json:"records"` // batch — 1 or more records
}

// IngestResponse is returned on successful ingestion.
type IngestResponse struct {
	Ingested  int    `json:"ingested"`
	RequestID string `json:"request_id"`
}
