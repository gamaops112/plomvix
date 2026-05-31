package hot

import (
	"encoding/binary"
	"fmt"

	"github.com/plomvix/plomvix/internal/config"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

type HotStats struct {
	TotalWrites int64
	DataDir     string
}

type Manager struct {
	store *Store
}

func Open(dataDir string, cfg *config.Config) (*Manager, error) {
	store, err := openRocksDB(dataDir, cfg)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store}, nil
}

func (m *Manager) WriteLog(timestampNs int64, payload []byte) error {
	key := BuildTimeSeriesKey(timestampNs)
	return m.store.Put(CFLogs, key, payload)
}

func (m *Manager) WriteMetric(timestampNs int64, metricName string, payload []byte) error {
	key := BuildMetricKey(timestampNs, metricName)
	return m.store.Put(CFMetrics, key, payload)
}

func (m *Manager) WriteJSON(timestampNs int64, payload []byte) error {
	key := BuildTimeSeriesKey(timestampNs)
	return m.store.Put(CFJSON, key, payload)
}

func (m *Manager) WriteKV(userKey string, payload []byte) error {
	key := BuildKVKey(userKey)
	return m.store.Put(CFKV, key, payload)
}

func (m *Manager) ReplayWALEntry(entry *walstore.Entry) error {
	ts := entry.Timestamp
	payload := entry.Payload

	switch entry.DataType {
	case walstore.DataTypeLog:
		return m.WriteLog(ts, payload)
	case walstore.DataTypeMetric:
		return m.WriteMetric(ts, "unknown", payload)
	case walstore.DataTypeJSON:
		return m.WriteJSON(ts, payload)
	case walstore.DataTypeKV:
		return m.WriteKV(fmt.Sprintf("wal_replay_%d", ts), payload)
	default:
		return fmt.Errorf("unknown WAL data type: %d", entry.DataType)
	}
}

func (m *Manager) ReplayWAL(entries []*walstore.Entry) (int, error) {
	count := 0
	for _, entry := range entries {
		if err := m.ReplayWALEntry(entry); err != nil {
			return count, fmt.Errorf("WAL replay failed at SeqID %d: %w", entry.SeqID, err)
		}
		count++
	}
	return count, nil
}

func (m *Manager) ScanLogs(fromNs, toNs int64) ([][]byte, error) {
	return m.scanTimeRange(CFLogs, fromNs, toNs)
}

func (m *Manager) ScanJSON(fromNs, toNs int64) ([][]byte, error) {
	return m.scanTimeRange(CFJSON, fromNs, toNs)
}

func (m *Manager) GetKV(userKey string) ([]byte, error) {
	return m.store.Get(CFKV, BuildKVKey(userKey))
}

func (m *Manager) scanTimeRange(cf string, fromNs, toNs int64) ([][]byte, error) {
	var results [][]byte
	prefix := BuildRangeScanPrefix(fromNs)

	err := m.store.Scan(cf, prefix, func(key, value []byte) bool {
		if toNs > 0 && len(key) >= 8 {
			keyTs := int64(bigEndianUint64(key[:8]))
			if keyTs >= toNs {
				return false
			}
		}
		results = append(results, value)
		return true
	})
	return results, err
}

func (m *Manager) Stats() HotStats {
	return HotStats{
		TotalWrites: m.store.TotalWrites(),
		DataDir:     m.store.dataDir,
	}
}

func (m *Manager) Close() {
	m.store.Close()
}

func bigEndianUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
