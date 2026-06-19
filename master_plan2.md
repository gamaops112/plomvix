# Plomvix — Master Development Plan

> **Combined from all agent plan files.**
> Each plan below links to its canonical source file in `agent/`.

---

## How to Read This Document

Plans are grouped by **foundation area** and listed in **implementation order** (prerequisites first). Each entry includes:

- **Source** — the canonical plan file
- **Package(s)** — what code will be created or modified
- **Purpose** — one-line summary
- **Dependencies** — which plans must be completed first
- **Key API / Concepts** — what the plan delivers

---

# Part I: Core Foundations

These plans build the bedrock of the Plomvix application — configuration,
logging, lifecycle management, and runtime wiring. They must be completed
in order before any SQL engine work begins.

---

## 1. Config Setup

| Field | Value |
|-------|-------|
| **Source** | `agent/config_setup.md` |
| **Package** | `internal/config` |
| **Purpose** | Create the initial configuration system with TOML loading, validation, and defaults. |
| **Dependencies** | None (first foundation plan). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Config types + `Default()`, `Validate()`, `Load()` |
| `internal/config/config_test.go` | Tests for defaults, validation, TOML loading |

### API

```go
type Config struct {
    Server ServerConfig `toml:"server"`
    Data   DataConfig   `toml:"data"`
}

func Default() Config
func Validate(cfg Config) error
func Load(path string) (Config, error)
```

### Tasks (7 total)

1. **Create minimal config types and defaults** — `Config`, `ServerConfig`, `DataConfig`, `Default()`
2. **Add default config tests** — external test package, verify defaults
3. **Add config validation** — `Validate()` rules for host, port, path
4. **Add config validation tests** — table-driven invalid cases
5. **Add TOML loading** — `Load()` using `github.com/pelletier/go-toml/v2`
6. **Add TOML loading tests** — valid file, missing file, partial TOML, strict decode
7. **Update root config files** — `config.toml`, `config.example.toml`

### Key Constraints

- Only standard library + `go-toml/v2`
- `Server.Port` is `int` (not `uint`)
- Root `config.toml` lives at project root
- No WAL, engine, query, API, or UI config fields
- `cmd/plomvix/main.go` is not touched

---

## 2. Enterprise Config Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/config_enterprise.md` |
| **Package** | `internal/config` |
| **Purpose** | Production-grade hardening: field-level errors, path normalization, strict decode, documentation. |
| **Dependencies** | Config Setup (above) must be complete. |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Field-level validation errors** — `server.host is required`, `server.port must be between 1 and 65535`
- **Table-driven validation tests** — exact error messages
- **Path normalization** — `normalize()` using `filepath.Clean`
- **Strict TOML decode** — reject unknown fields
- **Example config validation** — load `config.example.toml` in tests
- **Documentation** — `docs/config.md` with config precedence model and fail-fast policy

### Tasks (8 total)

1. Field-level validation errors
2. Table-driven validation tests
3. Path normalization (`normalize()`)
4. Path normalization tests
5. Strict TOML decode (reject unknown fields)
6. Config documentation
7. Example config validation test
8. Final scope-control review

---

## 3. Logging Setup

| Field | Value |
|-------|-------|
| **Source** | `agent/logging_setup.md` |
| **Package** | `internal/logger`, `internal/config` (extend) |
| **Purpose** | Add production-grade logging using Go standard `log/slog`. |
| **Dependencies** | Config Setup + Enterprise Config Hardening (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Extended with `LoggerConfig` |
| `internal/logger/logger.go` | Logger creation (`New()`) |
| `internal/logger/logger_test.go` | External test package |
| `internal/logger/logger_internal_test.go` | Internal test helper |

### API

```go
// Config extension
type LoggerConfig struct {
    Level  string `toml:"level"`  // debug|info|warn|error
    Format string `toml:"format"` // text|json
    Output string `toml:"output"` // stdout|stderr
}

// Logger package
func New(cfg config.LoggerConfig) (*slog.Logger, error)
func newWithWriter(cfg config.LoggerConfig, w io.Writer) (*slog.Logger, error) // internal
```

### Tasks (10 total)

1. Add `LoggerConfig` type and defaults to config
2. Add logger default tests
3. Add logger config validation
4. Add logger validation tests
5. Create `internal/logger` package
6. Implement `New()` and `newWithWriter()`
7. Add base logger tests
8. Implement config loading with TOML logger section
9. Add TOML loading tests for logger config
10. Update `config.toml` and `config.example.toml`

---

## 4. Enterprise Logger Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/logger_enterprise.md` |
| **Package** | `internal/logger` |
| **Purpose** | Safer, more consistent logger: field constants, component scoping, redaction. |
| **Dependencies** | Logging Setup (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Structured field constants** — `FieldComponent`, `FieldError`, `FieldPath`, `FieldDuration`
- **Component-scoped logger** — `WithComponent(base, "config")` → `component=config`
- **Error attribute helper** — `ErrorAttr(err)` → `error="message"`
- **Sensitive key constants & redaction** — `SensitiveKey` constants, `RedactSensitive()` helper
- **Runtime level foundation** — `SetLevel()` support
- **Logging policy documentation** — `docs/logging.md`

### Tasks (10 total)

1. Standard logger field constants
2. Field constant tests
3. Component-scoped logger helper (`WithComponent`)
4. Component-scoped logger tests
5. Error attribute helper (`ErrorAttr`)
6. Error attribute tests
7. Sensitive key constants + redaction helper
8. Sensitive key/redaction tests
9. Runtime level foundation + tests
10. Logging policy documentation

---

## 5. Lifecycle Foundation

| Field | Value |
|-------|-------|
| **Source** | `agent/lifecycle.md` |
| **Package** | `internal/lifecycle` |
| **Purpose** | First minimal lifecycle manager: register, start, stop components in order. |
| **Dependencies** | Config Setup + Logging Setup (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/lifecycle/lifecycle.go` | Component interface + Manager |
| `internal/lifecycle/lifecycle_test.go` | Lifecycle tests |
| `docs/lifecycle.md` | Lifecycle documentation |

### API

```go
type Component interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type Manager struct { /* unexported */ }

func NewManager() *Manager
func (m *Manager) Register(component Component) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
```

### Tasks (9 total)

1. Create lifecycle package skeleton
2. Add lifecycle skeleton tests
3. Implement Register with state management
4. Implement Start with ordered component startup
5. Implement Stop with reverse-order shutdown + error combining
6. Implement full lifecycle tests
7. Implement context cancellation integration
8. Add race-condition tests
9. Create lifecycle documentation

### Key Behavior

- Registration order preserved; start in order, stop in reverse order
- Registration after start begins is rejected
- Stop is idempotent (safe to call multiple times)
- `go test -race ./...` must pass

---

## 6. Enterprise Lifecycle Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/lifecycle_enterprise.md` |
| **Package** | `internal/lifecycle` |
| **Purpose** | Hardened lifecycle: state machine, duplicate detection, panic recovery. |
| **Dependencies** | Lifecycle Foundation (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Lifecycle state enum** — `StateNew`, `StateStarting`, `StateStarted`, `StateStopping`, `StateStopped`, `StateFailed`
- **State inspection API** — `Manager.State() State`
- **State transition rules** — strict, documented transitions
- **Duplicate component name rejection** — `ErrDuplicateComponent`
- **Panic recovery** — recover panics in `Start()` and `Stop()`, mark state as `failed`
- **Clearer lifecycle errors** — `ErrInvalidState`
- **Enterprise lifecycle tests** — state transitions, panic recovery, duplicate detection

### Tasks (8 total)

1. Lifecycle state enum + State() method
2. State tests
3. Duplicate component name rejection
4. Duplicate name tests
5. Panic recovery in Start
6. Panic recovery in Stop
7. State transition integration tests
8. Enterprise lifecycle documentation

---

## 7. Runtime Setup & Core Composition

| Field | Value |
|-------|-------|
| **Source** | `agent/runtime_setup.md` |
| **Package** | `internal/runtime`, `cmd/plomvix/main.go` |
| **Purpose** | Wire config + logger + lifecycle into the first working application entrypoint. |
| **Dependencies** | All previous foundation plans (1–6). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/runtime/runtime.go` | Runtime options + `Run()` |
| `internal/runtime/runtime_test.go` | Runtime tests |
| `cmd/plomvix/main.go` | Application entrypoint wired to runtime |
| `cmd/plomvix/main_test.go` | Entrypoint tests |
| `docs/runtime.md` | Runtime documentation |

### API

```go
type Options struct {
    ConfigPath string
}

const DefaultConfigPath = "config.toml"

func DefaultOptions() Options
func Run(ctx context.Context, opts Options) error
```

### Runtime Behavior

1. Read options → resolve config path → load config → create logger → create lifecycle manager → start lifecycle → stop lifecycle → return errors

### Tasks (7 total)

1. Create runtime package skeleton
2. Add runtime options tests
3. Implement config loading in runtime
4. Implement logger creation + lifecycle integration
5. Wire `cmd/plomvix/main.go` to `runtime.Run()`
6. Add `main_test.go` for entrypoint testing
7. Create runtime documentation

---

## 8. Enterprise Runtime Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/runtime_enterprise.md` |
| **Package** | `internal/runtime` |
| **Purpose** | Hardened runtime: error classification, timeouts, panic recovery, state object. |
| **Dependencies** | Runtime Setup (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Error sentinels** — `ErrInvalidOptions`, `ErrLoadConfig`, `ErrCreateLogger`, `ErrStartLifecycle`, `ErrStopLifecycle`, `ErrRuntimePanic`
- **Timeout policy** — `StartupTimeout` (default 30s), `ShutdownTimeout` (default 30s)
- **Runtime state object** — `Runtime` struct with `New()`, `Start()`, `Stop()`, `State()`
- **Panic recovery** — recover and return `ErrRuntimePanic`
- **Error classification** — `errors.Is()` compatible wrapping

### Tasks (9 total)

1. Enterprise runtime errors
2. Error sentinel tests
3. Timeout options + defaults
4. Timeout tests
5. Runtime state object (`New()`, `Start()`, `Stop()`)
6. Runtime state tests
7. Panic recovery
8. Panic recovery tests
9. Hardened runtime documentation

---

## 9. Runtime Signal Handling

| Field | Value |
|-------|-------|
| **Source** | `agent/runtime_signals.md` |
| **Package** | `internal/runtime` |
| **Purpose** | OS signal handling for clean process shutdown (SIGTERM, SIGINT, SIGHUP, SIGQUIT). |
| **Dependencies** | Enterprise Runtime Hardening (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Signal-aware context** — `withSignalContext()` creates context cancelled on signal
- **`RunWithSignals()`** — top-level entrypoint that handles OS signals
- **`ErrShutdownTimeout`** — new error sentinel for shutdown timeout expiry
- **Signal handlers** — SIGTERM, SIGINT, SIGHUP (treated as shutdown), SIGQUIT
- **SIGHUP policy** — currently shutdown; designated future config reload signal

### API

```go
var ErrShutdownTimeout = errors.New("runtime: shutdown timeout")

func RunWithSignals(opts Options) error
```

### Tasks (6 total)

1. Add `ErrShutdownTimeout` sentinel
2. Add shutdown timeout error tests
3. Implement `withSignalContext()` and `withSignalContextFromChan()`
4. Signal context tests
5. Implement `RunWithSignals()` + wire into `main.go`
6. Update runtime documentation for signal handling

---

# Part II: SQL Engine — Key Encoding

These plans build the key encoding layer that every downstream SQL engine
feature depends on.

---

## 10. SQL Key Encoding Setup

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/sql_key_setup.md` |
| **Package** | `internal/engine/sql/key` |
| **Purpose** | Pure, dependency-free key encoding for int64, uint64, string, bytes, sort-safe composites, and storage composites. |
| **Dependencies** | Runtime Signal Handling (plan 9). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/engine/sql/key/key.go` | Key encoding/decoding |
| `internal/engine/sql/key/key_test.go` | Tests |
| `internal/engine/sql/key/key_internal_test.go` | Internal tests |
| `docs/sql_key.md` | Key encoding documentation |

### API

```go
type Kind uint8

const (
    KindUint64 Kind = 1
    KindInt64  Kind = 2
    KindString Kind = 3
    KindBytes  Kind = 4
)

type Key struct { /* unexported */ }

// Encoding
func EncodeUint64(v uint64) Key
func EncodeInt64(v int64) Key
func EncodeString(s string) (Key, error)
func EncodeBytes(b []byte) Key
func EncodeSortComposite(fields ...any) (Key, error)
func EncodeStorageComposite(fields ...any) (Key, error)

// Decoding
func DecodeUint64(k Key) (uint64, error)
func DecodeInt64(k Key) (int64, error)
func DecodeString(k Key) (string, error)
func DecodeBytes(k Key) ([]byte, error)
func DecodeSortComposite(k Key) ([]any, error)
func DecodeStorageComposite(k Key) ([]any, error)

// Schema-driven decode
func ParseKey(data []byte, kinds []Kind) (Key, error)
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)

// Key methods
func (k Key) Bytes() []byte
func (k Key) Compare(other Key) int
func (k Key) Fields() []Field
```

### Key Design Decisions

- Big-endian only; no little-endian variants
- Int64 uses sign-bit flip for sort-safe encoding
- Strings are null-terminated (standalone) or length-prefixed (composites)
- `Key.Bytes()` and `Key.Fields()` return copies
- Zero internal imports; standard library only

---

## 11. SQL Key Encoding Enterprise Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/sql_key_enterprise.md` |
| **Package** | `internal/engine/sql/key` |
| **Purpose** | Centralized validation, fuzz testing, benchmarks, format stability. |
| **Dependencies** | SQL Key Encoding Setup (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Centralized field validation** — `validateField()` helper (overflow-safe bounds checking)
- **Decoder refactoring** — all decoders use `validateField()`
- **Go fuzz tests** — `FuzzParseKey`, `FuzzParseStorageCompositeKey`
- **Benchmarks** — hot-path encode/decode benchmarks
- **Format stability documentation** — frozen format guarantee

### Tasks (6 total)

1. Add `validateField` helper
2. Add `validateField` tests
3. Wire `validateField` into all decoders
4. Add Go fuzz tests for `ParseKey` and `ParseStorageCompositeKey`
5. Add benchmarks for scalar and composite encode/decode
6. Format stability documentation

---

## 12. Feature 1: Key Encoding (sql_engine)

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/feature1.md` |
| **Package** | `internal/engine/sql/key` |
| **Purpose** | Order-preserving byte key encoding with keyspace tags, table IDs, composite PKs, and MVCC version slots. |
| **Dependencies** | SQL Key Encoding Enterprise Hardening (above). |
| **Coding Agent** | DeepSeek V4 Pro |

> **Note:** This is a DIFFERENT key encoding plan than `sql_key_setup.md`.
> This plan implements a wire-level key format for the storage engine
> (keyspace tag + tableID + encoded PK columns + version), while
> `sql_key_setup.md` provides the standalone encoding primitives.

### API

```go
// Keyspace tags
const (
    TagTableData byte = 0x01
    TagMetadata  byte = 0x02 // reserved
    TagIndex     byte = 0x03 // reserved
)

// Value type for PK columns
type Value struct { /* unexported */ }

func Null() Value
func Bool(v bool) Value
func Int64(v int64) Value
func Uint64(v uint64) Value
func String(v string) Value
func Bytes(v []byte) Value // copies input

func (val Value) Kind() Kind
func (val Value) AsBool() (bool, bool)
func (val Value) AsInt64() (int64, bool)
func (val Value) AsUint64() (uint64, bool)
func (val Value) AsString() (string, bool)
func (val Value) AsBytes() ([]byte, bool) // returns copy
func (val Value) Equal(other Value) bool

// Key encoding/decoding
func EncodeTableRowKey(tableID uint64, pk []Value, version uint64) ([]byte, error)
func DecodeTableRowKey(b []byte, expectedKinds []Kind) (tableID uint64, pk []Value, version uint64, err error)
func TablePrefix(tableID uint64) []byte
```

### Tasks (9 total)

1. Create key package skeleton (constants, errors, stubs)
2. Implement `Value` type, constructors, accessors, `Equal`
3. Implement order-preserving single-column encode/decode
4. Ordering tests for single-column encoding
5. Implement `EncodeTableRowKey` and `TablePrefix`
6. Implement `DecodeTableRowKey` (schema-arity based)
7. Full-key ordering tests (version inversion, composite PK)
8. Error-path and randomized round-trip tests
9. Documentation (`docs/sql_engine_key.md`)

---

## 13. Feature 1 Enterprise: Key Encoding Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/feature1_enterprise.md` |
| **Package** | `internal/engine/sql/key` |
| **Purpose** | Canonical encoding validation, golden vectors, property tests, benchmarks, format lock. |
| **Dependencies** | Feature 1: Key Encoding (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Canonical encoding validation** — `IsCanonical()` verifies encode(decode(x)) == x
- **Prefix/range helpers** — `PrefixEnd()`, `TableRange()` for scan bounds
- **Golden vectors** — hand-authored hex literals locking the on-disk format
- **Property-style ordering tests** — randomized property verification
- **API immutability proofs** — copy-safety tests for all public API
- **Benchmarks** — encode/decode benchmarks
- **Format stability documentation** — frozen format guarantee

### Tasks (8 total)

1. Canonical encoding validation (`IsCanonical`)
2. Stricter malformed-key tests (exhaustive table-driven)
3. Prefix/range helpers (`PrefixEnd`, `TableRange`) + tests
4. Deterministic golden vectors (hand-authored hex, format lock)
5. Property-style ordering tests (randomized invariants)
6. API immutability / copy-safety proofs
7. Benchmarks
8. Format-stability documentation and guard

---

# Part III: SQL Engine — Storage Layer

---

## 14. Feature 2: KVStore Basic (bbolt)

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/feature2.md` |
| **Package** | `internal/engine/sql/kv`, `internal/config` (extend) |
| **Purpose** | Durable ordered KV store behind a backend-agnostic interface, with bbolt as the Basic tier. |
| **Dependencies** | Feature 1: Key Encoding (plan 12). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/engine/sql/kv/kv.go` | KVStore + Batch interfaces, sentinel errors |
| `internal/engine/sql/kv/bbolt.go` | bbolt backend implementation |
| `internal/engine/sql/kv/bbolt_test.go` | Tests |
| `internal/config/config.go` | Extended with `SQLConfig` |
| `docs/sql_engine_kv.md` | KVStore documentation |

### API

```go
type KVStore interface {
    Name() string
    Open(ctx context.Context) error
    Close(ctx context.Context) error
    Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
    Set(ctx context.Context, key, value []byte) error
    Delete(ctx context.Context, key []byte) error
    Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
    NewBatch() Batch
}

type Batch interface {
    Set(key, value []byte)
    Delete(key []byte)
    Commit(ctx context.Context) error
    Reset()
}

func NewBBolt(name, path string) KVStore
```

### Config Extension

```go
type SQLConfig struct {
    DataDir string `toml:"data_dir"`
    Backend string `toml:"backend"`
}
```

Default: `DataDir = "data/sql"`, `Backend = "bbolt"`.

### Tasks (9 total)

1. Add bbolt dependency + kv package skeleton
2. Implement bboltStore: Open/Close/Name
3. Implement Get/Set/Delete
4. Implement Scan (ordered, half-open, copies)
5. Implement Batch (atomic all-or-nothing)
6. Extend config with `[sql_engine]` section
7. Update `config.toml` and `config.example.toml`
8. End-to-end example test
9. KVStore documentation

---

## 15. Feature 2 Enterprise: KVStore Hardening (Pebble)

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/feature2_enterprise.md` |
| **Package** | `internal/engine/sql/kv`, `internal/config` (extend) |
| **Purpose** | Add Pebble backend, reverse scans, snapshots, diagnostics, compliance suite, benchmarks. |
| **Dependencies** | Feature 2: KVStore Basic (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Extended API

```go
// New methods on KVStore
ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
NewSnapshot(ctx context.Context) (Snapshot, error)
Stats(ctx context.Context) (Stats, error)
Check(ctx context.Context) error

type Snapshot interface {
    Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
    Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
    ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
    Close() error
}

type Stats struct {
    Backend    string
    KeyCount   int64
    SizeBytes  int64
    ReadOnly   bool
    SyncWrites bool
}

type Options struct {
    Backend      string // "bbolt" | "pebble"
    Path         string
    SyncWrites   bool
    ReadOnly     bool
    CacheSizeMB  int
    MaxOpenFiles int
}

func NewPebble(name, path string, opts Options) (KVStore, error)
func New(name string, opts Options) (KVStore, error)
```

### Tasks (10 total)

1. Define extended interface + bbolt stubs + compliance suite
2. Add Pebble dependency + implement `NewPebble`
3. Run compliance suite against Pebble (parity proof)
4. Implement `ScanReverse` on both backends
5. Implement `Snapshot`/`NewSnapshot` on both backends
6. Backend options + config (`sync_writes`, `read_only`, pebble opts)
7. Implement `Stats` and `Check` diagnostics on both backends
8. Benchmarks (bbolt vs Pebble)
9. Crash-safety / durability tests
10. Large dataset scan tests (10k / 100k)

---

## 16. In-Memory KV Store Setup

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/kv_store_setup.md` |
| **Package** | `internal/engine/sql/store` |
| **Purpose** | Ordered in-memory key-value store using sorted slice + binary search, built on the key package. |
| **Dependencies** | SQL Key Encoding Enterprise Hardening (plan 11). |
| **Coding Agent** | DeepSeek V4 Pro |

### Deliverables

| File | Purpose |
|------|---------|
| `internal/engine/sql/store/store.go` | In-memory store |
| `internal/engine/sql/store/store_test.go` | Tests |
| `docs/sql_store.md` | Store documentation |

### API

```go
type Entry struct {
    Key   key.Key
    Value []byte
}

type Store struct { /* unexported */ }

func New() *Store

func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
func (s *Store) Delete(k key.Key) error
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
func (s *Store) Len() int
```

### Tasks (7 total)

1. Create store package skeleton and types
2. Implement Put/Get (copy safety, overwrite semantics)
3. Implement Delete (no-op on missing)
4. Implement Scan (half-open range, sorted order)
5. Add concurrency tests (RWMutex, `go test -race`)
6. Add edge-case tests (nil store, empty range, nil key)
7. Store documentation

### Key Design

- Sorted slice + `sort.Search` for O(log n) reads, O(n) writes
- Single `sync.RWMutex` for concurrency
- In-memory stepping stone; no disk persistence
- Values copied at API boundary

---

## 17. In-Memory KV Store Enterprise Hardening

| Field | Value |
|-------|-------|
| **Source** | `agent/engine/sql_engine/kv_store_enterprise.md` |
| **Package** | `internal/engine/sql/store` |
| **Purpose** | Stress testing, size-scaling benchmarks, performance baseline for on-disk migration decision. |
| **Dependencies** | In-Memory KV Store Setup (above). |
| **Coding Agent** | DeepSeek V4 Pro |

### Key Additions

- **Sorted-invariant stress test** — 50 goroutines × 200 ops on overlapping keys, verify ascending order after churn
- **Duplicate-key concurrent stress** — many goroutines writing same keys
- **Size-scaling benchmarks** — Put/Get/Delete/Scan at 1k, 10k, 100k pre-populated entries
- **Performance baseline documentation** — evidence for when on-disk storage becomes necessary

### Tasks (3 total)

1. Sorted-invariant stress test
2. Duplicate-key concurrent stress variant
3. Size-scaling benchmarks (1k / 10k / 100k entries)

---

# Dependency Graph (Visual)

```
1. Config Setup
    └─ 2. Enterprise Config Hardening
3. Logging Setup
    └─ 4. Enterprise Logger Hardening
5. Lifecycle Foundation
    └─ 6. Enterprise Lifecycle Hardening
7. Runtime Setup ─── depends on 1–6
    └─ 8. Enterprise Runtime Hardening
        └─ 9. Runtime Signal Handling
              └─ 10. SQL Key Encoding Setup
                    └─ 11. SQL Key Encoding Enterprise Hardening
                          ├─ 12. Feature 1: Key Encoding
                          │     └─ 13. Feature 1 Enterprise: Key Encoding Hardening
                          ├─ 14. Feature 2: KVStore Basic (bbolt)
                          │     └─ 15. Feature 2 Enterprise: KVStore Hardening (Pebble)
                          └─ 16. In-Memory KV Store Setup
                                └─ 17. In-Memory KV Store Enterprise Hardening
```

---

# Quick Reference: Package Map

| Package | Plan | Status |
|---------|------|--------|
| `internal/config` | Config Setup + Enterprise Config Hardening | Planned |
| `internal/logger` | Logging Setup + Enterprise Logger Hardening | Planned |
| `internal/lifecycle` | Lifecycle Foundation + Enterprise Lifecycle Hardening | Planned |
| `internal/runtime` | Runtime Setup + Enterprise Runtime Hardening + Signal Handling | Planned |
| `cmd/plomvix/main.go` | Runtime Setup + Signal Handling | Planned |
| `internal/engine/sql/key` | SQL Key Encoding + Enterprise Hardening + Feature 1 + Feature 1 Enterprise | Planned |
| `internal/engine/sql/kv` | Feature 2 + Feature 2 Enterprise | Planned |
| `internal/engine/sql/store` | In-Memory KV Store Setup + Enterprise Hardening | Planned |

---

# Quick Reference: Docs Map

| Document | Source Plan | Purpose |
|----------|-------------|---------|
| `docs/config.md` | Enterprise Config Hardening | Configuration precedence, fail-fast policy |
| `docs/logging.md` | Enterprise Logger Hardening | Logging policy, format vs output, supported values |
| `docs/lifecycle.md` | Lifecycle Foundation | Lifecycle API, state machine, component rules |
| `docs/runtime.md` | Runtime Setup + Signal Handling | Runtime composition, config/logger/lifecycle wiring |
| `docs/sql_key.md` | SQL Key Encoding Setup | Key encoding API, encoding rules, composite formats |
| `docs/sql_engine_key.md` | Feature 1: Key Encoding | Wire-level key format, keyspace tags, version slot |
| `docs/sql_engine_kv.md` | Feature 2: KVStore Basic | KVStore interface, bbolt backend, scan semantics |
| `docs/sql_store.md` | In-Memory KV Store Setup | In-memory store API, sorted slice design, concurrency |

---

> **Generated from 17 agent plan files in `agent/`.**
> Last updated: 2026-06-18
