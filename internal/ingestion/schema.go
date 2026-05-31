package ingestion

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/plomvix/plomvix/internal/storage/hot"
)

// InferFieldType returns the FieldType for a single JSON value.
func InferFieldType(v interface{}) FieldType {
	if v == nil {
		return FieldTypeNull
	}
	switch v.(type) {
	case bool:
		return FieldTypeBool
	case float64:
		// JSON numbers decode as float64; check if it is a whole number
		f := v.(float64)
		if f == float64(int64(f)) {
			return FieldTypeInt64
		}
		return FieldTypeFloat64
	case string:
		return FieldTypeString
	case map[string]interface{}:
		return FieldTypeObject
	case []interface{}:
		return FieldTypeArray
	default:
		return FieldTypeString
	}
}

// InferSchema inspects a flat JSON object and returns a map of field → FieldType.
// Only top-level fields are inspected.
func InferSchema(record map[string]interface{}) map[string]FieldType {
	fields := make(map[string]FieldType, len(record))
	for k, v := range record {
		fields[k] = InferFieldType(v)
	}
	return fields
}

// MergeSchema merges newly inferred fields into an existing schema.
// New fields are added. Existing fields with a different type become FieldTypeMixed.
// RecordCount is incremented by delta.
func MergeSchema(s *Schema, newFields map[string]FieldType, delta int64) {
	for field, newType := range newFields {
		if currentType, ok := s.Fields[field]; ok {
			if currentType != newType {
				s.Fields[field] = FieldTypeMixed
			}
			// if same type, no change needed
		} else {
			s.Fields[field] = newType
		}
	}
	s.RecordCount += delta
	s.UpdatedAt = time.Now()
}

// schemaKey returns the RocksDB key for a data type's schema.
func schemaKey(dataType string) []byte {
	return []byte("schema:" + dataType)
}

// LoadSchema loads the current schema for a data type from RocksDB.
// Returns a new empty Schema if none exists yet.
func LoadSchema(store *hot.Manager, dataType string) (*Schema, error) {
	raw, err := store.GetMeta(schemaKey(dataType))
	if err != nil {
		return nil, fmt.Errorf("failed to load schema for %q: %w", dataType, err)
	}
	if raw == nil {
		return &Schema{
			DataType: dataType,
			Fields:   make(map[string]FieldType),
		}, nil
	}
	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema for %q: %w", dataType, err)
	}
	return &s, nil
}

// DeleteSchema removes the stored schema for a data type from the _meta CF.
// After deletion, schema inference starts fresh on the next ingest.
func DeleteSchema(store *hot.Manager, dataType string) error {
	return store.DeleteMeta(schemaKey(dataType))
}

// SaveSchema persists a schema to RocksDB.
func SaveSchema(store *hot.Manager, s *Schema) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal schema for %q: %w", s.DataType, err)
	}
	return store.PutMeta(schemaKey(s.DataType), raw)
}

// UpdateSchema loads, merges, and saves the schema for a data type in one call.
// Errors are non-fatal — schema update failure does not block ingestion.
func UpdateSchema(store *hot.Manager, dataType string, records []map[string]interface{}) error {
	s, err := LoadSchema(store, dataType)
	if err != nil {
		return err
	}
	for _, record := range records {
		inferred := InferSchema(record)
		MergeSchema(s, inferred, 1)
	}
	return SaveSchema(store, s)
}
