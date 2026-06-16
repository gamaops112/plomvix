feature1_enterprise.md

Plomvix sql_engine — Feature 1 Enterprise: Key Encoding Hardening

Purpose
Harden the completed Feature 1 key-encoding layer into an enterprise-grade,
provably-stable encoding. This plan adds correctness guarantees, stricter
validation, exhaustive tests, golden vectors, property-style ordering proofs,
immutability proofs, benchmarks, and format-stability documentation.

This is hardening only.
It MUST NOT change the on-disk key format defined in feature1.md.
If any hardening step appears to require a format change, STOP and report it as
a defect in the basic plan — do not silently change the format here. The whole
value of this tier is proving the EXISTING format is correct and stable.

Do not add a KVStore.
Do not add bbolt or Pebble.
Do not add a value/row codec.
Do not add schema or catalog logic.
Do not add operations.
Do not add transactions or MVCC version management.
Do not add a shared Engine interface.
Do not add networking or SQL parsing.

---

Feature Name
sql_engine Key Encoding Hardening

Plan file:
feature1_enterprise.md

Target package (same package; no new packages):
internal/engine/sql/key

---

Required Starting State
This plan starts only after feature1.md is completed and verified.
Before starting, the project must already have:
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
docs/sql_engine_key.md

And must pass:
go build ./...
go test ./...
go test -race ./...

The basic key package must already expose its full Public API: TagTableData/
TagMetadata/TagIndex, Kind constants, Value + constructors + accessors + Equal,
EncodeTableRowKey, DecodeTableRowKey(b, expectedKinds), TablePrefix, and all
sentinel errors.
If this starting state is not true, stop and report that feature1.md is
incomplete.

---

Go Version Requirement
Go 1.22 or later. Standard library only. No third-party dependencies.
Do not use t.Chdir.

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
Keep changes focused on hardening.
Do not change the on-disk key format.
Do not add future placeholders.
Do not add panic recovery in the encoding path.
Do not import anything outside the standard library.
Any NEW public API added here must be additive and must not alter existing
behavior or signatures.

---

Enterprise Hardening Goals
canonical-encoding validation
stricter malformed-key detection and tests
prefix/range helper(s) with tests
deterministic golden vectors (committed, format-locking)
property-style ordering tests
API immutability / copy-safety proofs
benchmarks
format-stability and backward-compatibility documentation and guard test

---

Non-Goals
Do not implement:
storage, KVStore, backends
value/row codec
schema, catalog, tableID assignment
operations, transactions, MVCC management
indexes or metadata encoding (tags remain reserved)
SQL, networking
any change to the byte format

---

New / Extended Public API (additive only)
Add a canonical-form validator and range helpers. These must NOT change
encoding output; they only validate and assist range scans.

  // IsCanonical reports whether b is in canonical encoded form: it decodes
  // cleanly with expectedKinds AND re-encoding the decoded result yields bytes
  // identical to b. Non-canonical or malformed input returns false (and the
  // reason via the error).
  func IsCanonical(b []byte, expectedKinds []Kind) (bool, error)

  // PrefixEnd returns the smallest byte slice strictly greater than every key
  // having prefix p (the standard "increment last non-0xFF byte" rule), or nil
  // if p is empty or all 0xFF (meaning unbounded upper end). Used to turn a
  // prefix into a [start, end) scan range.
  func PrefixEnd(p []byte) []byte

  // TableRange returns [start, end) bounding all rows of a table:
  // start = TablePrefix(tableID), end = PrefixEnd(start).
  func TableRange(tableID uint64) (start, end []byte)

Add sentinel error(s) only if needed for canonical checks:
  ErrNotCanonical = errors.New("key: non-canonical encoding")

---

Tasks (do in order, one at a time)

Task 1 — Canonical encoding validation
Implement IsCanonical(b, expectedKinds): decode b with expectedKinds; if decode
fails, return (false, decodeErr). Otherwise re-encode the decoded
(tableID, pk, version) via EncodeTableRowKey and compare to b with bytes.Equal;
return (true, nil) if identical, else (false, ErrNotCanonical).
Rationale: the current format defines exactly ONE valid byte representation per
logical key, so IsCanonical is primarily a self-consistency / format-drift
guard — it will catch any FUTURE change that introduces an alternate or
non-minimal encoding. Do NOT require constructing a non-canonical-but-decodable
input as a test fixture: under the current format such an input likely does not
exist, and fabricating one would be meaningless.
Tests:
- every key from EncodeTableRowKey reports IsCanonical == true.
- malformed inputs (truncated, bad tag, bad escape) return (false, err) with the
  underlying decode error propagated via errors.Is.
- only IF a genuine alternate decodable encoding actually exists under the
  current format, add a case proving it reports false; otherwise add a code
  comment stating that no alternate encoding is constructible under this format.
Verification:
  go test ./internal/engine/sql/key/

Task 2 — Stricter malformed-key tests
Add an exhaustive table-driven malformed-input suite asserting EXACT sentinels
via errors.Is, covering at minimum:
- empty input -> ErrEmptyKey
- wrong tag (0x00, 0x02, 0x03, 0xFF) -> ErrBadTag
- expectedKinds nil and empty -> ErrNoPKColumns
- tableID truncated at each length 0..7 -> ErrTruncated
- type tag mismatch for every (expected,actual) kind pair -> ErrKindMismatch
- unknown type tag (e.g. 0x99) -> ErrBadTypeTag
- variable-length field: no terminator before end -> ErrTruncated
- variable-length field: trailing lone 0x00 -> ErrTruncated
- variable-length field: invalid escape 0x00 0x02 (and other non-FF/01) -> ErrBadField
- version truncated at each length 0..7 -> ErrTruncated
- trailing bytes after version (1, 8, 100 extra) -> ErrTrailingBytes
Verification:
  go test ./internal/engine/sql/key/

Task 3 — Prefix/range helpers and tests
Implement PrefixEnd and TableRange per the API above.
Tests:
- every key produced by EncodeTableRowKey for tableID T satisfies
  start <= key < end where (start,end)=TableRange(T).
- a key for tableID T+1 is >= end of table T's range.
- PrefixEnd of an all-0xFF prefix returns nil (unbounded).
- PrefixEnd correctness: no key with the prefix is >= PrefixEnd, and the prefix
  start itself is < PrefixEnd.
Verification:
  go test ./internal/engine/sql/key/

Task 4 — Deterministic golden vectors (format lock)
Add a committed `goldenCases` table in the test file: a fixed list of
(tableID, pk kinds+values, version) inputs, each paired with its expected
encoded bytes written as a LITERAL hex string constant.
CRITICAL methodology rule (this is the whole point of golden vectors):
- The expected hex MUST be authored by hand from the format spec in
  docs/sql_engine_key.md / feature1.md — byte by byte.
- The expected bytes MUST NOT be produced by any helper, by calling the encoder,
  or by any computation that shares logic with the implementation. If the
  expected value is computed from the code, the test only compares the code to
  itself and proves nothing.
- The test does exactly: got := EncodeTableRowKey(input); want := hexDecode(the
  hand-written literal); assert bytes.Equal(got, want).
Include cases: single int64 PK, single string PK, composite (string,int64),
bytes-with-embedded-zero PK, uint64 PK, bool PK, null PK, min int64, max int64,
version 0, and version max (^0).
These vectors LOCK the on-disk format: if any future change alters output, this
test fails — the intended early warning. Decode-side lock: also assert
DecodeTableRowKey(want, kinds) returns the original input (round-trips the
hand-written bytes back).
Verification:
  go test ./internal/engine/sql/key/

Task 5 — Property-style ordering tests
Add randomized property tests (math/rand, FIXED seed) asserting invariants over
many generated inputs:
- ordering matches logical order: for random pairs of single-column values of
  the same kind, bytes.Compare(encode(a),encode(b)) has the same sign as the
  logical comparison of a and b (define logical order per kind; for bytes/string
  use bytes/lex order; for int64 numeric; etc.).
- composite monotonicity: appending/altering a later column never inverts the
  order decided by an earlier differing column.
- version inversion: for equal tableID+pk, higher version sorts earlier.
- round-trip: encode then decode (matching kinds) returns Equal values, for all
  generated inputs.
Use enough iterations to be meaningful but keep runtime reasonable.
Verification:
  go test ./internal/engine/sql/key/
  go test -race ./internal/engine/sql/key/

Task 6 — API immutability / copy-safety proofs
Add tests proving no aliasing through the public API:
- mutating the slice passed to Bytes() after construction does not change the
  Value (verified via Equal and via encode output stability).
- mutating the slice returned by AsBytes() does not change the Value.
- the slice returned by EncodeTableRowKey is safe for the caller to mutate
  without affecting subsequent encodes (each call returns a fresh slice).
- TablePrefix / TableRange return fresh slices on each call (mutating one
  result does not affect another).
Verification:
  go test ./internal/engine/sql/key/

Task 7 — Benchmarks
Add Go benchmarks (Benchmark* in the test file):
- BenchmarkEncodeTableRowKey for: single int64 PK, single short string PK,
  composite (string,int64) PK.
- BenchmarkDecodeTableRowKey for the same shapes.
- BenchmarkEncodeValue / decode for string with and without embedded zeros.
Benchmarks must compile and run via `go test -bench`. Record indicative
ns/op and allocs/op in the task report (informational; no hard threshold).
Verification:
  go test -bench=. -benchmem ./internal/engine/sql/key/
  go test ./internal/engine/sql/key/

Task 8 — Format-stability documentation and guard
Update docs/sql_engine_key.md with a "Format Stability" section stating:
- the key format is FROZEN; the golden vectors in the test file are the
  authoritative lock.
- any intentional format change requires a new keyspace tag or an explicit
  versioned-format migration plan; the existing 0x01 layout must never change
  meaning.
- the reserved tags 0x02 (metadata) and 0x03 (index) remain reserved.
Add a short doc note explaining backward-compatibility intent: keys written by
this version must remain decodable by future versions of the 0x01 format.
Verification:
  go build ./...
  go test ./...
  go test -race ./...
  (benchmarks are informational only — run them via the Task 7 command when
  desired; they are NOT a pass/fail gate for this feature)

---

Definition Of Done
IsCanonical implemented and tested (canonical true, alternates/malformed false)
exhaustive malformed-key suite passes with exact errors.Is assertions
PrefixEnd / TableRange implemented and tested for correctness and bounding
committed golden vectors lock the format and pass
property-style ordering + round-trip tests pass (with race)
immutability / copy-safety tests pass
benchmarks compile and run via the Task 7 command (informational; ns/op +
  allocs/op reported in the task report; NOT a pass/fail gate)
docs/sql_engine_key.md has a Format Stability section
NO change was made to the on-disk key format
all new public API is additive (no existing signature or behavior changed)
go build ./... passes
go test ./... passes
go test -race ./... passes
no third-party dependencies added

---

Out Of Scope (do not implement in this plan)
any change to the byte format
shared Engine interface
KVStore, bbolt, Pebble
value/row codec
schema / catalog-lite / tableID assignment
operations, transactions, MVCC management
secondary index or metadata encoding (tags stay reserved)
SQL parsing, networking