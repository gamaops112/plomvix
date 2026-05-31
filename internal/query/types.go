package query

// FilterOp represents a comparison operator in a filter expression.
type FilterOp string

const (
	FilterOpEq  FilterOp = "="
	FilterOpNeq FilterOp = "!="
	FilterOpGt  FilterOp = ">"
	FilterOpLt  FilterOp = "<"
	FilterOpGte FilterOp = ">="
	FilterOpLte FilterOp = "<="
)

// FilterCondition is a single field comparison: field op value.
type FilterCondition struct {
	Field string
	Op    FilterOp
	Value string
}

// QueryParams holds the parsed parameters for a time-range query.
type QueryParams struct {
	FromNs     int64             // start timestamp, Unix nanoseconds (0 = beginning of time)
	ToNs       int64             // end timestamp, Unix nanoseconds (0 = now)
	Filters    []FilterCondition // AND-combined filter conditions
	Limit      int               // max records to return (default 100, max 10000)
	Offset     int               // records to skip before returning
	MetricName string            // optional: metrics CF only — filter by metric name
}

// QueryResult is the response envelope for all query endpoints.
type QueryResult struct {
	Records  []map[string]interface{} `json:"records"`
	Count    int                      `json:"count"`    // number of records in this page
	Total    int                      `json:"total"`    // total matching records before pagination
	Limit    int                      `json:"limit"`
	Offset   int                      `json:"offset"`
	QueryMs  int64                    `json:"query_ms"` // query execution time in milliseconds
	DataType string                   `json:"data_type"`
}

// DefaultLimit is the default number of records returned per query.
const DefaultLimit = 100

// MaxLimit is the maximum number of records that can be requested in a single query.
const MaxLimit = 10000
