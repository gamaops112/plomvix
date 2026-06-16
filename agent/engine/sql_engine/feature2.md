feature2.md

Plomvix sql_engine — Feature 2: KVStore (Basic / bbolt)

Purpose
Implement the storage layer of the sql_engine: a durable, ordered key/value
store behind a backend-agnostic interface, with bbolt as the Basic-tier backend.
Feature 1 produces ordered byte keys; Feature 2 is where those keys/values are
durably stored, retrieved, deleted, and scanned in key order.

Every higher sql_engine layer (codec, schema, operations, transactions) talks to
this KVStore interface and NEVER to the backend directly. The swap to Pebble in
the enterprise tier must be invisible above the interface.

This is the storage layer only.
Do not add a value/row codec.
Do not add schema or catalog logic.
Do not add operations (Insert/Scan-by-table/etc — only raw byte Scan).
Do not add transaction policy (Begin/Commit/Rollback) — only atomic batch +
  read snapshot. Full transactions are Feature 6.
Do not add MVCC version management.
Do not add a shared Engine interface (no internal/engine/engine.go).
Do not parse or interpret key structure — the KVStore is a pure []byte->[]byte
  ordered store and must not know about tableIDs, versions, or Feature 1.
Do not add Pebble in this plan (enterprise tier).
Do not add networking or SQL parsing.

---

Feature Name
sql_engine KVStore (Basic)

Plan file:
feature2.md

Target packages:
internal/engine/sql/kv        (the KVStore interface + bbolt implementation)
internal/config               (extend with [sql_engine] section)

---

Required Starting State
This plan starts only after feature1.md is completed and verified.
Before starting, the project must already have and pass:
internal/engine/sql/key/      (Feature 1, complete)
internal/config/              (Config{Server,Data}, Default, Validate, Load)
internal/logger/
internal/lifecycle/
internal/runtime/
cmd/plomvix/main.go

And must pass:
go build ./...
go test ./...
go test -race ./...

Known facts about the existing config layer (do not re-derive; match exactly):
- module path: github.com/plomvix/plomvix
- config import: github.com/plomvix/plomvix/internal/config
- TOML library already in use: github.com/pelletier/go-toml/v2
- Config struct currently includes (at minimum): Server ServerConfig
  `toml:"server"`, Data DataConfig `toml:"data"`, Logger LoggerConfig
  `toml:"logger"`. There may be additional fields from enterprise tiers. DO NOT
  remove, rename, or ignore any existing field. You are ADDING one new field
  (SQL) alongside the existing ones.
- Public API: func Default() Config; func Validate(cfg Config) error;
  func Load(path string) (Config, error)
- Load pattern: cfg := Default(); decode TOML over it (partial TOML preserves
  defaults); then Validate.
- STRICT DECODE: the config layer REJECTS unknown TOML fields/sections. This
  means the new [sql_engine] section MUST be represented by a struct field with
  the correct toml tag, or loading any config containing [sql_engine] will fail.
  (This plan adds that field in Task 6, so it is consistent — but be aware that
  you cannot put [sql_engine] in a .toml file before the struct field exists.)
Before editing config, OPEN internal/config and read the actual current Config
struct, Default(), and Validate(). Match the real code. If the real struct
differs from the above, adapt additively and report the difference; never
rewrite or shrink the existing struct.

---

Go Version Requirement
Go 1.22 or later.
Allowed third-party dependencies in this plan:
- go.etcd.io/bbolt          (new — the Basic storage backend)
- github.com/pelletier/go-toml/v2  (already present — do not re-add/bump)
No other third-party dependencies.
Do not use t.Chdir; use t.TempDir() for on-disk test databases and let the test
framework clean them up.

---

Coding Agent
Coding agent: DeepSeek V4 Pro
Tasks one at a time, in exact order. Do not proceed until current task verifies.

---

Graphify Rule
Search Graphify before each task if available; update after; do not block if
unavailable; mention availability in the task report.

---

Global Project Rules
Keep the KVStore a thin, dumb, ordered durable byte store.
Do not interpret key contents.
Do not add panic recovery in the storage read/write path; on an invariant
  violation, return an error and let the caller fail fast. (Recovery from a
  crash is the backend's WAL responsibility, not a recovered panic.)
Get and Scan MUST return COPIES of keys/values — never a slice that points into
  backend-owned (e.g. mmap'd) memory that becomes invalid after the read.
The KVStore is a lifecycle component: it exposes Open/Close and a Name.
Do not import anything outside the allowed dependency list.
Do not make config, logger, lifecycle, or runtime depend on the kv package.
  (Dependency direction: runtime/engine wiring may import kv + config; kv must
  not import runtime/lifecycle/logger/config.)

---

Background: What the KVStore Is (read before coding)
A single durable, ordered map from []byte keys to []byte values. "Ordered" means
iteration and range scans visit keys in ascending byte (memcmp) order — exactly
the order Feature 1's key encoding was designed to produce. The store is opened
against a data directory, persists across process restarts, and is safe for
concurrent reads with a single writer (bbolt's native model in the Basic tier).

The interface deliberately exposes only:
- point ops: Get, Set, Delete
- ordered range scan: Scan(start, end) over the half-open range [start, end)
- atomic batch: apply a set of Set/Delete operations all-or-nothing
- read snapshot: a consistent read view for the duration of a scan/read
- lifecycle: Open, Close, Name

It deliberately does NOT expose Begin/Commit/Rollback transaction policy; that is
Feature 6. Atomic batch + snapshot is the minimal primitive the higher layers
need until then, and it maps cleanly onto bbolt's Update/View transactions.

Concurrency and state locking (MANDATORY):
The store wrapper holds mutable state (the NeverOpened/Open/Closed state and the
*bbolt.DB pointer). This state MUST be protected against data races with a
sync.RWMutex (or equivalent) inside the store:
- Open and Close take the write lock (they mutate state and the db pointer).
- Get/Set/Delete/Scan/Batch.Commit take the read lock for the duration needed to
  (a) verify the state is Open and (b) obtain the db pointer / run the bbolt
  txn. A read lock is sufficient because bbolt itself serializes its writers
  internally; the wrapper lock only guards the wrapper's own state + pointer,
  not bbolt's transaction concurrency.
- All access to the state field and the db pointer goes through the lock; never
  read them unlocked.
- The package must pass `go test -race` with concurrent Open-attempt/Get/Set
  exercises. Add a small concurrency test that, after Open, runs several
  goroutines doing Get/Set/Scan concurrently and asserts no race and no error.

---

Public API To Produce

  package kv

  import (
      "context"
      "errors"
  )

  // KVStore is a durable, ordered []byte->[]byte store.
  type KVStore interface {
      // Name identifies the store for lifecycle/logging.
      Name() string

      // Open prepares the store for use (opens/creates the backing file under
      // the configured data directory). ctx is accepted for interface
      // consistency; bbolt's open is not cancelable, so ctx is only checked for
      // cancellation before/after the open call, not during. Returns
      // ErrAlreadyOpen if already open, ErrClosed if previously closed.
      Open(ctx context.Context) error

      // Close flushes and closes the store, transitioning it to Closed.
      // ctx is accepted for interface consistency; Close may check ctx
      // cancellation BEFORE beginning to close, but once closing begins it must
      // not abort partway — the underlying DB is always fully closed. Calling
      // Close on a never-opened store -> ErrNotOpen; on an already-closed store
      // -> ErrClosed.
      // CLOSE-FAILURE RULE: if the underlying db.Close() returns an error, the
      // store STILL transitions to Closed (the state change is unconditional
      // once Close begins) and Close returns the underlying error. Subsequent
      // operations therefore return ErrClosed, never ErrOpen-state behavior.
      Close(ctx context.Context) error

      // Get returns a COPY of the value for key and whether it existed.
      Get(ctx context.Context, key []byte) (value []byte, found bool, err error)

      // Set stores value for key (atomic single write). value is copied.
      Set(ctx context.Context, key, value []byte) error

      // Delete removes key if present (no error if absent).
      Delete(ctx context.Context, key []byte) error

      // Scan iterates [start, end) in ascending key order, calling fn for each
      // pair. start==nil means "from the first key"; end==nil means "to the
      // last key". A nil fn returns ErrNilCallback. Returning a non-nil error
      // from fn stops the scan and is returned. Keys and values passed to fn
      // are COPIES, valid after fn returns.
      Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error

      // NewBatch returns a Batch for accumulating writes applied atomically.
      NewBatch() Batch
  }

  // Batch accumulates writes applied atomically by Commit. Not safe for
  // concurrent use by multiple goroutines.
  type Batch interface {
      Set(key, value []byte)
      Delete(key []byte)
      // Commit applies all accumulated operations atomically (all-or-nothing).
      // Before applying, Commit validates every recorded op: any op with an
      // empty key causes Commit to return ErrEmptyKey and apply NOTHING.
      // On success the batch is cleared, so a second Commit with no new ops is
      // a no-op returning nil. If the originating store is not Open (never
      // opened -> ErrNotOpen; already Closed -> ErrClosed), Commit applies
      // nothing and returns that error. (Set/Delete record ops and do not
      // return errors; all validation happens at Commit.)
      Commit(ctx context.Context) error
      // Reset discards accumulated (uncommitted) operations without applying.
      Reset()
  }

  // Sentinel errors.
  var (
      ErrNotOpen      = errors.New("kv: store not open")
      ErrAlreadyOpen  = errors.New("kv: store already open")
      ErrClosed       = errors.New("kv: store is closed")
      ErrEmptyKey     = errors.New("kv: key must not be empty")
      ErrNilCallback  = errors.New("kv: scan callback must not be nil")
  )

Context handling for data ops (MANDATORY):
Get/Set/Delete/Scan/Batch.Commit each check ctx.Err() BEFORE starting the bbolt
transaction; if ctx is already canceled or past deadline, return ctx.Err()
without touching the DB. bbolt transactions are not themselves cancelable, so no
mid-transaction cancellation is attempted. Scan additionally checks ctx.Err()
before invoking the callback on each iteration, returning ctx.Err() if canceled
mid-scan (stopping cleanly between rows).

Store state machine (MANDATORY — implement exactly):
A store is in exactly one of three states: NeverOpened, Open, Closed.
- NeverOpened: Open() -> transitions to Open. Any data op (Get/Set/Delete/Scan)
  or Close() while NeverOpened -> ErrNotOpen.
- Open: data ops succeed. Open() again while Open -> ErrAlreadyOpen.
  Close() -> transitions to Closed.
- Closed: any data op (Get/Set/Delete/Scan), Close() again, or Open() ->
  ErrClosed. (A closed store is terminal; it is not reopened. Reopening means
  constructing a NEW store via NewBBolt against the same path.)
This resolves the ErrNotOpen vs ErrClosed distinction: never-opened yields
ErrNotOpen, already-closed yields ErrClosed.

Notes:
- Empty key ([]byte of length 0) is invalid -> ErrEmptyKey. (Feature 1 keys are
  never empty: they always have at least tag+tableID+pk+version.)
- A nil value is treated as an empty value (stored as zero-length), distinct
  from "not found". found=false means the key does not exist.
- The bbolt implementation uses a single fixed bucket (e.g. "plomvix_sql") to
  hold all keys; bucket management is an internal detail, not part of the API.

---

Tasks (do in order, one at a time)

Task 1 — Add bbolt dependency and kv package skeleton
Add go.etcd.io/bbolt to go.mod. Use the latest stable v1 available at
implementation time, record the EXACT resolved version in go.mod and go.sum, and
report that exact version string in the task report (do not leave it vague).
Reproducibility: once resolved, the version is pinned by go.mod/go.sum and must
not drift in later tasks.
Create internal/engine/sql/kv/kv.go with the package declaration, the KVStore
and Batch interfaces, and the sentinel errors exactly as in the Public API.
Create internal/engine/sql/kv/kv_test.go with package kv and one trivial test
that references the sentinels (compile check).
Do NOT implement bbolt yet — interfaces and errors only.
Verification:
  go build ./...
  go test ./internal/engine/sql/kv/
Report Graphify availability and confirm only go.etcd.io/bbolt was added.

Task 2 — Implement bboltStore: Open/Close/Name
Create internal/engine/sql/kv/bbolt.go implementing an unexported bboltStore
struct that satisfies KVStore. Use os and path/filepath for directory creation.
Implement:
- a constructor: func NewBBolt(name, path string) KVStore  (path = the .db file
  path; does NOT open it yet)
- Name() returns the provided name
- Open(ctx): FIRST ensure the parent directory exists via
  os.MkdirAll(filepath.Dir(path), 0700) (so a configured data_dir like
  "data/sql" is created if absent); THEN open/create the bbolt file at path with
  file mode 0600; create the fixed bucket in an Update txn if missing.
  ErrAlreadyOpen if already open; ErrClosed if previously closed.
  OPEN-FAILURE CLEANUP: if directory creation fails, do not open the DB. If
  bbolt opens successfully but the subsequent bucket-creation txn fails, the
  implementation MUST close the bbolt DB and leave the store in the NeverOpened
  state (not half-open), returning the underlying error wrapped. No open file
  handle may leak on a failed Open.
- Close(ctx): close the bbolt DB and transition state to Closed. If db.Close()
  errors, STILL transition to Closed and return the error (close-failure rule).
  ErrNotOpen if never opened; ErrClosed if already closed.
Guard all data ops (added later) with a state check: never-opened -> ErrNotOpen,
closed -> ErrClosed, open -> proceed.
Tests (use t.TempDir() for the db path):
- NewBBolt then Open then Close succeeds
- double Open (while open) -> ErrAlreadyOpen
- Close without ever opening -> ErrNotOpen
- Close, then any op or second Close -> ErrClosed
- Open after Close -> ErrClosed (terminal closed state)
- file exists on disk after Open
- directory creation: give a path inside a NOT-yet-existing subdirectory of
  t.TempDir() (e.g. tmp/data/sql/sql.db); Open creates the directory and
  succeeds
- open-failure cleanup (if practical to trigger): construct a store via
  NewBBolt with a path whose directory cannot be created or is unwritable; call
  Open and assert it returns an error AND that this SAME store is still in the
  NeverOpened state — i.e. a subsequent Get returns ErrNotOpen (NOT ErrClosed),
  proving Open did not leave it half-open. (Note: NewBBolt's path is fixed per
  store, so "recovering" means constructing a FRESH store on a good path, not
  re-opening this one.) If the failure cannot be reliably triggered on the CI
  platform, document this in the task report and assert the NeverOpened ->
  ErrNotOpen behavior via the plain never-opened path instead.
Verification:
  go test ./internal/engine/sql/kv/

Task 3 — Implement Get / Set / Delete
Each of Set/Get/Delete first checks ctx.Err() and returns it if canceled, before
opening any bbolt txn (per the Context handling rule).
Implement Set (bbolt Update txn, put into the fixed bucket, copy the value in),
Get (bbolt View txn; copy the bytes out of the txn before returning — never
return the bbolt-internal slice), Delete (Update txn, delete; no error if
absent). Enforce ErrEmptyKey on empty key for all three. Enforce the state
machine: never-opened -> ErrNotOpen, closed -> ErrClosed.
Tests:
- Set then Get returns an equal COPY; mutating the returned slice does not
  change stored data (re-Get proves it)
- Get missing key -> found=false, no error
- Delete existing then Get -> found=false
- Delete missing -> no error
- empty key on Get/Set/Delete -> ErrEmptyKey
- nil value Set then Get -> found=true, zero-length value
- persistence: Set, Close; then construct a FRESH store via NewBBolt on the SAME
  path, Open it, Get returns the value (closed stores are terminal; persistence
  is verified with a new store instance, not by reopening the closed one)
- concurrency: after Open, run several goroutines performing Get/Set
  concurrently; the test must pass under `go test -race` with no error
- protect store state + db pointer with a sync.RWMutex per the Concurrency
  section: Open/Close take the write lock; Get/Set/Delete take the read lock
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 4 — Implement Scan (ordered, half-open range, copies)
Scan first checks ctx.Err() before opening the txn, and re-checks ctx.Err()
before each callback invocation (return ctx.Err() if canceled mid-scan).
Implement Scan using a bbolt View txn and the bucket Cursor:
- start==nil: begin at first key; else Seek(start)
- iterate while key < end (end==nil means no upper bound); half-open [start,end)
- for each pair, pass COPIES of key and value to fn (copy before calling fn)
- if fn returns an error, stop and return it
- state machine: never-opened -> ErrNotOpen, closed -> ErrClosed
- nil fn -> ErrNilCallback (validate before starting the txn; do not panic)
Tests (insert keys in non-sorted insertion order, assert sorted visitation):
- full scan (nil,nil) visits all keys in ascending byte order
- bounded scan [b, d) excludes d and anything >= d, includes b
- start==nil bounded above; end==nil bounded below
- empty range (start==end) visits nothing
- fn error stops iteration and is returned
- keys/values given to fn are copies (mutating them does not corrupt the store;
  a subsequent scan still returns originals)
- ordering test using actual Feature 1 encoded keys: import the key package in
  the TEST only, encode several keys, Set them in random order, Scan, and assert
  visitation order equals bytes-sorted order. (This validates that Feature 1 +
  Feature 2 compose correctly. The kv package itself still must NOT import key.)
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 5 — Implement Batch (atomic all-or-nothing)
Implement a bboltBatch that records Set/Delete ops in memory and applies them in
a single bbolt Update txn on Commit (atomic: if the txn fails, none apply).
Reset clears recorded ops. NewBatch returns a fresh batch bound to the store.
Copy key/value bytes when recording so later caller mutation cannot affect the
pending batch.
Tests:
- batch of several Sets + a Delete, Commit, then Get reflects all changes
- a batch that is built but not Committed changes nothing
- Reset discards pending ops (Commit after Reset is a no-op returning nil)
- double Commit: after a successful Commit, calling Commit again is a no-op
  returning nil (the batch was cleared) and does NOT reapply the ops
- batch built while Open, then store Closed, then Commit -> ErrClosed and
  nothing is applied
- batch created and Committed against a never-opened store -> ErrNotOpen
- batch with an empty-key Set -> Commit returns ErrEmptyKey and applies nothing
- batch with an empty-key Delete -> Commit returns ErrEmptyKey and applies
  nothing (verify a valid op in the same batch is also NOT applied)
- atomicity: after a successful Commit all ops are visible (exercise success and
  no-op paths; forcing a mid-txn failure is out of scope for Basic)
- mutating the slices passed to batch.Set after the call does not change what
  gets committed
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 6 — Extend internal/config with the [sql_engine] section
Extend the existing config package (match its exact style; do not rewrite it):
- Add type SQLConfig struct with toml tags:
    DataDir string `toml:"data_dir"`   // directory holding the sql_engine db
    Backend string `toml:"backend"`    // "bbolt" (Basic). "pebble" reserved.
- Add field to Config:  SQL SQLConfig `toml:"sql_engine"`
- Extend Default() so cfg.SQL has sensible defaults:
    DataDir = "data/sql"
    Backend = "bbolt"
- Extend Validate(cfg) to check:
    cfg.SQL.DataDir is not empty            -> "sql_engine data_dir is required"
    cfg.SQL.Backend is one of {"bbolt"}     -> "sql_engine backend must be bbolt"
  (Only "bbolt" is accepted in the Basic tier. "pebble" is added by the
  enterprise tier; do NOT accept it here.)
- Preserve the decode-over-Default() behavior: partial TOML without an
  [sql_engine] section must still yield the defaults (do not regress the
  existing partial-load test).
Tests (in the config package):
- Default().SQL has DataDir="data/sql", Backend="bbolt"
- Validate rejects empty DataDir and rejects an unknown backend (e.g. "pebble",
  "rocks") with the documented messages
- loading a TOML that omits [sql_engine] preserves the defaults
- loading a TOML with [sql_engine] data_dir/backend overrides them
Verification:
  go test ./internal/config/
  go test ./...

Task 7 — Update config.toml and config.example.toml
Add an [sql_engine] section to BOTH the root config.toml and
config.example.toml, consistent with the new defaults:
  [sql_engine]
  data_dir = "data/sql"
  backend  = "bbolt"
Keep the existing [server] and [data] sections intact.
Verification:
  go build ./...
  go test ./...
  (manually confirm both files contain the [sql_engine] section)

Task 8 — End-to-end example test (no new API)
Do NOT add any new constructor or helper. NewBBolt(name, path) from Task 2 is
the only construction entry point and is sufficient.
Add ONE end-to-end example test in the kv package that:
- computes a db path the way runtime wiring will, e.g.
  filepath.Join(t.TempDir(), "sql", "sql.db") (a path whose parent dir does not
  yet exist, exercising Open's directory creation),
- constructs the store via NewBBolt("sql", path),
- Opens, does Set + Get (asserting the value round-trips), runs a small Scan,
  Closes,
- and documents in a comment that runtime/engine wiring (a LATER feature) is
  responsible for deriving this path from cfg.SQL.DataDir
  (filepath.Join(cfg.SQL.DataDir, "sql.db")) and calling NewBBolt.
Do NOT modify cmd/plomvix/main.go or runtime in this plan. Engine registration
into the lifecycle/runtime is a later feature (assembled when the sql_engine
type itself exists). Feature 2 delivers the storage component only.
Verification:
  go build ./...
  go test ./...
  go test -race ./...

Task 9 — Documentation
Create docs/sql_engine_kv.md describing:
- what the KVStore is (durable ordered []byte->[]byte store, key-format-agnostic)
- the full interface (Get/Set/Delete/Scan/Batch/Open/Close/Name) and the
  half-open [start,end) scan semantics
- the copy guarantee (Get/Scan return copies; callers may mutate freely)
- the Basic backend (bbolt), the fixed bucket, the data_dir/backend config
- the deliberate non-goals: no transaction policy (Feature 6), no key parsing,
  no MVCC, no Pebble (enterprise)
- how it composes with Feature 1 (Feature 1 makes ordered keys; Feature 2 stores
  and range-scans them in that order)
Update docs index/readme if one exists to reference the new doc.
Verification:
  go build ./...
  go test ./...
  go test -race ./...

---

Definition Of Done
internal/engine/sql/kv/ implements KVStore + Batch on bbolt
Get/Scan return copies (mutation tests pass)
Scan is ordered, half-open [start,end), with Feature-1-encoded-key ordering test
Batch applies atomically; uncommitted/Reset batches change nothing
store state machine correct: never-opened ops -> ErrNotOpen; ops after Close ->
  ErrClosed; double-open while open -> ErrAlreadyOpen; Open after Close ->
  ErrClosed; empty-key -> ErrEmptyKey; nil Scan callback -> ErrNilCallback
failed Open leaves the store NeverOpened with no leaked handle
store state + db pointer guarded by a sync.RWMutex; passes go test -race under
  concurrent Get/Set/Scan
Close transitions to Closed even if db.Close() errors (returns the error)
data ops check ctx.Err() before starting a txn (and Scan re-checks per row)
exact resolved go.etcd.io/bbolt version recorded in go.mod/go.sum and reported
Open creates a missing data directory (os.MkdirAll)
Batch: double Commit is a no-op; Commit against closed/never-opened store ->
  ErrClosed/ErrNotOpen and applies nothing; empty-key op in a batch -> Commit
  returns ErrEmptyKey and applies nothing
persistence across Close + fresh NewBBolt/Open on the same path verified
kv package does NOT import key/config/logger/lifecycle/runtime
config extended with SQLConfig + [sql_engine]; Default/Validate updated; partial
  load still preserves defaults
config.toml and config.example.toml contain [sql_engine]
docs/sql_engine_kv.md exists and matches the code
go build ./... passes
go test ./... passes
go test -race ./... passes
only go.etcd.io/bbolt added as a new dependency

---

Out Of Scope (do not implement in this plan)
Pebble backend (enterprise tier)
reverse iteration (enterprise tier)
transaction policy Begin/Commit/Rollback (Feature 6)
MVCC version management (Feature 6)
value/row codec (Feature 3)
schema / catalog-lite / tableID assignment (Feature 4)
operations CreateTable/Insert/Scan-by-table (Feature 5)
shared Engine interface / engine.go
full engine registration into runtime (later feature)
SQL parsing, networking