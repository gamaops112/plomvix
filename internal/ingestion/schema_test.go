package ingestion

import (
	"testing"
	"time"
)

func TestInferFieldType(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected FieldType
	}{
		{nil, FieldTypeNull},
		{true, FieldTypeBool},
		{false, FieldTypeBool},
		{float64(42), FieldTypeInt64},     // whole number → int64
		{float64(3.14), FieldTypeFloat64}, // decimal → float64
		{"hello", FieldTypeString},
		{map[string]interface{}{"a": 1}, FieldTypeObject},
		{[]interface{}{1, 2}, FieldTypeArray},
	}
	for _, tt := range tests {
		got := InferFieldType(tt.input)
		if got != tt.expected {
			t.Errorf("InferFieldType(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestInferSchema(t *testing.T) {
	record := map[string]interface{}{
		"level":   "info",
		"count":   float64(5),
		"ratio":   float64(0.5),
		"active":  true,
		"nothing": nil,
	}
	schema := InferSchema(record)

	if schema["level"] != FieldTypeString {
		t.Errorf("level: got %q, want %q", schema["level"], FieldTypeString)
	}
	if schema["count"] != FieldTypeInt64 {
		t.Errorf("count: got %q, want %q", schema["count"], FieldTypeInt64)
	}
	if schema["ratio"] != FieldTypeFloat64 {
		t.Errorf("ratio: got %q, want %q", schema["ratio"], FieldTypeFloat64)
	}
	if schema["active"] != FieldTypeBool {
		t.Errorf("active: got %q, want %q", schema["active"], FieldTypeBool)
	}
	if schema["nothing"] != FieldTypeNull {
		t.Errorf("nothing: got %q, want %q", schema["nothing"], FieldTypeNull)
	}
}

func TestMergeSchemaNewFields(t *testing.T) {
	s := &Schema{
		DataType:  "logs",
		Fields:    map[string]FieldType{},
		UpdatedAt: time.Now(),
	}
	newFields := map[string]FieldType{
		"level": FieldTypeString,
		"count": FieldTypeInt64,
	}
	MergeSchema(s, newFields, 1)

	if s.Fields["level"] != FieldTypeString {
		t.Errorf("level: got %q, want string", s.Fields["level"])
	}
	if s.Fields["count"] != FieldTypeInt64 {
		t.Errorf("count: got %q, want int64", s.Fields["count"])
	}
	if s.RecordCount != 1 {
		t.Errorf("RecordCount = %d, want 1", s.RecordCount)
	}
}

func TestMergeSchemaConflict(t *testing.T) {
	s := &Schema{
		DataType: "logs",
		Fields:   map[string]FieldType{"level": FieldTypeString},
	}
	// Same field, different type → should become mixed
	MergeSchema(s, map[string]FieldType{"level": FieldTypeInt64}, 1)
	if s.Fields["level"] != FieldTypeMixed {
		t.Errorf("level after conflict: got %q, want mixed", s.Fields["level"])
	}
}

func TestMergeSchemaSameType(t *testing.T) {
	s := &Schema{
		DataType: "logs",
		Fields:   map[string]FieldType{"level": FieldTypeString},
	}
	// Same type twice — should stay string, not become mixed
	MergeSchema(s, map[string]FieldType{"level": FieldTypeString}, 1)
	if s.Fields["level"] != FieldTypeString {
		t.Errorf("level after same type: got %q, want string", s.Fields["level"])
	}
}
