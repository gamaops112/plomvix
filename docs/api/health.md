# Plomvix Health API

## GET /health

Returns the current health status of the Plomvix server.

**Auth:** None — public endpoint, no authentication required.

---

### Response — 200 OK (all checks pass)

```json
{
  "status": "ok",
  "data": {
    "version": "0.1.0",
    "env": "development",
    "uptime_seconds": 3600,
    "pid": 12345,
    "go_version": "go1.22.0",
    "os_arch": "linux/amd64",
    "wal": {
      "segment_count": 2,
      "active_segment": 2,
      "active_size_bytes": 1048576,
      "total_entries": 1500
    },
    "hot": {
      "total_writes": 0,
      "data_dir": "./data/hot"
    }
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Response — 503 Service Unavailable (checks failing)

```json
{
  "status": "error",
  "error": {
    "code": "HEALTH_CHECK_FAILED",
    "message": "One or more health checks failed",
    "details": [
      "data directory not writable: ./data/wal",
      "data directory not writable: ./data/hot"
    ]
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

### Data Fields

| Field | Type | Description |
|---|---|---|
| `version` | string | Binary version (injected at build time) |
| `env` | string | Environment mode: `development` or `production` |
| `uptime_seconds` | int64 | Seconds since server started |
| `pid` | int | Operating system process ID |
| `go_version` | string | Go runtime version, e.g. `go1.22.0` |
| `os_arch` | string | OS and architecture, e.g. `linux/amd64` |

### Hot Tier Stats Fields (added in Sprint 4)

| Field | Type | Description |
|---|---|---|
| `hot.total_writes` | int64 | Total Put operations to RocksDB since server start (resets on restart) |
| `hot.data_dir` | string | Filesystem path of the RocksDB data directory |

### WAL Stats Fields (added in Sprint 3)

| Field | Type | Description |
|---|---|---|
| `wal.segment_count` | int | Number of WAL segment files currently on disk |
| `wal.active_segment` | uint64 | Index of the currently active (writable) segment |
| `wal.active_size_bytes` | int64 | Bytes written to the active segment so far |
| `wal.total_entries` | int64 | Total WAL entries written since server start (resets on restart) |

---

### Example

```bash
curl http://localhost:8080/health
```

### Checks Performed

The health handler verifies that each data subdirectory is writable by
creating and immediately deleting a temporary file. Directories checked:

- `{data_dir}/wal`
- `{data_dir}/hot`
- `{data_dir}/cold/logs`
- `{data_dir}/cold/metrics`
- `{data_dir}/cold/json`
- `{data_dir}/cold/kv`

If any directory is not writable, the response is 503 with a `details` array
listing each failing directory.
