package query

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseFilter parses a filter string into a slice of FilterConditions.
// Returns an error if the expression is malformed.
// Empty string returns nil slice (no filter — all records match).
//
// Format: "field=value AND field2>value2"
// Conditions are split on " AND " (space-AND-space, case-insensitive).
func ParseFilter(expr string) ([]FilterCondition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}

	// Split on " AND " — case-insensitive
	parts := splitAND(expr)
	conditions := make([]FilterCondition, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cond, err := parseCondition(part)
		if err != nil {
			return nil, fmt.Errorf("invalid filter condition %q: %w", part, err)
		}
		conditions = append(conditions, cond)
	}
	return conditions, nil
}

// splitAND splits a filter expression on " AND " (case-insensitive).
func splitAND(expr string) []string {
	upper := strings.ToUpper(expr)
	var parts []string
	for {
		idx := strings.Index(upper, " AND ")
		if idx == -1 {
			parts = append(parts, expr)
			break
		}
		parts = append(parts, expr[:idx])
		expr = expr[idx+5:]
		upper = upper[idx+5:]
	}
	return parts
}

// parseCondition parses a single "field op value" condition.
// Operators are tried longest-first to avoid matching ">" before ">=".
func parseCondition(s string) (FilterCondition, error) {
	ops := []FilterOp{FilterOpGte, FilterOpLte, FilterOpNeq, FilterOpEq, FilterOpGt, FilterOpLt}
	for _, op := range ops {
		idx := strings.Index(s, string(op))
		if idx > 0 {
			field := strings.TrimSpace(s[:idx])
			value := strings.TrimSpace(s[idx+len(op):])
			if field == "" || value == "" {
				return FilterCondition{}, fmt.Errorf("field and value must not be empty")
			}
			return FilterCondition{Field: field, Op: op, Value: value}, nil
		}
	}
	return FilterCondition{}, fmt.Errorf("no valid operator found in %q", s)
}

// ApplyFilters returns true if the record matches ALL filter conditions.
// If the record is nil or filters is nil/empty, returns true.
func ApplyFilters(record map[string]interface{}, filters []FilterCondition) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if !matchCondition(record, f) {
			return false
		}
	}
	return true
}

// matchCondition checks a single filter condition against a record.
func matchCondition(record map[string]interface{}, f FilterCondition) bool {
	val, ok := record[f.Field]
	if !ok {
		return false // field not present — exclude record
	}

	recordStr := fmt.Sprintf("%v", val)

	switch f.Op {
	case FilterOpEq:
		return recordStr == f.Value
	case FilterOpNeq:
		return recordStr != f.Value
	case FilterOpGt, FilterOpLt, FilterOpGte, FilterOpLte:
		return numericCompare(val, f.Op, f.Value)
	}
	return false
}

// numericCompare performs a numeric comparison between a record value and a filter value.
// Falls back to string comparison if either value is non-numeric.
func numericCompare(recordVal interface{}, op FilterOp, filterVal string) bool {
	var recordFloat float64

	switch v := recordVal.(type) {
	case float64:
		recordFloat = v
	case int64:
		recordFloat = float64(v)
	default:
		// Try string-to-float conversion
		f, err := strconv.ParseFloat(fmt.Sprintf("%v", recordVal), 64)
		if err != nil {
			return false
		}
		recordFloat = f
	}

	filterFloat, err := strconv.ParseFloat(filterVal, 64)
	if err != nil {
		return false
	}

	switch op {
	case FilterOpGt:
		return recordFloat > filterFloat
	case FilterOpLt:
		return recordFloat < filterFloat
	case FilterOpGte:
		return recordFloat >= filterFloat
	case FilterOpLte:
		return recordFloat <= filterFloat
	}
	return false
}
