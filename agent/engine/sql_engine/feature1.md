feature1.md

Plomvix sql_engine — Feature 1: Key Encoding

Purpose
Implement the key-encoding layer for the Plomvix sql_engine.
This is the lowest layer of the sql_engine. Every other sql_engine feature
(KVStore scans, value codec, schema, operations, transactions) depends on the
key format defined here.

This layer turns logical table identifiers and primary-key column values into
order-preserving byte keys, and decodes them back. It is pure encoding logic.

This is key-encoding only.
Do not add a KVStore.
Do not add bbolt or Pebble.
Do not add a value/row codec.
Do not add schema or catalog logic.
Do not add operations (Insert/Scan/etc).
Do not add transactions.
Do not add MVCC version-management logic (only the version SLOT in the key).
Do not add a shared Engine interface (no internal/engine/engine.go in this plan).
Do not add networking.
Do not add SQL parsing.

---

Feature Name
sql_engine Key Encoding

Plan file:
feature1.md

Target package (the ONLY package this plan creates):
internal/engine/sql/key

---

Required Starting State
This plan starts only after the foundation is complete and verified.
Before starting this plan, the project must already have:
internal/config/
internal/logger/
internal/lifecycle/
internal/runtime/
cmd/plomvix/main.go

And must pass:
go build ./...
go test ./...
go test -race ./...

The directories internal/engine/ and internal/engine/sql/ may not exist yet.
This plan is allowed to create ONLY:
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
docs/sql_engine_key.md

This plan must NOT create internal/engine/engine.go or any shared Engine
interface. That belongs to a later feature when an engine actually implements
it. Adding it now is a future placeholder and is forbidden.
If the foundation build/test does not pass, stop and report it before starting.

---

Go Version Requirement
Plomvix uses Go 1.22 or later.
Use only the Go standard library.
Do not add any third-party dependency in this plan.
Do not use t.Chdir. (No test in this plan should need filesystem changes — key
encoding is pure and filesystem-free.)

---

Coding Agent
Coding agent: DeepSeek V4 Pro
Tasks must be executed one at a time, in exact order.
Do not proceed to the next task until the current task passes verification.

---

Graphify Rule
For every task:
Search Graphify before starting the task if Graphify is available.
Update Graphify after completing the task if Graphify is available.
If Graphify is unavailable, do not block the task.
Mention Graphify availability in the task report.

---

Global Project Rules
Keep implementation small and focused on encoding.
Do not add future placeholders beyond what this plan specifies.
Do not add panic recovery in the encoding path; invalid input returns an error.
Encoding and decoding must be exact inverses (round-trip safe).
All multi-byte integers use big-endian.
Do not import anything outside the standard library.
Do not make config, logger, lifecycle, or runtime depend on this package.

---

Background: Why This Layout (read before coding)
The sql_engine stores every row as one key/value pair in an ordered key/value
store. The KEY must sort, by raw byte comparison (memcmp order), in the same
order as the logical data. This is what makes range scans correct.

The key has four parts, concatenated in this fixed order:
[ keyspace tag ][ tableID ][ encoded primary-key columns ][ version ]

Part definitions:

1. keyspace tag — 1 byte. Separates categories of keys so they never collide
   and so "scan everything in one category" is a bounded range.
   Reserved values for this plan:
   0x01 = table row data
   0x02 = engine metadata (schema/catalog-lite)  (reserved, NOT used in Feature 1)
   0x03 = secondary index data                    (reserved, NOT used in Feature 1)
   Feature 1 only produces 0x01 keys, but the tag MUST be present so later
   features add 0x02 / 0x03 without changing the format.

2. tableID — 8 bytes, big-endian uint64. Big-endian so byte order equals
   numeric order. (ID assignment is NOT in this plan; this plan only encodes a
   given uint64.)

3. encoded primary-key columns — an ordered sequence of one or more typed
   key-columns, each encoded order-preservingly, concatenated so the
   concatenation is BOTH unambiguous AND order-preserving. Composite
   (multi-column) primary keys MUST be supported from the start.

4. version — 8 bytes. Reserved slot for MVCC. In the Basic tier a single
   version value is used. Encoded so that, within the same tableID+PK, NEWER
   versions sort BEFORE older versions (descending), so "latest" is found first
   on a forward scan. Encoding rule: store bitwise-inverted big-endian uint64
   (encode value v as ^v in big-endian). A larger v then sorts earlier.
   Feature 1 only encodes/decodes this slot; it does NOT select versions.

DECODE REQUIRES KNOWN PK ARITY (important — read carefully)
A row key does NOT store how many PK columns it contains. This is deliberate:
in the real engine the schema for a table always tells us the exact number and
types of PK columns, so storing them in every key would waste space. Therefore
decoding is NEVER done "blind." DecodeTableRowKey takes the expected PK column
KINDS as an argument (supplied by the caller / schema). The decoder:
- decodes exactly len(expectedKinds) PK columns,
- verifies each decoded column's type tag matches the expected Kind,
- then reads exactly the 8-byte version,
- then requires the input to be fully consumed (leftover -> ErrTrailingBytes).
This removes all decode ambiguity: there is no "decode until 8 bytes remain"
heuristic. The count is known, so the boundary between the last PK column and
the version is unambiguous, ErrTrailingBytes is reachable, and a zero-length
expectedKinds is rejected with ErrNoPKColumns.

Order-preserving encoding rules for primary-key column values:

- Each key-column begins with a 1-byte TYPE TAG so decode can verify the type:
  0x10 = null
  0x20 = bool
  0x30 = int64   (signed)
  0x40 = uint64
  0x50 = string  (UTF-8 bytes)
  0x60 = bytes   (raw)
  Tags ascend so that across types: null < bool < int64 < uint64 < string <
  bytes. Cross-type ordering only needs to be deterministic. NOTE: this
  cross-type ordering exists ONLY for raw encoded-key byte comparison. It is
  NOT a decoding mechanism: decode never infers a column's type from its tag,
  it always uses the caller-supplied expectedKinds and verifies the tag matches.

- null: just the tag 0x10, no payload.

- bool: tag 0x20 then 1 byte, 0x00 false, 0x01 true (false < true).

- int64: tag 0x30 then 8 bytes big-endian of (value XOR 0x8000000000000000).
  The sign-bit flip makes negatives sort before positives under unsigned byte
  comparison.

- uint64: tag 0x40 then 8 bytes big-endian, no transformation.

- string / bytes (variable length): tag (0x50 or 0x60) then an
  ESCAPE-AND-TERMINATE encoding:
    For each input byte:
      0x00 is written as 0x00 0xFF   (escape a real zero)
      every other byte is written unchanged
    Terminate the field with 0x00 0x01
  This is Plomvix's order-preserving variable-length scheme.
  Why 0x00 0x01 as terminator (not 0x00 0x00):
  - The only other two-byte sequence starting with 0x00 the encoder emits is
    0x00 0xFF (escaped zero). Since 0x01 != 0xFF, the terminator can never be
    confused with an escaped zero, so decode is unambiguous.
  - Prefix ordering holds: a shorter string is a prefix of a longer one up to
    the terminator; at that position the shorter has 0x00 0x01 and the longer
    has its next real (escaped) byte, and the bytes compare correctly so the
    shorter sorts first.
  Decode reads bytes until it sees 0x00 followed by 0x01 (terminator),
  converting any 0x00 0xFF back into a single 0x00.
  EXACT malformed-field error semantics (no ambiguity):
  - input ends before a terminator is found            -> ErrTruncated
  - a 0x00 is the final byte (nothing follows it)        -> ErrTruncated
  - a 0x00 is followed by a byte that is neither 0xFF
    (escape) nor 0x01 (terminator), e.g. 0x00 0x02      -> ErrBadField

A composite primary key is each key-column encoded as above and concatenated in
column order. Because every field is self-delimiting (fixed width, or
escape-terminated for variable width) AND the decoder knows the expected count
and kinds, the concatenation is unambiguous and order-preserving.

Worked example (informal):
table 7, PK = ("ab", int64 5), version 1
  keyspace tag : 01
  tableID      : 00 00 00 00 00 00 00 07
  pk col 1     : 50 (string) 61 62 00 01            ("ab" + terminator 00 01)
  pk col 2     : 30 (int64)  80 00 00 00 00 00 00 05 (5 with sign bit flipped)
  version      : FF FF FF FF FF FF FF FE            (^1)
Decoding this key requires expectedKinds = [KindString, KindInt64].

---

Public API To Produce
Final package internal/engine/sql/key must expose:

  package key

  import "errors"

  // Keyspace tags.
  const (
      TagTableData byte = 0x01
      TagMetadata  byte = 0x02 // reserved, unused in Feature 1
      TagIndex     byte = 0x03 // reserved, unused in Feature 1
  )

  // Kind enumerates supported key-column types.
  type Kind uint8
  const (
      KindNull Kind = iota
      KindBool
      KindInt64
      KindUint64
      KindString
      KindBytes
  )

  // Value is one primary-key column value. Fields are unexported.
  type Value struct { /* Kind + concrete payload */ }

  // Constructors. Bytes MUST COPY its input so later caller mutation cannot
  // corrupt an already-constructed Value.
  func Null() Value
  func Bool(v bool) Value
  func Int64(v int64) Value
  func Uint64(v uint64) Value
  func String(v string) Value
  func Bytes(v []byte) Value   // MUST copy v

  // Accessors.
  func (val Value) Kind() Kind
  func (val Value) AsBool() (bool, bool)     // value, ok
  func (val Value) AsInt64() (int64, bool)
  func (val Value) AsUint64() (uint64, bool)
  func (val Value) AsString() (string, bool)
  func (val Value) AsBytes() ([]byte, bool)  // MUST return a COPY
  func (val Value) Equal(other Value) bool

  // EncodeTableRowKey builds [0x01][tableID][encoded pk][version].
  // Requires len(pk) >= 1.
  func EncodeTableRowKey(tableID uint64, pk []Value, version uint64) ([]byte, error)

  // DecodeTableRowKey parses a key. The caller MUST supply expectedKinds (from
  // the table schema): the exact ordered Kinds of the PK columns. Decode
  // verifies count and per-column kind, then version, then full consumption.
  func DecodeTableRowKey(b []byte, expectedKinds []Kind) (tableID uint64, pk []Value, version uint64, err error)

  // TablePrefix returns [0x01][tableID]; bounds all rows of a table.
  func TablePrefix(tableID uint64) []byte

  // Sentinel errors.
  var (
      ErrEmptyKey      = errors.New("key: empty input")
      ErrBadTag        = errors.New("key: unknown keyspace tag")
      ErrBadTypeTag    = errors.New("key: unknown column type tag")
      ErrKindMismatch  = errors.New("key: decoded column kind does not match expected")
      ErrTruncated     = errors.New("key: truncated input")
      ErrBadField      = errors.New("key: malformed variable-length field")
      ErrTrailingBytes = errors.New("key: trailing bytes after decode")
      ErrNoPKColumns   = errors.New("key: at least one pk column required")
  )

---

Tasks (do in order, one at a time)

Task 1 — Create key package skeleton
Create internal/engine/sql/key/key.go with the package declaration, tag
constants, Kind constants, sentinel errors, and compiling stub signatures for
the full Public API (returning zero values / "not implemented" error is fine
here).
Create internal/engine/sql/key/key_test.go with package key and one trivial
test asserting the tag and Kind constants have expected values.
Do NOT create internal/engine/engine.go. Do NOT create any other package.
Verification:
  go build ./...
  go test ./internal/engine/sql/key/
Report Graphify availability.

Task 2 — Implement Value type, constructors, accessors, Equal
Implement Value, the six constructors, Kind(), the typed accessors, and Equal.
Rules:
- Bytes(v) MUST store a copy of v.
- String(v) stores the string (immutable, no copy needed).
- AsBytes() MUST return a copy, never the internal slice.
- Equal compares Kind first then payload; different Kind => not equal;
  bytes/string compared by content.
Tests: construct each Kind, read back via accessors; Equal reflexive/true for
same input, false across kinds; mutation test proving mutating the slice passed
to Bytes (and the slice returned by AsBytes) does NOT change the stored value.
Verification:
  go test ./internal/engine/sql/key/

Task 3 — Implement order-preserving single-column encode/decode
Implement internal:
- encodeValue(val Value) []byte  (type tag + payload per Background rules;
  int64 sign-bit flip; escape 0x00->0x00 0xFF, terminate 0x00 0x01 for
  string/bytes).
- decodeValue(b []byte, expected Kind) (val Value, consumed int, err error)
  reads one value from the FRONT of b, verifies tag == expected (else
  ErrKindMismatch; unknown tag -> ErrBadTypeTag), returns Value and bytes
  consumed. Malformed variable-length field, EXACT semantics: no terminator
  before input ends (or a trailing lone 0x00) -> ErrTruncated; a 0x00 followed
  by a byte other than 0xFF or 0x01 (e.g. 0x00 0x02) -> ErrBadField.
Round-trip tests for each Kind including strings/bytes with embedded 0x00:
[]byte{0}, []byte{0,1}, []byte{1,0}, empty string, empty bytes; assert decoded
Equal and consumed == len(encoded).
Verification:
  go test ./internal/engine/sql/key/

Task 4 — Ordering tests for single-column encoding
Use bytes.Compare on encoded outputs to prove byte order == logical order:
- int64: min, -5, -1, 0, 1, 5, max ascending.
- uint64: 0, 1, 256, max ascending.
- string: "" < "a" < "ab" < "b".
- bytes with embedded zeros: {} < {0} < {0,0} < {0,1} < {1}.
- bool: false < true.
Verification:
  go test ./internal/engine/sql/key/

Task 5 — Implement EncodeTableRowKey and TablePrefix
TablePrefix(tableID) = [0x01][8-byte BE tableID].
EncodeTableRowKey:
- if len(pk) == 0 return ErrNoPKColumns
- write tag 0x01; write 8-byte BE tableID
- append encodeValue(each pk column) in order
- write 8-byte version as ^version big-endian
Tests: worked example produces documented bytes; output begins with
TablePrefix(tableID); empty pk -> ErrNoPKColumns.
Verification:
  go test ./internal/engine/sql/key/

Task 6 — Implement DecodeTableRowKey (schema-arity based)
DecodeTableRowKey(b, expectedKinds):
- len(b) == 0 -> ErrEmptyKey
- len(expectedKinds) == 0 -> ErrNoPKColumns
- read 1 byte tag; != 0x01 -> ErrBadTag
- read 8-byte tableID (ErrTruncated if short)
- for each kind in expectedKinds: decodeValue(remaining, kind); advance by
  consumed; propagate ErrKindMismatch/ErrBadTypeTag/ErrBadField/ErrTruncated
- read exactly 8 remaining bytes as version (invert); ErrTruncated if < 8 remain
- any bytes remaining after the 8-byte version -> ErrTrailingBytes. (This is
  reachable: a caller appends extra bytes after an otherwise-valid key; decode
  consumes exactly tag+tableID+N columns+version and the remainder triggers it.)
Full round-trip tests for single and COMPOSITE PKs across all kinds (including
embedded-zero strings/bytes): encode then decode(expectedKinds) returns the
original tableID, pk (Equal), and version.
Verification:
  go test ./internal/engine/sql/key/

Task 7 — Full-key ordering tests
bytes.Compare on full encoded keys proving:
- equal tableID + equal pk: NEWER version sorts BEFORE older.
- equal tableID: order by pk ascending regardless of version.
- different tableIDs: order by tableID ascending.
- composite PK: lexicographic by first column, then second, etc.
Verification:
  go test ./internal/engine/sql/key/
  go test -race ./internal/engine/sql/key/

Task 8 — Error-path and randomized round-trip tests
Error-path (assert exact sentinel via errors.Is):
- empty input -> ErrEmptyKey
- bad keyspace tag -> ErrBadTag
- expectedKinds empty -> ErrNoPKColumns
- truncated tableID -> ErrTruncated
- type tag != expected -> ErrKindMismatch
- unknown type tag -> ErrBadTypeTag
- variable-length field with no terminator before end -> ErrTruncated
- variable-length field with invalid escape (0x00 0x02) -> ErrBadField
- fewer than 8 version bytes -> ErrTruncated
- extra bytes appended after a valid key -> ErrTrailingBytes
Randomized round-trip (math/rand, FIXED seed): random tableIDs, random
composite PKs (random arity 1..6, random kinds including strings/bytes whose
bytes may contain 0x00), random versions; encode then decode with matching
expectedKinds; assert tableID, version, and every pk column recovered Equal.
Verification:
  go test ./internal/engine/sql/key/
  go test -race ./internal/engine/sql/key/

Task 9 — Documentation
Create docs/sql_engine_key.md: four-part key layout; reserved keyspace tags
(0x02 metadata, 0x03 index reserved for later); per-type encoding rules;
escape (0x00 0xFF) / terminate (0x00 0x01) scheme and why it is unambiguous and
order-preserving; descending version rule; the "decode requires known PK arity"
contract; the worked example. Keep it accurate to the code.
Verification:
  go build ./...
  go test ./...
  go test -race ./...

---

Definition Of Done
internal/engine/sql/key/ implements the full Public API
NO internal/engine/engine.go was created
encode/decode are exact inverses for all kinds, composite PKs, and embedded-zero
  strings/bytes
DecodeTableRowKey uses caller-supplied expectedKinds (no blind decode)
byte order equals logical order for every supported type, including
  embedded-zero byte slices
newer versions sort before older versions
ErrTrailingBytes, ErrNoPKColumns, ErrKindMismatch are all reachable and tested
Bytes() copies on construct; AsBytes() returns a copy (mutation test passes)
all sentinel errors returned via errors.Is on the right inputs
randomized round-trip test passes
go build ./... passes
go test ./... passes
go test -race ./... passes
docs/sql_engine_key.md exists and matches the code
no third-party dependencies added

---

Out Of Scope (do not implement in this plan)
shared Engine interface / internal/engine/engine.go
KVStore interface
bbolt or Pebble backends
value/row codec
schema / catalog-lite / tableID assignment
operations (CreateTable/Insert/Scan/Update/Delete)
transactions / MVCC version management
secondary index encoding (only the 0x03 tag is reserved)
metadata encoding (only the 0x02 tag is reserved)
SQL parsing
networking