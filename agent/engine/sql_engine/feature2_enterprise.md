feature2_enterprise.md

Plomvix sql_engine — Feature 2 Enterprise: KVStore Hardening

Purpose
Harden the completed Feature 2 KVStore into an enterprise-grade storage layer:
add a second backend (Pebble) behind the SAME interface, a shared backend
compliance suite proving both backends behave identically, reverse scans, an
explicit read-snapshot API, safe configurable backend options, diagnostics
(Stats/Check), benchmarks, crash-safety tests, large-dataset tests, and docs.

This is hardening + interface EXPANSION.
It MUST NOT change the existing Feature 2 KVStore BEHAVIOR (state machine, copy
guarantees, error semantics, batch atomicity, context handling). However, this
plan DOES expand the KVStore interface (adding ScanReverse, NewSnapshot, Stats,
Check). Be clear-eyed: in Go, adding methods to an interface is a BREAKING change
for every implementer — bbolt's implementation must be updated in the SAME task
sequence so the package always compiles. "Expansion, implemented for all backends
together" — NOT "purely additive." Existing method signatures and their behavior
are frozen; only new methods and new options are introduced. If a hardening step
appears to require changing an EXISTING method's behavior, STOP and report it as
a Feature 2 basic defect — do not silently alter the contract.

Do not add transaction policy (Begin/Commit/Rollback) — still Feature 6.
Do not add MVCC version management — still Feature 6.
Do not add a value/row codec, schema, operations, or SQL — later features.
Do not add a shared Engine interface (engine.go).
Do not make the kv package import key/config/logger/lifecycle/runtime.
Do not auto-repair data; Check is validation/visibility only.

---

Feature Name
sql_engine KVStore Hardening

Plan file:
feature2_enterprise.md

Target packages:
internal/engine/sql/kv        (extend: Pebble backend, new APIs, compliance suite)
internal/config               (extend [sql_engine] with backend options)

---

Required Starting State
This plan starts only after feature2.md is completed and verified.
Before starting, the project must already have and pass:
internal/engine/sql/kv/       (KVStore + Batch on bbolt, Feature 2 basic)
internal/engine/sql/key/      (Feature 1)
internal/config/              (with SQLConfig{DataDir, Backend} + [sql_engine])
internal/logger/ internal/lifecycle/ internal/runtime/
cmd/plomvix/main.go

And must pass:
go build ./...
go test ./...
go test -race ./...

The existing kv package must expose: KVStore (Name/Open/Close/Get/Set/Delete/
Scan/NewBatch), Batch (Set/Delete/Commit/Reset), NewBBolt(name, path), and the
sentinels ErrNotOpen/ErrAlreadyOpen/ErrClosed/ErrEmptyKey/ErrNilCallback, with
the three-state machine and sync.RWMutex state locking.
If this is not true, STOP and report that feature2.md is incomplete.

---

Go Version Requirement
Go 1.22 or later.
Allowed new third-party dependency in this plan:
- github.com/cockroachdb/pebble   (the enterprise backend)
Already present (do not bump/re-add): go.etcd.io/bbolt,
github.com/pelletier/go-toml/v2.
Record exact resolved versions in go.mod/go.sum and report them.
Do not use t.Chdir; use t.TempDir() for on-disk databases.

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
Existing KVStore method signatures and BEHAVIOR are FROZEN.
The interface is EXPANDED with new methods; this is a deliberate breaking
  interface change handled within this plan. Every new method must be
  implemented by BOTH backends (bbolt and Pebble) so the package always compiles
  and parity holds — no backend-only methods, no leaving bbolt non-compiling.
Get/Scan/ScanReverse/Snapshot reads return COPIES (same guarantee as basic).
No panic recovery in the storage path; invalid input returns an error.
Diagnostics (Check) never mutate data.
Do not import outside the allowed dependency list.

---

Additive API To Produce (extends the existing kv package)

  // ---- extended KVStore interface (additive methods) ----
  // The existing methods are unchanged. Add:

  // ScanReverse iterates the SAME half-open range as Scan, [start, end)
  // (i.e. start <= key < end), but visits matching keys in DESCENDING order
  // (largest first). Lower bound = start (inclusive), upper bound = end
  // (exclusive). Iteration begins just below end and proceeds down to start.
  // start==nil means no lower bound (down to the first key); end==nil means no
  // upper bound (begin at the very last key). Same copy + nil-callback
  // (ErrNilCallback) + context semantics as Scan.
  // Worked examples (keys a,b,c,d,e present):
  //   ScanReverse(b, d)  visits: c, b        (d excluded, b included)
  //   ScanReverse(nil, c) visits: b, a       (down to first key)
  //   ScanReverse(c, nil) visits: e, d, c    (from last key down to c)
  //   ScanReverse(b, b)  visits: nothing     (empty range)
  ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error

  // NewSnapshot returns a consistent read-only view of the store as of the call
  // time. Reads through the snapshot are unaffected by later writes. The caller
  // MUST Close the snapshot to release resources.
  // State rules: canceled ctx -> ctx.Err(); never-opened store -> ErrNotOpen;
  // closed store -> ErrClosed (no new snapshots after the store closes).
  // Parent-close interaction (MANDATORY, backend-uniform): while ANY snapshot is
  // open, the parent store's Close() returns ErrSnapshotActive and the store
  // REMAINS Open. Callers must Close all snapshots before closing the store.
  // This avoids closing a bbolt DB out from under an open read txn (unsafe/
  // blocking) and makes both backends behave identically.
  NewSnapshot(ctx context.Context) (Snapshot, error)

  // Stats returns operational counters/sizes for visibility.
  Stats(ctx context.Context) (Stats, error)

  // Check validates internal consistency without mutating data (e.g. opens a
  // read view, verifies the bucket/keyspace is readable and iterable). Returns
  // an error describing any problem found. No auto-repair.
  Check(ctx context.Context) error

  // ---- Snapshot ----
  type Snapshot interface {
      Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
      Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
      ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
      // Close releases the snapshot. First call returns nil; every subsequent
      // call returns ErrSnapshotClosed. The closed flag MUST be mutex-protected.
      Close() error
  }
  // After Close, snapshot Get/Scan/ScanReverse return ErrSnapshotClosed.
  // Snapshot reads honor ctx (canceled -> ctx.Err()) and return copies.

  // ---- Stats ----
  type Stats struct {
      Backend    string // "bbolt" or "pebble"
      KeyCount   int64  // EXACT key count: implementations iterate and count for
                        // BOTH backends (deterministic; optimize later if needed)
      SizeBytes  int64  // on-disk size estimate (may be approximate; document)
      ReadOnly   bool
      SyncWrites bool
  }

  // ---- new sentinels ----
  var (
      ErrReadOnly       = errors.New("kv: store is read-only")
      ErrSnapshotClosed = errors.New("kv: snapshot is closed")
      ErrSnapshotActive = errors.New("kv: snapshot is active")
      ErrUnknownBackend = errors.New("kv: unknown backend")
  )

  // ---- backend construction ----
  // NewBBolt(name, path) keeps its Feature-2 signature for backward compat and
  // now delegates to an unexported options-aware constructor with defaults:
  //   func NewBBolt(name, path string) KVStore {
  //       return newBBoltWithOptions(name, path,
  //           Options{Backend:"bbolt", Path:path, SyncWrites:true})
  //   }
  // unexported, applies ReadOnly/SyncWrites to the bbolt store:
  func newBBoltWithOptions(name, path string, opts Options) KVStore
  // Pebble constructor (options-aware):
  func NewPebble(name, path string, opts Options) (KVStore, error)
  // Config-driven factory dispatching by opts.Backend:
  func New(name string, opts Options) (KVStore, error)
  // where Options carries backend selection + safe options (see below).
  // New/newBBoltWithOptions are how enterprise options (ReadOnly, SyncWrites)
  // reach the bbolt store; Feature 2 callers using NewBBolt are unaffected.

  type Options struct {
      Backend    string // "bbolt" | "pebble"
      Path       string // db file (bbolt) or directory (pebble)
      SyncWrites bool   // fsync on commit (durability vs speed)
      ReadOnly   bool   // reject writes with ErrReadOnly
      // pebble-only (ignored by bbolt):
      CacheSizeMB  int
      MaxOpenFiles int
  }

Notes on options:
- SyncWrites=true: bbolt keeps default sync (DB.NoSync=false); Pebble commits
  with Sync. SyncWrites=false: bbolt sets NoSync=true; Pebble uses NoSync write
  options. Document the durability tradeoff.
- ReadOnly=true: Set, Delete, and Batch.Commit (with any op) return ErrReadOnly;
  reads still work. CRITICAL open semantics: a read-only store REQUIRES the data
  to already exist — it must NOT create a new db/dir. If the bbolt file (or
  Pebble directory) does not exist, Open in read-only mode returns an error
  (do not MkdirAll, do not create the file). bbolt: open with ReadOnly:true
  option (fails if the file is absent). Pebble: open with ReadOnly:true; the
  directory must already contain a VALID Pebble database, not merely exist — if
  the directory is missing, empty, or not a valid Pebble store, Open returns the
  backend's error and the store remains NeverOpened (no creation, no half-open).
  In read-only mode, Open does NOT call os.MkdirAll for either backend.
- bbolt ignores CacheSizeMB/MaxOpenFiles.

---

Tasks (do in order, one at a time)

Task 1 — Define the final extended interface + bbolt stubs + compliance suite
This task makes the package compile end-to-end against the FINAL contract before
any new behavior is implemented, avoiding a mid-plan compile break.
Step A — expand the interface NOW to its final shape: add ScanReverse,
NewSnapshot, Stats, Check to the KVStore interface; add the Snapshot interface,
the Stats struct, the Options struct, and the new sentinels (ErrReadOnly,
ErrSnapshotClosed, ErrSnapshotActive, ErrUnknownBackend) exactly as in the
Additive API section.
Step B — add TEMPORARY bbolt stub methods so bbolt still satisfies KVStore and
the package compiles: each stub returns a clear "not implemented yet" error
(e.g. errors.New("kv: ScanReverse not implemented")). Do NOT implement real
behavior here — later tasks (4,5,6,7) replace each stub. The existing
Name/Open/Close/Get/Set/Delete/Scan/NewBatch behavior is untouched.
Step C — create internal/engine/sql/kv/compliance_test.go, package kv (a normal
in-package test file used only by tests within the kv package; no export/testkit
complexity needed). It exposes within the package:
  func RunKVStoreComplianceTests(t *testing.T, factory func(t *testing.T, path string) KVStore)
The factory returns a FRESH, not-yet-open store bound to the GIVEN path. The
SUITE owns the temp path (it calls t.TempDir() and derives the db path), so it
can construct TWO stores against the SAME path to test persistence (write+Close
on store A, then a fresh store B on the same path reads the data). Path meaning
differs by backend: bbolt path is a file (<dir>/sql.db), Pebble path is a
directory; each backend's factory interprets `path` accordingly while the suite
passes a stable path per logical store.
The suite covers the FULL existing (frozen) contract and is RUN against bbolt
now. It must NOT yet assert the stubbed methods (those assertions are added in
the tasks that implement them). The suite must cover:
- Open/Close state machine (NeverOpened->ErrNotOpen, Closed->ErrClosed,
  double-open->ErrAlreadyOpen, Open-after-Close->ErrClosed)
- Get/Set/Delete incl. empty-key->ErrEmptyKey, missing key found=false, nil
  value semantics, copy-safety (mutating returned slice doesn't change store)
- Scan: ordered, half-open [start,end), copies, nil-callback->ErrNilCallback,
  fn-error propagation
- Batch: atomic apply, uncommitted no-op, Reset, double-commit no-op, empty-key
  op->ErrEmptyKey applies nothing, commit-after-close->ErrClosed
- context cancellation: canceled ctx before a data op returns ctx.Err()
- persistence: data survives Close + fresh store + Open on same path
Run the suite against bbolt (factory uses NewBBolt) to PROVE it encodes the real
Feature 2 contract and passes. Keep the original bbolt tests or fold them into
the suite. After this task: the package compiles with the FINAL interface, bbolt
satisfies it (real methods + temporary stubs), and the compliance suite passes
against bbolt for all NON-stubbed behavior.
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/
Report Graphify availability.

Task 2 — Add Pebble dependency and NewPebble backend
Add github.com/cockroachdb/pebble; record exact version.
Create internal/engine/sql/kv/pebble.go implementing the FULL (now-extended)
KVStore interface on Pebble. For the core methods (Name/Open/Close/Get/Set/
Delete/Scan/NewBatch) implement real behavior matching the Feature 2 contract
EXACTLY. For ScanReverse/NewSnapshot/Stats/Check, add the SAME temporary
"not implemented yet" stubs as bbolt for now (later tasks 4/5/7 implement both
backends together). This keeps the package compiling with two backends against
one final interface. Core methods to match exactly:
- same three-state machine + sync.RWMutex state locking
- Open creates the data DIRECTORY (Pebble uses a dir, not a single file) via
  os.MkdirAll; same open-failure cleanup (no half-open, no leaked handle)
- Get/Scan return copies; empty-key->ErrEmptyKey; nil-callback->ErrNilCallback
- Batch via Pebble's Batch, atomic on Commit, same empty-key/closed semantics
- Close-failure rule: transition to Closed even if close errors
- context checks before each op (and Scan per-row)
NewPebble(name, path string, opts Options) (KVStore, error).
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 3 — Run the compliance suite against Pebble (parity proof)
Add a test that runs RunKVStoreComplianceTests with a Pebble factory (builds a Pebble store at the given path). Both
backends must pass the IDENTICAL suite. Fix Pebble (never the suite, unless the
suite encodes a bbolt-specific assumption — in which case generalize the suite
and re-run against bbolt too). Add a focused parity test: write the same set of
Feature-1-encoded keys to a bbolt store and a Pebble store, full-scan both, and
assert identical visitation order and identical values.
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 4 — Implement ScanReverse on both backends (replace stubs)
ScanReverse is ALREADY in the interface (Task 1). REPLACE the temporary
"not implemented" stubs on BOTH backends with real implementations: bbolt
(Cursor.Last + Prev), Pebble (iterator SeekLT/Prev), descending order over
half-open [start,end) per the worked examples. Same copy + nil-callback
(ErrNilCallback) + context semantics as Scan.
Add to the compliance suite: ScanReverse visits the exact reverse of Scan over
the same range; bounds are correct (end excluded, start included); empty range
visits nothing; nil callback -> ErrNilCallback.
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 5 — Implement Snapshot / NewSnapshot on both backends (replace stubs)
NewSnapshot and the Snapshot type are ALREADY declared (Task 1). REPLACE the
temporary NewSnapshot stubs on BOTH backends with real implementations.
bbolt: the snapshot is a manually-managed read transaction obtained via
db.Begin(false) (NOT db.View — db.View closes the txn when its callback returns
and therefore cannot back a long-lived snapshot). Snapshot.Get/Scan/ScanReverse
read through this held tx; Snapshot.Close() calls tx.Rollback() to release it
(first Close nil, subsequent ErrSnapshotClosed, mutex-protected). Because this
read tx is held open, the single bbolt writer blocks until the snapshot closes
(see writer-blocking policy).
Pebble: use pebble.NewSnapshot; Snapshot reads go through it; Close releases it
(same double-close rule). Pebble writers are NOT blocked by an open snapshot.
The store MUST maintain an open-snapshot counter: NewSnapshot increments it,
Snapshot.Close decrements it, and store.Close returns ErrSnapshotActive whenever
the counter is > 0 (store stays Open).

LOCK DISCIPLINE (MANDATORY — prevents BOTH deadlock AND use-after-close):
The bbolt snapshot test intentionally starts a write while a snapshot is open and
expects the write to BLOCK (inside bbolt) until the snapshot closes. The safe
design relies on ONE key separation: the open-snapshot counter is synchronized
INDEPENDENTLY of the store's main state RWMutex. With that separation, the
following rules are all simultaneously safe:

- Open and Close take the store's main WRITE lock.
- Get/Set/Delete/Scan/Batch.Commit take the store's main READ lock and HOLD it
  ACROSS the entire backend transaction. This is REQUIRED so that Close() (which
  needs the write lock) cannot close the DB while an operation is still running
  on the db pointer — it prevents a use-after-close race. (Do NOT release the
  main lock before the backend call; an earlier draft suggested that and it is
  WRONG — it reintroduces a use-after-close race.)
- Snapshot.Close MUST NOT acquire the store's main lock. The open-snapshot
  counter uses its OWN mutex (or an atomic). Therefore Snapshot.Close can
  decrement the counter and release the bbolt read txn (tx.Rollback) EVEN WHILE a
  writer is blocked inside bbolt holding the store's main READ lock. This is what
  breaks the would-be deadlock: the blocked writer is unblocked by Snapshot.Close
  without Snapshot.Close ever needing the lock the writer holds.
- Close() checks the INDEPENDENT snapshot counter: if > 0 it returns
  ErrSnapshotActive (store stays Open) without taking any backend action.

Why this is safe (the invariant):
The original deadlock was NOT caused by holding the read lock across the backend
op — it was caused by Snapshot.Close contending for the SAME main lock the
blocked writer held. Making the snapshot counter fully independent removes that
contention, so the read lock can be safely held across the backend op (closing
the use-after-close race) AND no deadlock occurs.

The bbolt-only snapshot test MUST prove the full sequence with NO deadlock:
a write starts and blocks while the snapshot is open (writer holds the main read
lock, blocked inside bbolt) -> Snapshot.Close() returns (using only the
independent counter sync) -> the blocked write then completes (assert via
timeout/ordering).
Semantics: writes after NewSnapshot are NOT visible through the snapshot;
operations on a closed snapshot -> ErrSnapshotClosed; snapshot reads return
copies.
Snapshot tests are split into a COMMON contract (in RunKVStoreComplianceTests,
runs for both backends) and BACKEND-SPECIFIC tests (separate, not in the shared
suite), because writer-vs-snapshot behavior legitimately differs.

COMMON (both backends, in the compliance suite):
- snapshot reads return the expected data and return COPIES
- ops on a closed snapshot -> ErrSnapshotClosed
- FIRST Close returns nil; every subsequent Close -> ErrSnapshotClosed
  (closed flag mutex-protected)
- NewSnapshot on a never-opened store -> ErrNotOpen, on a closed store ->
  ErrClosed, canceled ctx -> ctx.Err()
- while a snapshot is open, store.Close() -> ErrSnapshotActive and the store
  stays Open; after the snapshot is Closed, store.Close() succeeds
The common test MUST NOT attempt a write-through while a snapshot is open (that
behavior differs by backend and is covered separately).

PEBBLE-ONLY (separate test, not in the shared suite):
- take snapshot, perform a store write (succeeds immediately, NOT blocked),
  snapshot still reads the OLD data, and a fresh non-snapshot Get reads the NEW
  data.

BBOLT-ONLY (separate test, not in the shared suite):
- take snapshot; a store write issued from another goroutine BLOCKS while the
  snapshot is open; after Snapshot.Close() the write completes. Use a timeout to
  assert the write is blocked (e.g. it does not complete within a short window
  while the snapshot is open, then completes promptly after Close). Do not
  assert immediate write success.
bbolt writer-blocking policy (MANDATORY, document in code + docs): a bbolt read
txn (the snapshot) held open BLOCKS the single bbolt writer for its entire
lifetime. The real risk is a WRITE blocking behind a held snapshot, not reads.
Policy: snapshot users MUST Close the snapshot before issuing writes that need to
make progress. Tests:
- concurrent snapshot READS do not deadlock (reads are fine alongside a snapshot)
- DO NOT assert write-through while holding a bbolt snapshot as if it succeeds;
  if a test exercises a write while a bbolt snapshot is open, it MUST use a
  timeout and assert the documented behavior (the write blocks until the
  snapshot closes), not silent success. Pebble snapshots do NOT block writers;
  a Pebble test may show a write proceeding while a snapshot is open, proving the
  backend difference. Document this divergence explicitly (it is a real,
  acceptable backend behavioral difference OUTSIDE the frozen core contract).
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 6 — Backend options + config (sync_writes, read_only, pebble opts)
Extend internal/config SQLConfig (additively; match existing style, strict
decode) with:
  SyncWrites   bool `toml:"sync_writes"`    default true
  ReadOnly     bool `toml:"read_only"`      default false
  CacheSizeMB  int  `toml:"cache_size_mb"`  default 0 (0 = backend default)
  MaxOpenFiles int  `toml:"max_open_files"` default 0 (0 = backend default)
Extend Default() and Validate(): Backend now accepts "bbolt" OR "pebble";
cache/files must be non-negative. The config package MUST NOT import kv. Config
validation returns its OWN plain error string for an unknown backend, exactly:
  "sql_engine backend must be bbolt or pebble"
and for negatives e.g. "sql_engine cache_size_mb must be >= 0". kv.ErrUnknownBackend
is a SEPARATE concern that lives ONLY inside kv.New (defense-in-depth for callers
that bypass config). Update config.toml and config.example.toml with the new keys
(commented sensible defaults). Map config -> kv.Options at the wiring layer (not
inside config; config stays free of kv).
Implement kv.New(name, opts) factory in the kv package: for opts.Backend
"bbolt" call newBBoltWithOptions(name, opts.Path, opts); for "pebble" call
NewPebble(name, opts.Path, opts); otherwise return ErrUnknownBackend. Refactor
NewBBolt(name, path) to delegate to newBBoltWithOptions with default options
(SyncWrites=true, ReadOnly=false) so Feature 2 callers are unaffected.
newBBoltWithOptions applies SyncWrites (bbolt DB.NoSync = !SyncWrites) and
ReadOnly (bbolt Options.ReadOnly; read-only Open must not create the file).
Implement ReadOnly enforcement: when opts.ReadOnly, Set and Delete return
ErrReadOnly; Batch.Commit returns ErrReadOnly ONLY if the batch has >=1 recorded
op — an EMPTY batch Commit in read-only mode is a no-op returning nil. Reads
always allowed. Implement SyncWrites mapping for both backends.
Add to compliance suite (parameterized): run a read-only variant asserting
Set/Delete -> ErrReadOnly, a non-empty Batch.Commit -> ErrReadOnly (applies
nothing), an EMPTY Batch.Commit -> nil (no-op), and reads succeed.
Verification:
  go test ./internal/config/
  go test ./internal/engine/sql/kv/
  go test ./...
  go test -race ./...

Task 7 — Stats and Check diagnostics on both backends
REPLACE the temporary Stats/Check stubs (declared in Task 1) with real
implementations on BOTH backends.
Stats: KeyCount is EXACT for both backends — iterate the keyspace and count
(deterministic; do not use approximate estimators). SizeBytes is an on-disk size
estimate and MAY be approximate (document the method used). Set Backend,
ReadOnly, SyncWrites accurately.
Check: read-only consistency validation — open a read view, iterate the
keyspace, confirm readable; for bbolt verify the fixed bucket exists; no
mutation.
Tests: Stats.KeyCount EXACTLY equals the number of inserted keys for BOTH
backends; correct Backend string and flags; Check returns nil on a healthy store
and does not modify data (verify via before/after full-scan equality).
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 8 — Benchmarks (bbolt vs Pebble)
Add benchmarks (Benchmark*) covering, for BOTH backends:
- point write (Set), point read (Get)
- batch write (N ops per Commit)
- full scan, range scan
Report ns/op, allocs/op, and MB/s where meaningful in the task report. Use a
shared benchmark body parameterized by backend factory. Benchmarks are
INFORMATIONAL ONLY — not a pass/fail gate.
Verification:
  go test -bench=. -benchmem ./internal/engine/sql/kv/
  go test ./internal/engine/sql/kv/

Task 9 — Crash-safety / durability tests (practical)
Add tests (no unsafe process-kill):
- write a batch, Commit, Close; fresh store + Open same path; committed data
  persists (both backends)
- build a batch but do NOT commit, Close; reopen; the uncommitted data is absent
- with SyncWrites=true, committed data persists across Close/reopen
Keep these as clean Close/reopen cycles; do not attempt kill -9 style tests yet.
Verification:
  go test ./internal/engine/sql/kv/
  go test -race ./internal/engine/sql/kv/

Task 10 — Large dataset scan tests
Insert keys (test-only import of the key package) in randomized order into each
backend, in two tiers:
- 10k tier: runs in normal mode for both backends.
- 100k tier: runs ONLY when the environment variable PLOMVIX_KV_LARGE_TESTS=1
  is set; otherwise t.Skip with a clear message. NEVER required in normal or
  -race verification. The 10k tier is sufficient for the -race pass and runs
  without the env var.
Assertions (both tiers):
- full scan visits ascending order; ScanReverse visits descending
- a bounded range returns exactly the expected subset
- memory safety (returned slices are copies; mutating them is harmless)
- reasonable runtime (no hard threshold; informational timing in report)
Verification:
  go test ./internal/engine/sql/kv/                            (10k; 100k skipped)
  go test -race ./internal/engine/sql/kv/                      (10k under race)
  PLOMVIX_KV_LARGE_TESTS=1 go test ./internal/engine/sql/kv/   (optional 100k tier)

Task 11 — Documentation
Create docs/sql_engine_kv_enterprise.md covering:
- the backend compatibility contract (the compliance suite is the authority;
  both backends MUST pass it identically)
- bbolt vs Pebble: model (B+tree single-writer vs LSM), when each is preferred,
  the single-writer/snapshot caveat for bbolt
- snapshot behavior and the short-lived-snapshot guidance
- reverse scans
- backend options (sync_writes, read_only, cache_size_mb, max_open_files) and
  their durability/performance tradeoffs
- Stats/Check diagnostics (and that Check is validation-only, no repair)
- migration rules: switching backends requires data migration (the on-disk
  formats differ); document that changing `backend` does not convert existing
  data and the intended migration path is export/import (future feature).
Update any docs index to reference it.
Verification:
  go build ./...
  go test ./...
  go test -race ./...

---

Definition Of Done
shared RunKVStoreComplianceTests suite exists and PASSES identically for bbolt
  and Pebble
Pebble backend implements the full extended KVStore contract (state machine,
  copies, errors, batch, context, locking, open/close-failure rules)
ScanReverse implemented on both backends; reverse of Scan proven
Snapshot/NewSnapshot implemented on both; isolation + ErrSnapshotClosed proven;
  bbolt snapshot non-deadlock test passes
config extended (sync_writes, read_only, cache_size_mb, max_open_files); Backend
  accepts bbolt|pebble; strict decode preserved; config.toml/example updated
kv.New factory dispatches by backend; ErrUnknownBackend on unknown
ReadOnly enforced (writes -> ErrReadOnly); SyncWrites mapped on both backends
Stats and Check implemented on both; Check does not mutate (proven)
benchmarks run for both backends (informational; ns/op, allocs/op, MB/s reported)
crash-safety tests (clean Close/reopen) pass for both backends
large-dataset scan/order/range tests pass (10k normal; 100k only under PLOMVIX_KV_LARGE_TESTS=1)
docs/sql_engine_kv_enterprise.md exists and matches the code
existing Feature 2 method signatures + behavior UNCHANGED (the interface is
  expanded with new methods; existing methods are not altered); package always
  compiles (bbolt stubs added in Task 1, filled by later tasks)
Stats.KeyCount is EXACT for both backends (deterministic count)
100k dataset tier gated ONLY by PLOMVIX_KV_LARGE_TESTS=1; not required under normal or -race runs
read-only Open requires existing VALID data (bbolt: existing file; pebble:
  existing valid pebble dir), never creates/MkdirAll, and a failed read-only Open
  leaves the store NeverOpened
read-only: Set/Delete and non-empty Batch.Commit -> ErrReadOnly; empty
  Batch.Commit -> nil
Snapshot: first Close nil, subsequent -> ErrSnapshotClosed (mutex-guarded); ops
  after Close -> ErrSnapshotClosed; NewSnapshot honors store state
  (ErrNotOpen/ErrClosed) + ctx
store.Close while any snapshot is open -> ErrSnapshotActive (store stays Open);
  open-snapshot counter uses INDEPENDENT sync (own mutex/atomic), NOT the store's
  main RWMutex; data ops HOLD the main read lock across the backend txn (so
  Close cannot close the DB mid-op — no use-after-close); Snapshot.Close never
  takes the main lock; bbolt write-blocked-behind-snapshot test completes with
  NO deadlock; Close succeeds after all snapshots closed
100k dataset tier runs only under PLOMVIX_KV_LARGE_TESTS=1; never required in
  normal or -race runs
compliance_test.go is a plain in-package (package kv) test file; compliance
  factory shape is func(t *testing.T, path string) KVStore so persistence is
  testable (two stores on the same path)
NewBBolt(name,path) preserved (delegates to newBBoltWithOptions defaults);
  enterprise options reach bbolt via newBBoltWithOptions and Pebble via
  NewPebble; kv.New dispatches by backend
ErrSnapshotActive is declared in Task 1 with the other new sentinels
config package does NOT import kv; unknown-backend error is config's own string
go build ./... ; go test ./... ; go test -race ./... all pass
exact resolved pebble version recorded and reported

---

Out Of Scope (do not implement in this plan)
transaction policy Begin/Commit/Rollback (Feature 6)
MVCC version management (Feature 6)
auto-repair of corrupted data (Check is validation only)
backend data migration/conversion tooling (future)
value/row codec, schema, operations, SQL (later features)
shared Engine interface / engine.go
process-kill / power-loss crash injection (not yet)
networking