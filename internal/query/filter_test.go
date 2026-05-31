package query

import "testing"

func TestParseFilterEmpty(t *testing.T) {
	conditions, err := ParseFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conditions) != 0 {
		t.Errorf("expected 0 conditions, got %d", len(conditions))
	}
}

func TestParseFilterSingle(t *testing.T) {
	conditions, err := ParseFilter("level=info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Field != "level" {
		t.Errorf("field = %q, want %q", conditions[0].Field, "level")
	}
	if conditions[0].Op != FilterOpEq {
		t.Errorf("op = %q, want =", conditions[0].Op)
	}
	if conditions[0].Value != "info" {
		t.Errorf("value = %q, want %q", conditions[0].Value, "info")
	}
}

func TestParseFilterMultiple(t *testing.T) {
	conditions, err := ParseFilter("level=info AND value>50")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}
	if conditions[1].Op != FilterOpGt {
		t.Errorf("second op = %q, want >", conditions[1].Op)
	}
}

func TestParseFilterAllOps(t *testing.T) {
	tests := []struct {
		input string
		op    FilterOp
	}{
		{"x=1", FilterOpEq},
		{"x!=1", FilterOpNeq},
		{"x>1", FilterOpGt},
		{"x<1", FilterOpLt},
		{"x>=1", FilterOpGte},
		{"x<=1", FilterOpLte},
	}
	for _, tt := range tests {
		conds, err := ParseFilter(tt.input)
		if err != nil {
			t.Errorf("ParseFilter(%q) error: %v", tt.input, err)
			continue
		}
		if len(conds) != 1 || conds[0].Op != tt.op {
			t.Errorf("ParseFilter(%q) op = %q, want %q", tt.input, conds[0].Op, tt.op)
		}
	}
}

func TestParseFilterInvalid(t *testing.T) {
	_, err := ParseFilter("noop")
	if err == nil {
		t.Error("expected error for filter with no operator, got nil")
	}
}

func TestApplyFiltersEmpty(t *testing.T) {
	record := map[string]interface{}{"level": "info"}
	if !ApplyFilters(record, nil) {
		t.Error("ApplyFilters with nil conditions should return true")
	}
}

func TestApplyFiltersMatch(t *testing.T) {
	record := map[string]interface{}{"level": "info", "value": float64(75)}
	conditions := []FilterCondition{
		{Field: "level", Op: FilterOpEq, Value: "info"},
		{Field: "value", Op: FilterOpGt, Value: "50"},
	}
	if !ApplyFilters(record, conditions) {
		t.Error("expected record to match all conditions")
	}
}

func TestApplyFiltersNoMatch(t *testing.T) {
	record := map[string]interface{}{"level": "debug", "value": float64(10)}
	conditions := []FilterCondition{
		{Field: "level", Op: FilterOpEq, Value: "info"}, // does not match
	}
	if ApplyFilters(record, conditions) {
		t.Error("expected record to NOT match")
	}
}

func TestApplyFiltersMissingField(t *testing.T) {
	record := map[string]interface{}{"level": "info"}
	conditions := []FilterCondition{
		{Field: "nonexistent", Op: FilterOpEq, Value: "anything"},
	}
	if ApplyFilters(record, conditions) {
		t.Error("expected false for missing field")
	}
}

func TestApplyFiltersNeq(t *testing.T) {
	record := map[string]interface{}{"level": "warn"}
	conditions := []FilterCondition{
		{Field: "level", Op: FilterOpNeq, Value: "info"},
	}
	if !ApplyFilters(record, conditions) {
		t.Error("warn != info should be true")
	}
}

func TestNumericCompareGte(t *testing.T) {
	record := map[string]interface{}{"value": float64(50)}
	conds := []FilterCondition{{Field: "value", Op: FilterOpGte, Value: "50"}}
	if !ApplyFilters(record, conds) {
		t.Error("50 >= 50 should be true")
	}
}

func TestNumericCompareLte(t *testing.T) {
	record := map[string]interface{}{"value": float64(49)}
	conds := []FilterCondition{{Field: "value", Op: FilterOpLte, Value: "50"}}
	if !ApplyFilters(record, conds) {
		t.Error("49 <= 50 should be true")
	}
}
