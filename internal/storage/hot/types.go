package hot

import (
	"encoding/binary"

	"github.com/google/uuid"
)

const (
	CFLogs    = "logs"
	CFMetrics = "metrics"
	CFJSON    = "json"
	CFKV      = "kv"
)

func AllColumnFamilies() []string {
	return []string{"default", CFLogs, CFMetrics, CFJSON, CFKV}
}

func BuildTimeSeriesKey(timestampNs int64) []byte {
	key := make([]byte, 8+16)
	binary.BigEndian.PutUint64(key[:8], uint64(timestampNs))
	id := uuid.New()
	copy(key[8:], id[:])
	return key
}

func BuildMetricKey(timestampNs int64, metricName string) []byte {
	nameBytes := []byte(metricName)
	key := make([]byte, 8+len(nameBytes)+1+16)
	binary.BigEndian.PutUint64(key[:8], uint64(timestampNs))
	copy(key[8:], nameBytes)
	key[8+len(nameBytes)] = 0x00
	id := uuid.New()
	copy(key[8+len(nameBytes)+1:], id[:])
	return key
}

func BuildKVKey(userKey string) []byte {
	return []byte(userKey)
}

func BuildRangeScanPrefix(timestampNs int64) []byte {
	prefix := make([]byte, 8)
	binary.BigEndian.PutUint64(prefix, uint64(timestampNs))
	return prefix
}
