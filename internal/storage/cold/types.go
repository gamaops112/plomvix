package cold

import (
	"fmt"
	"path/filepath"
	"time"
)

// ParquetRow is the universal row schema for all Parquet files in Plomvix.
type ParquetRow struct {
	TimestampNs int64  `parquet:"timestamp_ns"`
	Payload     string `parquet:"payload"`
}

// Tierable data type constants. KV is intentionally excluded from Sprint 7 tiering.
const (
	DataTypeLogs    = "logs"
	DataTypeMetrics = "metrics"
	DataTypeJSON    = "json"
)

// TierableDataTypes returns the data types eligible for cold tier in Sprint 7.
// KV is excluded — KV keys have no timestamp prefix and require separate design.
func TierableDataTypes() []string {
	return []string{DataTypeLogs, DataTypeMetrics, DataTypeJSON}
}

// PartFileName returns the filename for a part file.
// Example: PartFileName(1) → "part-000001.parquet"
func PartFileName(index int) string {
	return fmt.Sprintf("part-%06d.parquet", index)
}

// DatePartitionDir returns the date-partitioned directory path for a data type and timestamp.
// Example: DatePartitionDir("logs", t) → "logs/2024-01-15"
func DatePartitionDir(dataType string, ts time.Time) string {
	return filepath.Join(dataType, ts.UTC().Format("2006-01-02"))
}

// TierStats holds statistics about the cold tier.
type TierStats struct {
	TotalParquetFiles int
	TotalRecordsMoved int64
	LastFlushAt       time.Time
	LastFlushDuration time.Duration
}
