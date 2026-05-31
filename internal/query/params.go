package query

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ParseQueryParams extracts and validates query parameters from an HTTP request.
// Handles: from, to, filter, limit, offset.
//
// from / to: Unix nanoseconds as int64 strings.
//            If from is 0 or absent, defaults to 0 (beginning of time).
//            If to is 0 or absent, defaults to time.Now().UnixNano().
//
// filter: filter expression string (see filter.go).
//
// limit: max records per page. Default DefaultLimit. Max MaxLimit.
//
// offset: records to skip. Default 0.
func ParseQueryParams(r *http.Request) (*QueryParams, error) {
	q := r.URL.Query()
	params := &QueryParams{
		Limit: DefaultLimit,
	}

	// from
	if s := q.Get("from"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid 'from' parameter: must be Unix nanoseconds int64")
		}
		params.FromNs = v
	}

	// to
	if s := q.Get("to"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid 'to' parameter: must be Unix nanoseconds int64")
		}
		params.ToNs = v
	} else {
		params.ToNs = time.Now().UnixNano()
	}

	if params.FromNs > 0 && params.ToNs > 0 && params.FromNs >= params.ToNs {
		return nil, fmt.Errorf("'from' must be less than 'to'")
	}

	// filter
	if s := q.Get("filter"); s != "" {
		filters, err := ParseFilter(s)
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		params.Filters = filters
	}

	// limit
	if s := q.Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("invalid 'limit' parameter: must be a positive integer")
		}
		if v > MaxLimit {
			v = MaxLimit
		}
		params.Limit = v
	}

	// offset
	if s := q.Get("offset"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("invalid 'offset' parameter: must be a non-negative integer")
		}
		params.Offset = v
	}

	return params, nil
}
