# sql_key_setup.md

# Plomvix SQL Key Encoding Plan

## Purpose

Create the key encoding foundation for the Plomvix SQL engine.

This plan introduces `internal/engine/sql/key` — a pure, dependency-free
package that encodes and decodes int64, uint64, string, raw byte slice,
sort-safe composite, and storage composite keys.

This is the single authoritative key encoding package for the Plomvix
SQL engine. No other key encoding package exists or should be created.

This is database foundation work only.

Do not add a KV store yet.
Do not add WAL.
Do not add storage pages.
Do not add query execution.
Do not add API server.
Do not add UI.
Do not wire key encoding into lifecycle or runtime.

---

## Feature Name

```text
SQL Key Encoding
```

Plan file:

```text
sql_key_setup.md
```

New package:

```text
internal/engine/sql/key
```

---

## Required Starting State

This plan starts only after `runtime_signals.md` is completed and
verified.

The runtime layer must be complete:

```text
config     → done and hardened
logger     → done and hardened
lifecycle  → done and hardened
runtime    → done, hardened, and signal-aware
```

This plan does not depend on config, logger, lifecycle, or runtime
at the code level. The key package is a pure standard-library
package with zero internal imports.

---

## Current Project State

Completed foundation work:

```text
config foundation:              done
enterprise config hardening:    done
basic logger setup:             done
enterprise logger hardening:    done
lifecycle foundation:           done
enterprise lifecycle hardening: done
runtime setup:                  done
enterprise runtime hardening:   done
runtime signal handling:        done
```

Current stage:

```text
first database foundation feature
```

Current feature area:

```text
SQL key encoding
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

Use only Go standard library.

Do not add external dependencies.

---

## Coding Agent

Coding agent:

```text
DeepSeek V4 Pro
```

If the local environment uses a different exact DeepSeek model identifier,
use the configured DeepSeek coding model available there.

Tasks must be executed one at a time, in exact order.

Do not proceed to the next task until the current task passes verification.

---

## Graphify Rule

For every task:

1. Search Graphify before starting the task if Graphify is available.
2. Update Graphify after completing the task if Graphify is available.
3. If Graphify is unavailable, do not block the task.
4. Mention Graphify availability in the task report.

---

## Global Project Rules

Follow these rules for every task:

* Keep implementation small and focused.
* Do not add future placeholders.
* Do not add unrelated folders.
* Do not import internal/config, internal/logger, internal/lifecycle,
  or internal/runtime from the key package.
* Do not add external dependencies.
* Use only Go standard library.
* Keep tests deterministic.
* Use table-driven tests throughout.
* Do not create a root-level `tests/` directory.
* Do not add a KV store in this plan.
* Do not add WAL in this plan.
* Do not add storage pages in this plan.
* Do not add little-endian encoding variants.

---

## Dependency Direction Rules

The key package must have zero internal imports:

```text
internal/engine/sql/key imports nothing from internal/
```

Allowed standard library imports:

```text
bytes
encoding/binary
errors
fmt
math
strings
```

Use only what is actually needed.

Future packages that may import this package:

```text
internal/engine/sql/index    (not in this plan)
internal/engine/sql/table    (not in this plan)
internal/engine/sql/storage  (not in this plan)
```

---

## Design Decisions

### No Little-Endian Variants

Little-endian encoding is not included in this plan.

Every key that participates in ordering, scanning, or index lookups
must use big-endian sort-safe encoding. Little-endian has no use in
a SQL engine at this stage. If LE is ever needed for a specific
low-level storage context, it will be added in a separate plan with
its own Kind values and its own decoders.

Do not add LE variants in this plan.

### Key Output Type

The encoding output is a `Key` struct:

```go
type Key struct {
    data   []byte
    fields []Field
}
```

`data` is the encoded byte representation used for storage and
comparison.

`fields` records each encoded field in order so in-process decoders
can use field metadata for fast access.

Both fields are unexported. Access is through methods only.

`Key.Bytes()` and `Key.Fields()` return copies, not internal slices.

### Persistence-Safe Decoding

The `fields` slice exists only in memory. When a key is read back
from disk, only the raw bytes are available — `fields` is gone.

To support decoding from raw bytes, the package exposes:

```go
func ParseKey(data []byte, kinds []Kind) (Key, error)
```

The caller provides the raw bytes and the expected field kinds in
order. The SQL engine knows the schema of every key it reads, so
this is always available at the call site.

`ParseKey` reconstructs the `fields` slice from the known wire
format and the provided kinds, producing a fully usable `Key` that
can then be decoded with the normal `DecodeXxx` functions.

For storage composite keys, a separate parser is provided:

```go
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)
```

This parser reads length-prefix framing to reconstruct field metadata.

This is how production SQL engines work: the schema is the type
information, not the wire format.

### Field Type

```go
type Field struct {
    Kind   Kind
    Offset int
    Length int
}
```

`Kind` identifies the type of the encoded value.

`Offset` is the byte offset into `Key.data` where this field's
content starts.

`Length` is the number of bytes this field occupies in `Key.data`.

For composite keys, `Offset` and `Length` point to the field
content bytes only, not including any framing bytes.

### Kind Type

```go
type Kind uint8

const (
    KindUint64 Kind = 1
    KindInt64  Kind = 2
    KindString Kind = 3
    KindBytes  Kind = 4
)
```

Kind values start at 1. Zero is never a valid Kind, which helps
catch uninitialized fields.

There are no LE Kind variants. All integer encoding is big-endian.

### Integer Encoding

Integers use fixed-width big-endian encoding for lexicographic sort
order.

`uint64`: encoded as 8 bytes big-endian.

`int64`: encoded as 8 bytes big-endian with the sign bit flipped
so that negative values sort before positive values:

```go
u := uint64(v) ^ (1 << 63)
```

Decoding reverses the flip:

```go
u := binary.BigEndian.Uint64(data)
v := int64(u ^ (1 << 63))
```

### String Encoding

Standalone string keys are null-terminated.

```text
encoded = []byte(s) + 0x00
```

The null terminator marks the end of the string.

Strings containing embedded null bytes are rejected at encode time.

### Bytes Encoding

Raw byte slice keys are encoded as-is with no delimiter or framing.

Empty and nil byte slices produce a Key with empty data and a
single Field of Length 0. This is valid. The decoder must not
treat zero-length data as an error when field metadata is present.

### Sort-Safe Composite Keys

Sort-safe composite keys encode multiple fixed-width fields in
sequence with no framing or separators.

```go
func EncodeSortComposite(fields ...any) (Key, error)
```

Accepted field types:

```text
uint64  — 8 bytes big-endian
int64   — 8 bytes big-endian with sign bit flip
```

Strings and bytes are NOT allowed in sort composites.

Reason:

```text
Variable-length fields in a composite key break lexicographic sort
order when concatenated or length-prefixed. Fixed-width fields have
no such problem. Sort composites in this plan are integer-only.
Variable-length composite keys use EncodeStorageComposite instead.
```

Return `ErrUnsupportedSortType` for string or bytes fields.

Sort order guarantee:

```text
EncodeSortComposite(a1, b1).Compare(EncodeSortComposite(a2, b2))
orders first by a, then by b, for any int64/uint64 a and b.
This works because fixed-width fields have uniform byte length,
so concatenation preserves lexicographic sort order across fields.
```

### Storage Composite Keys

Storage composite keys encode variable-length fields using
length-prefix framing. They are NOT sort-safe.

```go
func EncodeStorageComposite(fields ...any) (Key, error)
```

Accepted field types:

```text
uint64  — 8 bytes big-endian
int64   — 8 bytes big-endian with sign bit flip
string  — raw bytes, no null terminator, null bytes rejected
[]byte  — raw bytes as-is
```

Each field is framed as:

```text
[4-byte big-endian uint32 length][field content bytes]
```

The length is the byte count of the field content only, not
including the 4 framing bytes.

String fields inside storage composites do NOT use null termination.
The length prefix defines the field boundary.

Return `ErrUnsupportedType` for unrecognised field types.
Return `ErrNullByteInString` for string fields containing null bytes.

Storage composite keys must be clearly documented as NOT sort-safe.
Do not use storage composites as index keys or scan boundaries.

### Decoder Error Rules

Decoders must validate using field metadata, not raw data length.

A key with zero-length data but valid field metadata is valid
(e.g. an empty bytes key).

A zero Key with no data and no fields is invalid.

Decoder rules:

```text
Zero Key (no data, no fields)          → ErrInvalidKey
Wrong Kind                             → ErrKindMismatch
Field offset/length out of bounds      → ErrInvalidKey
ParseKey kinds mismatch or out of sync → ErrInvalidKey
```

Do not return ErrInvalidKey solely because len(data) == 0.
Check field metadata first.

---

## Final Public API

Package:

```text
internal/engine/sql/key
```

Types:

```go
type Kind uint8

const (
    KindUint64 Kind = 1
    KindInt64  Kind = 2
    KindString Kind = 3
    KindBytes  Kind = 4
)

type Field struct {
    Kind   Kind
    Offset int
    Length int
}

type Key struct {
    // unexported fields only
}
```

Encoding functions:

```go
func EncodeUint64(v uint64) Key
func EncodeInt64(v int64) Key
func EncodeString(s string) (Key, error)
func EncodeBytes(b []byte) Key
func EncodeSortComposite(fields ...any) (Key, error)
func EncodeStorageComposite(fields ...any) (Key, error)
```

Decoding functions:

```go
func DecodeUint64(k Key) (uint64, error)
func DecodeInt64(k Key) (int64, error)
func DecodeString(k Key) (string, error)
func DecodeBytes(k Key) ([]byte, error)
func DecodeSortComposite(k Key) ([]any, error)
func DecodeStorageComposite(k Key) ([]any, error)
```

Schema-driven decode:

```go
func ParseKey(data []byte, kinds []Kind) (Key, error)
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)
```

Key methods:

```go
func (k Key) Bytes() []byte
func (k Key) Compare(other Key) int
func (k Key) Fields() []Field
```

Public errors:

```go
var (
    ErrNullByteInString    = errors.New("sql/key: string contains null byte")
    ErrUnsupportedType     = errors.New("sql/key: unsupported field type")
    ErrUnsupportedSortType = errors.New("sql/key: type not allowed in sort composite")
    ErrInvalidKey          = errors.New("sql/key: invalid key")
    ErrKindMismatch        = errors.New("sql/key: kind mismatch")
    ErrNotComposite        = errors.New("sql/key: key is not composite")
)
```

---

## Non-Goals

Do not implement:

* KV store
* WAL
* storage pages
* buffer pool
* query execution
* indexes
* table heap
* schema catalog
* transaction IDs in keys
* version stamps in keys
* key compression
* key prefix truncation
* bloom filters
* little-endian encoding
* external dependencies
* config, logger, lifecycle, or runtime integration

---

## Final Expected Structure

After this plan:

```text
plomvix/
├── internal/
│   └── engine/
│       └── sql/
│           └── key/
│               ├── key.go
│               ├── key_test.go
│               └── key_internal_test.go
├── docs/
│   └── sql_key.md
```

The `internal/engine/sql/key/` directory is created as part of
this plan. No other new folders are required.

---

## Task Plan

---

## TASK 01 — Create key package skeleton and types

### Goal

Create `internal/engine/sql/key` with all public types, constants,
and error sentinels.

### Files

Create:

```text
internal/engine/sql/key/key.go
```

### Requirements

Add package declaration:

```go
package key
```

Add import:

```go
errors
```

Add `Kind` type and constants:

```go
type Kind uint8

const (
    KindUint64 Kind = 1
    KindInt64  Kind = 2
    KindString Kind = 3
    KindBytes  Kind = 4
)
```

Add `Field` struct:

```go
type Field struct {
    Kind   Kind
    Offset int
    Length int
}
```

Add `Key` struct with unexported fields:

```go
type Key struct {
    data   []byte
    fields []Field
}
```

Add public errors:

```go
var (
    ErrNullByteInString    = errors.New("sql/key: string contains null byte")
    ErrUnsupportedType     = errors.New("sql/key: unsupported field type")
    ErrUnsupportedSortType = errors.New("sql/key: type not allowed in sort composite")
    ErrInvalidKey          = errors.New("sql/key: invalid key")
    ErrKindMismatch        = errors.New("sql/key: kind mismatch")
    ErrNotComposite        = errors.New("sql/key: key is not composite")
)
```

Add `Key` method stubs returning zero values:

```go
func (k Key) Bytes() []byte       { return nil }
func (k Key) Compare(other Key) int { return 0 }
func (k Key) Fields() []Field     { return nil }
```

Do not implement encoding, decoding, or ParseKey yet.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 01 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 02 — Add type and error sentinel tests

### Goal

Verify Kind constants and error sentinel values are stable.

### Files

Create:

```text
internal/engine/sql/key/key_test.go
```

### Package

```go
package key_test
```

### Requirements

Add tests confirming Kind constant values:

```text
KindUint64 == 1
KindInt64  == 2
KindString == 3
KindBytes  == 4
```

Add test comment:

```go
// These values are part of the stable sql/key API.
```

Add tests confirming error sentinel strings:

```text
ErrNullByteInString.Error()    == "sql/key: string contains null byte"
ErrUnsupportedType.Error()     == "sql/key: unsupported field type"
ErrUnsupportedSortType.Error() == "sql/key: type not allowed in sort composite"
ErrInvalidKey.Error()          == "sql/key: invalid key"
ErrKindMismatch.Error()        == "sql/key: kind mismatch"
ErrNotComposite.Error()        == "sql/key: key is not composite"
```

Add test comment:

```go
// These values are part of the stable sql/key API.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 02 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Implement Key methods

### Goal

Implement `Bytes`, `Compare`, and `Fields` on the `Key` struct.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Add import:

```go
bytes
```

Implement `Bytes()`:

```go
func (k Key) Bytes() []byte {
    if k.data == nil {
        return nil
    }
    out := make([]byte, len(k.data))
    copy(out, k.data)
    return out
}
```

Implement `Compare(other Key) int`:

```go
func (k Key) Compare(other Key) int {
    return bytes.Compare(k.data, other.data)
}
```

Implement `Fields() []Field`:

```go
func (k Key) Fields() []Field {
    if k.fields == nil {
        return nil
    }
    out := make([]Field, len(k.fields))
    copy(out, k.fields)
    return out
}
```

Rules:

```text
Bytes() and Fields() must return copies, not internal slices.
External callers must not be able to mutate Key internals.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 03 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add Key method tests

### Goal

Verify `Bytes`, `Compare`, and `Fields` behavior on zero Keys.
Copy-safety tests for non-zero Keys are deferred to TASK 10 after
encoders and ParseKey are available.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Package

Keep:

```go
package key_test
```

### Requirements

At this point no encoders exist and `Key` fields are unexported,
so only zero Key behavior can be tested from the external package.

Test `Bytes()`:

* zero Key returns nil

Test `Compare()`:

* zero Key compared to zero Key returns 0

Test `Fields()`:

* zero Key returns nil

Do not attempt to test copy-safety or non-zero Key behavior in this
task. Those tests require a constructed Key with known content and
must wait until encoders are available in TASK 06 onward.

Copy-safety tests will be added in TASK 10 using keys constructed
via ParseKey.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 04 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 05 — Implement integer encoding and decoding

### Goal

Implement `EncodeUint64`, `EncodeInt64` and their decoders.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Add import:

```go
encoding/binary
```

Implement `EncodeUint64(v uint64) Key`:

* encode `v` as 8 bytes big-endian
* store one Field: Kind=KindUint64, Offset=0, Length=8

Implement `EncodeInt64(v int64) Key`:

* flip sign bit: `u := uint64(v) ^ (1 << 63)`
* encode `u` as 8 bytes big-endian
* store one Field: Kind=KindInt64, Offset=0, Length=8

Implement `DecodeUint64(k Key) (uint64, error)`:

* if k has no fields and no data: return ErrInvalidKey
* if len(k.fields) != 1: return ErrInvalidKey
* if field.Kind != KindUint64: return ErrKindMismatch
* if field.Length != 8: return ErrInvalidKey
* if field.Offset+field.Length > len(k.data): return ErrInvalidKey
* decode 8 bytes big-endian from field.Offset

Implement `DecodeInt64(k Key) (int64, error)`:

* if k has no fields and no data: return ErrInvalidKey
* if len(k.fields) != 1: return ErrInvalidKey
* if field.Kind != KindInt64: return ErrKindMismatch
* if field.Length != 8: return ErrInvalidKey
* if field.Offset+field.Length > len(k.data): return ErrInvalidKey
* decode 8 bytes big-endian from field.Offset
* reverse sign bit: `v := int64(u ^ (1 << 63))`

Decoder validation rule:

```text
Do not return ErrInvalidKey solely because len(k.data) == 0.
A key is invalid only when it has no fields AND no data (zero Key).
Validate by checking that field.Offset+field.Length is within
bounds of k.data.
```

Scalar decoder strictness rule:

```text
DecodeUint64 and DecodeInt64 must require len(k.fields) == 1.
A composite or sort-composite key has multiple fields and must
not be silently decoded as a scalar using only the first field.
If len(k.fields) != 1, return ErrInvalidKey.
Additionally, integer decoders must require field.Length == 8
exactly, not merely that the offset/length fall within bounds.
This rule applies to DecodeUint64, DecodeInt64, DecodeString, and
DecodeBytes equally: all scalar decoders require exactly one field.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 05 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add integer encoding tests

### Goal

Verify integer encoding, decoding, sort order, and round-trip.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Use table-driven tests.

Test `EncodeUint64` round-trip:

```text
0
1
math.MaxUint64
math.MaxUint64 / 2
```

Test `EncodeInt64` round-trip:

```text
0
1
-1
math.MinInt64
math.MaxInt64
```

Test uint64 sort order:

```text
EncodeUint64(0).Compare(EncodeUint64(1))     == -1
EncodeUint64(1).Compare(EncodeUint64(0))     == +1
EncodeUint64(5).Compare(EncodeUint64(5))     == 0
EncodeUint64(255).Compare(EncodeUint64(256)) == -1
```

Test int64 sort order:

```text
EncodeInt64(math.MinInt64).Compare(EncodeInt64(-1)) == -1
EncodeInt64(-1).Compare(EncodeInt64(0))             == -1
EncodeInt64(0).Compare(EncodeInt64(1))              == -1
EncodeInt64(1).Compare(EncodeInt64(math.MaxInt64))  == -1
```

Test decoder error cases:

* zero Key returns ErrInvalidKey
* DecodeUint64 on int64-encoded key returns ErrKindMismatch
* DecodeInt64 on uint64-encoded key returns ErrKindMismatch

Note: a test confirming DecodeUint64 rejects multi-field composite
keys is added later in TASK 12, once EncodeSortComposite exists to
construct such a key.

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 06 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Implement string and bytes encoding and decoding

### Goal

Implement `EncodeString`, `EncodeBytes`, `DecodeString`, `DecodeBytes`.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Add import:

```go
strings
```

Implement `EncodeString(s string) (Key, error)`:

* reject strings containing null bytes:

```go
if strings.ContainsRune(s, 0) {
    return Key{}, ErrNullByteInString
}
```

* encode as `[]byte(s)` followed by null terminator `0x00`
* total data length: `len(s) + 1`
* store one Field: Kind=KindString, Offset=0, Length=len(s)+1

Implement `EncodeBytes(b []byte) Key`:

* encode as a copy of `b` with no framing
* store one Field: Kind=KindBytes, Offset=0, Length=len(b)
* nil and empty slices are valid: produce empty data with Length=0

Implement `DecodeString(k Key) (string, error)`:

* if k has no fields and no data: return ErrInvalidKey
* if len(k.fields) != 1: return ErrInvalidKey
* if field.Kind != KindString: return ErrKindMismatch
* if field.Offset+field.Length > len(k.data): return ErrInvalidKey
* read field bytes from offset
* verify last byte is null terminator: if not, return ErrInvalidKey
* strip null terminator and return as string

Implement `DecodeBytes(k Key) ([]byte, error)`:

* if k has no fields and no data: return ErrInvalidKey
* if len(k.fields) != 1: return ErrInvalidKey
* if field.Kind != KindBytes: return ErrKindMismatch
* if field.Length > 0 and field.Offset+field.Length > len(k.data):
  return ErrInvalidKey
* return a copy of field bytes
* zero-length bytes field returns empty non-nil slice

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 07 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add string and bytes encoding tests

### Goal

Verify string and bytes encoding, decoding, and error cases.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Use table-driven tests.

Test `EncodeString` round-trip:

```text
""
"hello"
"plomvix"
"a\x01b"
```

Test `EncodeString` rejects null bytes:

```text
"hel\x00lo"  → ErrNullByteInString
"\x00"        → ErrNullByteInString
```

Test `EncodeBytes` round-trip:

```text
nil
[]byte{}
[]byte{0x00, 0x01, 0x02}
[]byte{0x00}
```

Test that `EncodeBytes(nil)` and `EncodeBytes([]byte{})` both
decode successfully to an empty non-nil byte slice.

Test string sort order:

```text
"" < "a" < "aa" < "b" < "z"
```

Test decoder error cases using `errors.Is`:

* zero Key returns ErrInvalidKey for both DecodeString and DecodeBytes
* wrong Kind returns ErrKindMismatch

Note: a test confirming DecodeString and DecodeBytes reject
multi-field composite keys is added later in TASK 14, once
EncodeStorageComposite exists to construct such a key with string
and bytes fields.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 08 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 09 — Implement ParseKey

### Goal

Implement schema-driven decode from raw bytes for persistence safety.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Implement:

```go
func ParseKey(data []byte, kinds []Kind) (Key, error)
```

Behavior:

* if len(kinds) == 0: return ErrInvalidKey
* walk the data bytes according to each kind's wire format:

```text
KindUint64 → consume exactly 8 bytes
             Field: Offset=current, Length=8
             return ErrInvalidKey if fewer than 8 bytes remain

KindInt64  → consume exactly 8 bytes
             Field: Offset=current, Length=8
             return ErrInvalidKey if fewer than 8 bytes remain

KindString → scan forward from current offset until null byte 0x00
             Field: Offset=current, Length=bytes including null terminator
             return ErrInvalidKey if no null terminator found before
             end of data

KindBytes  → consume all remaining bytes as one field
             Field: Offset=current, Length=remaining bytes
             KindBytes MUST be the last kind in the slice
             return ErrInvalidKey if KindBytes appears in non-last position
```

* after walking all kinds, if bytes remain unaccounted for and the
  last kind was not KindBytes: return ErrInvalidKey
* store data copy and reconstructed fields in a new Key
* return the Key

Rules:

```text
ParseKey does not decode values, only reconstructs field metadata.
After ParseKey, the normal DecodeXxx functions work correctly.
ParseKey is the persistence boundary: it reconstructs the in-memory
Key from raw stored bytes using the caller-provided schema.
KindBytes may only appear as the last field. Enforce this explicitly.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 09 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Add ParseKey tests

### Goal

Verify ParseKey reconstructs keys correctly from raw bytes.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Use table-driven tests.

Test ParseKey round-trip for each scalar type:

```go
EncodeUint64(42) → Bytes() → ParseKey(data, []Kind{KindUint64})
    → DecodeUint64 → 42

EncodeInt64(-1) → Bytes() → ParseKey(data, []Kind{KindInt64})
    → DecodeInt64 → -1

EncodeString("hi") → Bytes() → ParseKey(data, []Kind{KindString})
    → DecodeString → "hi"

EncodeBytes([]byte{1,2,3}) → Bytes() →
    ParseKey(data, []Kind{KindBytes}) → DecodeBytes → []byte{1,2,3}
```

Test ParseKey error cases:

* empty kinds slice returns ErrInvalidKey
* data too short for KindUint64 returns ErrInvalidKey
* string data with no null terminator returns ErrInvalidKey
* KindBytes in non-last position returns ErrInvalidKey
* leftover bytes after all non-KindBytes kinds are consumed
  returns ErrInvalidKey

Test Compare works correctly after ParseKey:

```go
k1 := EncodeUint64(1)
k2raw := EncodeUint64(2).Bytes()
k2, _ := ParseKey(k2raw, []Kind{KindUint64})
// k1.Compare(k2) must equal -1
```

Test copy-safety of Bytes() using a ParseKey-constructed key:

```go
k, _ := ParseKey(EncodeUint64(42).Bytes(), []Kind{KindUint64})
b := k.Bytes()
b[0] = 0xFF
b2 := k.Bytes()
// b2[0] must not equal 0xFF — mutation must not affect original
```

Test copy-safety of Fields() using a ParseKey-constructed key:

```go
k, _ := ParseKey(EncodeUint64(42).Bytes(), []Kind{KindUint64})
f := k.Fields()
f[0].Offset = 999
f2 := k.Fields()
// f2[0].Offset must not equal 999 — mutation must not affect original
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 10 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 11 — Implement sort composite encoding and decoding

### Goal

Implement `EncodeSortComposite` and `DecodeSortComposite`.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Implement `EncodeSortComposite(fields ...any) (Key, error)`:

* accept only uint64 and int64 values
* reject string and []byte with ErrUnsupportedSortType
* reject all other types with ErrUnsupportedType
* encode each field using the same big-endian encoding as scalar
  integer encoders (with sign bit flip for int64)
* concatenate all field bytes with no framing or separator
* record one Field per value: Kind, Offset, Length=8

Wire format:

```text
[8 bytes field 0][8 bytes field 1]...[8 bytes field N]
```

No length prefixes. No separators. Pure fixed-width concatenation.

Sort order guarantee:

```text
EncodeSortComposite(a1, b1).Compare(EncodeSortComposite(a2, b2))
orders first by a, then by b, for any int64/uint64 combinations.
Fixed-width fields have uniform byte length so concatenation
preserves lexicographic sort order across fields.
```

Implement `DecodeSortComposite(k Key) ([]any, error)`:

* if k has no fields: return ErrNotComposite
* for each field in k.Fields():
  * if field.Offset+field.Length > len(k.data): return ErrInvalidKey
  * if field.Kind == KindUint64 or field.Kind == KindInt64:
    if field.Length != 8: return ErrInvalidKey
  * KindUint64: decode 8 bytes big-endian → uint64
  * KindInt64:  decode 8 bytes big-endian, reverse sign bit → int64
  * other Kind: return ErrKindMismatch
* return []any with values in field order

Validation rule:

```text
Validate field bounds and exact length BEFORE decoding each field,
not after. A field with Length != 8 for KindUint64 or KindInt64
must be rejected with ErrInvalidKey before any byte read is
attempted. This prevents a malformed or hand-crafted Key (for
example one built via a buggy or adversarial ParseKey call) from
causing an out-of-bounds slice read or silently decoding the wrong
number of bytes into a value.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 11 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 12 — Add sort composite tests

### Goal

Verify sort composite encoding, decoding, sort order, and error cases.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Use table-driven tests.

Test `EncodeSortComposite` round-trip:

```text
(uint64(1), uint64(2))
(int64(-1), int64(0))
(uint64(0), int64(math.MinInt64))
(int64(math.MaxInt64), uint64(math.MaxUint64))
```

Test sort order:

```text
EncodeSortComposite(uint64(0), uint64(0)) <
EncodeSortComposite(uint64(0), uint64(1))

EncodeSortComposite(uint64(0), uint64(999)) <
EncodeSortComposite(uint64(1), uint64(0))

EncodeSortComposite(int64(-1), uint64(0)) <
EncodeSortComposite(int64(0), uint64(0))
```

Test `EncodeSortComposite` rejects:

* string field   → ErrUnsupportedSortType
* []byte field   → ErrUnsupportedSortType
* bool field     → ErrUnsupportedType

Test `DecodeSortComposite` error cases:

* zero Key returns ErrNotComposite

Note: defensive validation tests for malformed field metadata
(wrong length, out-of-bounds offset) are added separately in
TASK 12B using an internal test file, since constructing such
malformed keys requires access to unexported `Key` fields that
the external `key_test` package cannot reach.

Test that scalar decoders reject multi-field composite keys:

```go
composite, _ := EncodeSortComposite(uint64(1), uint64(2))
_, err := DecodeUint64(composite)
// err must match ErrInvalidKey, because len(fields) != 1
// DecodeUint64 must not silently decode only the first field
```

This proves a composite key cannot be accidentally decoded as a
scalar using only its first field.

Test that ParseKey works with sort composite keys:

```go
k, _ := EncodeSortComposite(uint64(1), int64(-1))
raw := k.Bytes()
k2, _ := ParseKey(raw, []Kind{KindUint64, KindInt64})
vals, _ := DecodeSortComposite(k2)
// vals[0] must equal uint64(1)
// vals[1] must equal int64(-1)
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 12 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 12B — Add internal defensive validation tests for decoders

### Goal

Directly test the defensive bounds and length validation added to
`DecodeSortComposite` (and later `DecodeStorageComposite` in TASK 14B)
by constructing malformed `Key` values from inside the package, where
unexported fields are accessible.

This is necessary because the external `key_test` package cannot
construct a `Key` with a deliberately wrong-length or out-of-bounds
`Field`, since `Key.fields` is unexported and every public constructor
(`EncodeSortComposite`, `EncodeStorageComposite`, `ParseKey`,
`ParseStorageCompositeKey`) only ever produces well-formed keys.

### Files

Create:

```text
internal/engine/sql/key/key_internal_test.go
```

### Package

Use the internal package:

```go
package key
```

### Requirements

Add tests that construct a `Key` directly using unexported fields
to simulate malformed input that should never occur through normal
public API usage, but which the decoders must still reject safely:

```go
func TestDecodeSortComposite_RejectsWrongLength(t *testing.T) {
    k := Key{
        data: make([]byte, 4), // too short for an 8-byte field
        fields: []Field{
            {Kind: KindUint64, Offset: 0, Length: 4},
        },
    }
    _, err := DecodeSortComposite(k)
    if !errors.Is(err, ErrInvalidKey) {
        t.Fatalf("expected ErrInvalidKey, got %v", err)
    }
}

func TestDecodeSortComposite_RejectsOutOfBoundsOffset(t *testing.T) {
    k := Key{
        data: make([]byte, 8),
        fields: []Field{
            {Kind: KindUint64, Offset: 4, Length: 8}, // 4+8 > 8
        },
    }
    _, err := DecodeSortComposite(k)
    if !errors.Is(err, ErrInvalidKey) {
        t.Fatalf("expected ErrInvalidKey, got %v", err)
    }
}
```

Add equivalent tests for `DecodeStorageComposite` separately in
TASK 14B, once `DecodeStorageComposite` exists. Do not add them here.

Rules:

* this file exists specifically to exercise defensive code paths
  that are unreachable through the public API alone
* do not use this file to bypass intended public API behavior in
  any other test
* keep this file focused only on malformed-Key defensive tests

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 12B completed.
Files changed:
- internal/engine/sql/key/key_internal_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 13 — Implement storage composite encoding and decoding

### Goal

Implement `EncodeStorageComposite`, `DecodeStorageComposite`, and
`ParseStorageCompositeKey`.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Implement `EncodeStorageComposite(fields ...any) (Key, error)`:

* accept uint64, int64, string, []byte
* reject all other types with ErrUnsupportedType
* reject string fields containing null bytes with ErrNullByteInString
* encode each field with a 4-byte big-endian uint32 length prefix
  followed by the field content bytes

Field encoding per type:

```text
uint64 → [4-byte length=8][8-byte big-endian uint64]
int64  → [4-byte length=8][8-byte big-endian, sign bit flipped]
string → [4-byte length=len(s)][raw string bytes, no null terminator]
[]byte → [4-byte length=len(b)][raw bytes]
```

Record one Field per value:

```text
Kind:   field kind
Offset: byte offset of field CONTENT (after the 4-byte length prefix)
Length: byte length of field CONTENT (not including length prefix)
```

Add a code comment on the function:

```go
// EncodeStorageComposite encodes variable-length fields using
// length-prefix framing. The resulting key is NOT sort-safe.
// Do not use storage composite keys as index keys or scan boundaries.
```

Implement `DecodeStorageComposite(k Key) ([]any, error)`:

* if k has no fields: return ErrNotComposite
* for each field in k.Fields():
  * if field.Offset+field.Length > len(k.data): return ErrInvalidKey
  * if field.Kind == KindUint64 or field.Kind == KindInt64:
    if field.Length != 8: return ErrInvalidKey
  * KindUint64: decode 8 bytes big-endian → uint64
  * KindInt64:  decode 8 bytes big-endian, reverse sign bit → int64
  * KindString: read field bytes at Offset/Length, return as string
                (no null terminator expected)
  * KindBytes:  return copy of field bytes at Offset/Length
  * other Kind: return ErrKindMismatch
* return []any with values in field order

Validation rule:

```text
Validate field bounds and exact length BEFORE decoding each field,
not after. KindUint64 and KindInt64 fields must have Length == 8
exactly, checked before any byte read. String and bytes fields only
need the offset/length bounds check since they are variable-length
by design. This prevents a malformed Key from causing an
out-of-bounds slice read or a silently wrong decoded value.
```

Implement `ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)`:

* if len(kinds) == 0: return ErrInvalidKey
* for each kind, read 4-byte big-endian uint32 length prefix:
  * if fewer than 4 bytes remain: return ErrInvalidKey
  * read length value
  * if fewer than length bytes remain after prefix: return ErrInvalidKey
  * record Field: Kind=kind, Offset=current+4, Length=length
  * advance current position by 4 + length
* after all kinds are consumed, if bytes remain: return ErrInvalidKey
* store data copy and reconstructed fields in a new Key

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 13 completed.
Files changed:
- internal/engine/sql/key/key.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 14 — Add storage composite tests

### Goal

Verify storage composite encoding, decoding, and persistence safety.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Use table-driven tests.

Test `EncodeStorageComposite` round-trip:

```text
(uint64(1), "hello", []byte{0x01, 0x02})
(int64(-1), uint64(42))
("plomvix", int64(0))
("", []byte{})
```

Test `EncodeStorageComposite` rejects:

* unsupported type (bool)  → ErrUnsupportedType
* string with null byte    → ErrNullByteInString

Test that storage composite string fields do NOT include null
terminator in raw bytes:

```go
k, _ := EncodeStorageComposite("hello")
fields := k.Fields()
if fields[0].Length != len("hello") {
    t.Fatal("storage composite string must not include null terminator")
}
```

Test `ParseStorageCompositeKey` round-trip:

```go
k, _ := EncodeStorageComposite(uint64(1), "hi", []byte{0xFF})
raw := k.Bytes()
k2, _ := ParseStorageCompositeKey(raw, []Kind{KindUint64, KindString, KindBytes})
vals, _ := DecodeStorageComposite(k2)
// verify vals[0] == uint64(1), vals[1] == "hi", vals[2] == []byte{0xFF}
```

Test that sort and storage composite wire formats are byte-distinct
for the same integer fields:

```go
sort, _    := EncodeSortComposite(uint64(1), uint64(2))
storage, _ := EncodeStorageComposite(uint64(1), uint64(2))
if bytes.Equal(sort.Bytes(), storage.Bytes()) {
    t.Fatal("sort and storage composites must differ in wire format")
}
```

Test `DecodeStorageComposite` error cases:

* zero Key returns ErrNotComposite

Test that scalar decoders reject multi-field storage composite keys:

```go
storage, _ := EncodeStorageComposite("hello", []byte{0x01})
_, err := DecodeString(storage)
// err must match ErrInvalidKey, because len(fields) != 1

_, err2 := DecodeBytes(storage)
// err2 must match ErrInvalidKey, because len(fields) != 1
```

This proves a storage composite key cannot be accidentally decoded
as a single scalar string or bytes value.

Add test comment:

```go
// Storage composite keys are not sort-safe.
// Do not use Compare to order storage composite keys.
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 14 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 14B — Add internal defensive validation tests for DecodeStorageComposite

### Goal

Directly test the defensive bounds and length validation added to
`DecodeStorageComposite` by constructing malformed `Key` values from
inside the package.

### Files

Modify:

```text
internal/engine/sql/key/key_internal_test.go
```

### Package

Use the internal package:

```go
package key
```

### Requirements

Add tests in the same internal test file created in TASK 12B:

```go
func TestDecodeStorageComposite_RejectsWrongIntegerLength(t *testing.T) {
    k := Key{
        data: make([]byte, 4), // too short for an 8-byte uint64 field
        fields: []Field{
            {Kind: KindUint64, Offset: 0, Length: 4},
        },
    }
    _, err := DecodeStorageComposite(k)
    if !errors.Is(err, ErrInvalidKey) {
        t.Fatalf("expected ErrInvalidKey, got %v", err)
    }
}

func TestDecodeStorageComposite_RejectsOutOfBoundsStringField(t *testing.T) {
    k := Key{
        data: []byte("hi"),
        fields: []Field{
            {Kind: KindString, Offset: 0, Length: 10}, // exceeds data
        },
    }
    _, err := DecodeStorageComposite(k)
    if !errors.Is(err, ErrInvalidKey) {
        t.Fatalf("expected ErrInvalidKey, got %v", err)
    }
}
```

Rules:

* add these tests to the same `key_internal_test.go` file from
  TASK 12B, do not create a second internal test file
* do not duplicate function names already present in the file
* keep this file focused only on malformed-Key defensive tests

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 14B completed.
Files changed:
- internal/engine/sql/key/key_internal_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 15 — Add sort order and comparison tests

### Goal

Verify sort order is preserved across all key types and sort
composite keys.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Add a test helper:

```go
func assertAscending(t *testing.T, keys []key.Key)
```

This helper verifies that each consecutive pair of keys satisfies
`keys[i].Compare(keys[i+1]) <= 0`.

Test uint64 sort order over a range:

```text
0, 1, 255, 256, math.MaxUint64/2, math.MaxUint64
```

Test int64 sort order over a range:

```text
math.MinInt64, -256, -1, 0, 1, 256, math.MaxInt64
```

Test string sort order:

```text
"", "a", "aa", "b", "z", "zz"
```

Test bytes sort order:

```text
[]byte{}, []byte{0x00}, []byte{0x01}, []byte{0x7F}, []byte{0xFF}
```

Test sort composite sort order:

```text
(uint64(0), uint64(0))
(uint64(0), uint64(1))
(uint64(0), uint64(math.MaxUint64))
(uint64(1), uint64(0))
(uint64(math.MaxUint64), uint64(math.MaxUint64))
```

Test int64/uint64 mixed sort composite:

```text
(int64(math.MinInt64), uint64(0))
(int64(-1), uint64(0))
(int64(0), uint64(0))
(int64(1), uint64(0))
(int64(math.MaxInt64), uint64(0))
```

Add test comment:

```go
// Storage composite keys are not sort-safe.
// Do not use assertAscending on EncodeStorageComposite results.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 15 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 16 — Add key encoding documentation

### Goal

Document the SQL key encoding package.

### Files

Create:

```text
docs/sql_key.md
```

### Requirements

Create documentation with heading:

```text
# Plomvix SQL Key Encoding
```

Document:

* purpose: single authoritative key encoder for the SQL engine
* Key struct and Field struct
* Kind constants
* uint64 encoding: big-endian sort-safe
* int64 encoding: sign bit flip, sort-safe
* string encoding: null-terminated standalone
* bytes encoding: raw, no framing
* EncodeSortComposite: fixed-width integer fields only, sort-safe
* EncodeStorageComposite: variable-length, length-prefix framing,
  NOT sort-safe
* ParseKey: schema-driven decode from raw bytes for scalar and
  sort composite keys
* ParseStorageCompositeKey: schema-driven decode for storage composites
* Compare method
* zero internal imports
* no little-endian variants

The documentation must include these exact strings because TASK 17
checks them:

```text
# Plomvix SQL Key Encoding
sql/key
Key struct
Field struct
Kind
uint64
int64
sign bit flip
big-endian
sort order
null-terminated
EncodeSortComposite
EncodeStorageComposite
storage composite is not sort-safe
length-prefix
ParseKey
ParseStorageCompositeKey
Compare
zero internal imports
no little-endian
```

Non-goals section must include:

```text
KV store
WAL
storage pages
query execution
indexes
transaction IDs
version stamps
compression
little-endian
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 16 completed.
Files changed:
- docs/sql_key.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 17 — Add documentation tests

### Goal

Verify documentation exists and covers required content.

### Files

Modify:

```text
internal/engine/sql/key/key_test.go
```

### Requirements

Add a documentation test that reads:

```go
os.ReadFile("../../../../docs/sql_key.md")
```

Path note:

```text
This path assumes the test runs from internal/engine/sql/key/,
which is the default behavior of go test ./...
```

Test that the document contains these exact strings:

```text
# Plomvix SQL Key Encoding
sql/key
Key struct
Field struct
Kind
uint64
int64
sign bit flip
big-endian
sort order
null-terminated
EncodeSortComposite
EncodeStorageComposite
storage composite is not sort-safe
length-prefix
ParseKey
ParseStorageCompositeKey
Compare
zero internal imports
no little-endian
KV store
WAL
storage pages
query execution
indexes
transaction IDs
version stamps
compression
little-endian
```

Use stable substring checks.

Do not make fragile checks for full paragraphs.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 17 completed.
Files changed:
- internal/engine/sql/key/key_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 18 — Final review

### Goal

Review the key encoding package for correctness, completeness,
scope control, and project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
internal/engine/sql/key/key_internal_test.go
docs/sql_key.md
go.mod
go.sum
```

### Requirements

Confirm:

* package is `internal/engine/sql/key`
* Kind constants are correct and stable
* error sentinels are correct and stable
* Key.Bytes() returns a copy
* Key.Fields() returns a copy
* Key.Compare() uses bytes.Compare
* EncodeUint64 is big-endian sort-safe
* EncodeInt64 flips sign bit correctly
* EncodeString null-terminates and rejects null bytes
* EncodeBytes handles nil and empty slices
* EncodeSortComposite is fixed-width integer-only
* EncodeSortComposite rejects strings/bytes with ErrUnsupportedSortType
* EncodeSortComposite preserves sort order across fields
* EncodeStorageComposite uses length-prefix framing
* EncodeStorageComposite does not null-terminate string fields
* EncodeStorageComposite is documented as NOT sort-safe
* sort and storage composite wire formats are byte-distinct
* ParseKey reconstructs scalar and sort composite keys from raw bytes
* ParseKey rejects KindBytes in non-last position
* ParseStorageCompositeKey reconstructs storage composite keys
* DecodeUint64 and DecodeInt64 round-trip correctly
* DecodeUint64, DecodeInt64, DecodeString, DecodeBytes all require
  exactly one field (len(k.fields) == 1), rejecting composite keys
* DecodeUint64 and DecodeInt64 require field.Length == 8 exactly
* DecodeString strips null terminator correctly
* DecodeBytes handles empty field correctly
* DecodeSortComposite reconstructs integer fields
* DecodeSortComposite validates field bounds and exact Length==8
  before decoding, tested via key_internal_test.go
* DecodeStorageComposite reconstructs all variable-length field types
* DecodeStorageComposite validates field bounds and exact Length==8
  for integer fields before decoding, tested via key_internal_test.go
* `internal/engine/sql/key/key_internal_test.go` exists and contains
  defensive validation tests for malformed Key field metadata
* decoder validates using field metadata not len(data)==0
* sort order tests pass for all scalar types and EncodeSortComposite;
  EncodeStorageComposite is documented and tested as not sort-safe
* ParseKey round-trip tests pass
* ParseStorageCompositeKey round-trip tests pass
* storage composite round-trip tests pass
* no LE variants exist anywhere in the package
* no internal imports in key package
* no external dependencies added
* go.mod unchanged
* go.sum unchanged
* docs exist and pass tests

If issues are found:

1. Fix them.
2. Run final verification again.
3. Report what was fixed.

### Final Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
go mod tidy
go test ./...
```

### Completion Report

```text
TASK 18 completed.
Files reviewed:
- internal/engine/sql/key/key.go
- internal/engine/sql/key/key_test.go
- docs/sql_key.md
- go.mod
- go.sum

Final verification:
- go test ./...
- go build ./...
- go test -race ./...
- go mod tidy
- go test ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable

Final status:
- SQL key encoding complete
- single authoritative key encoder established
- sort-safe and storage composite encoding both implemented
- round-trip verified for all types
- sort order verified for scalar keys and EncodeSortComposite only;
  EncodeStorageComposite is documented/tested as not sort-safe
- persistence-safe via ParseKey and ParseStorageCompositeKey
- no LE variants
- no non-goal systems introduced
```

---

## Completion Criteria

This plan is complete only when:

* `internal/engine/sql/key/key.go` exists
* `internal/engine/sql/key/key_test.go` exists
* `internal/engine/sql/key/key_internal_test.go` exists with
  defensive validation tests for DecodeSortComposite and
  DecodeStorageComposite
* `docs/sql_key.md` exists
* Kind constants are stable and tested
* error sentinels are stable and tested
* Key.Bytes() and Key.Fields() return copies
* Key.Compare() is correct
* EncodeUint64/DecodeUint64 round-trip verified
* EncodeInt64/DecodeInt64 round-trip with sign bit flip verified
* scalar decoders reject composite keys via len(fields)==1 check
* integer decoders require field.Length == 8 exactly
* DecodeSortComposite and DecodeStorageComposite validate field
  bounds and exact integer field length before decoding
* EncodeString/DecodeString round-trip with null termination verified
* EncodeBytes/DecodeBytes round-trip with empty/nil verified
* EncodeSortComposite integer-only sort order verified
* EncodeSortComposite rejects strings and bytes
* EncodeStorageComposite variable-length round-trip verified
* storage composite documented and tested as NOT sort-safe
* sort and storage composites are byte-distinct
* ParseKey round-trip verified for all scalar and sort composite types
* ParseStorageCompositeKey round-trip verified
* decoder validation uses field metadata not raw data length
* zero internal imports confirmed
* no LE variants confirmed
* no external dependencies added
* go test ./... passes
* go build ./... passes
* go test -race ./... passes
* go mod tidy produces no unwanted changes
* final go test ./... passes
* no non-goal systems introduced

---

## Recommended Next Step After Completion

After `sql_key_setup.md` is completed and verified, the SQL engine has
its single authoritative key encoding foundation.

Do not automatically move to the next feature. Continue with the
selected database foundation roadmap as confirmed by the project owner.

Possible next directions:

```text
sql_key_enterprise.md   — hardening, edge cases, fuzzing
kv_store_setup.md       — in-memory KV store using sql/key
storage_setup.md        — page-based file storage
```