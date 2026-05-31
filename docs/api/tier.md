# Plomvix Tier API Reference

The tiering system automatically moves aged data from the hot tier (RocksDB)
to the cold tier (Parquet files) based on the `retention_days` config value.

**Tierable data types:** logs, metrics, json.
**KV is not tiered in Sprint 7** — KV keys have no timestamp prefix.
KV data always stays in the hot tier.

---

## POST /admin/tier/flush

Triggers an immediate cold tier flush outside the hourly background schedule.
Moves all eligible hot tier data (older than `retention_days`) to Parquet files.

**Auth:** Admin only

**Request body:** none

**Response 200:**
```json
{
  "status": "ok",
  "data": {
    "message": "tier flush complete",
    "records_moved": 1500,
    "parquet_files": 3,
    "last_flush_at": "2024-01-15T10:30:00Z",
    "flush_duration": "1.23s"
  },
  "request_id": "uuid"
}
```

**Response 500:** flush failed — check server logs for details.

**curl example:**
```bash
curl -X POST http://localhost:8080/admin/tier/flush \
  -H "Authorization: Bearer <token>"
```

---

## Health Endpoint — Cold Tier Stats

```json
{
  "status": "ok",
  "data": {
    "cold": {
      "parquet_files": 3,
      "records_moved": 1500,
      "last_flush_at": "2024-01-15T10:30:00Z"
    }
  }
}
```

| Field | Type | Description |
|---|---|---|
| `cold.parquet_files` | int | Total Parquet files currently on disk |
| `cold.records_moved` | int64 | Records moved to cold tier this process lifetime (resets on restart) |
| `cold.last_flush_at` | time | Timestamp of most recent flush |

---

## Configuration

```yaml
storage:
  retention_days: 30   # data older than this moves to cold tier
```

---

## Cold Tier Directory Structure

```
data/cold/
├── logs/YYYY-MM-DD/part-000001.parquet
├── metrics/YYYY-MM-DD/part-000001.parquet
└── json/YYYY-MM-DD/part-000001.parquet
```

Date partition is based on the **oldest record's timestamp** in each flush batch (UTC).
KV has no cold tier directory.

---

## Query Behaviour

GET /query/logs, GET /query/metrics, GET /query/json automatically search
both hot and cold tiers. Results are merged and sorted by timestamp ascending
before pagination.

GET /query/kv/{key} searches hot tier only — KV is not tiered in Sprint 7.

---

## Tiering Behaviour

- Background flush runs every hour automatically.
- Each flush moves all records older than `retention_days` from RocksDB to Parquet.
- Records are deleted from RocksDB after successful cold write.
- If any deletion fails, the flush returns an error immediately. Since deletion
  happens after the Parquet write and key-by-key, partial hot+cold state is possible
  on failure. Operators should retry the flush or run reconciliation before assuming
  duplicate-free query results.
- `records_moved` in health reflects process lifetime only — resets on restart.
