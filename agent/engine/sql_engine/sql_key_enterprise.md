# sql_key_enterprise.md

# Plomvix SQL Key Encoding Enterprise Hardening Plan

## Purpose

Harden the existing `internal/engine/sql/key` package into a safer,
more maintainable, production-grade foundation.

This plan centralizes field validation, adds fuzz testing for the
two byte-parsing entry points, and adds benchmarks for the hot
encode/decode paths.

This is still key-encoding hardening only. No new key types, no new
composite formats, no API removals.

Do not add a KV store.
Do not add WAL.
Do not add storage pages.
Do not add query execution.
Do not add API server.
Do not add UI.
Do not add little-endian variants.
Do not wire key encoding into lifecycle or runtime.

---

## Feature Name

```text
SQL Key Encoding Enterprise Hardening
```

Plan file:

```text
sql_key_enterprise.md
```

Existing package:

```text
internal/engine/sql/key
```

---

## Required Starting State

This plan starts only after `sql_key_setup.md` is completed and
verified.

Before starting this plan, the project must already have:

```text
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
internal/engine/sql/key/key_internal_test.go
docs/sql_key.md
```

The key package must already expose:

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

func EncodeUint64(v uint64) Key
func EncodeInt64(v int64) Key
func EncodeString(s string) (Key, error)
func EncodeBytes(b []byte) Key
func EncodeSortComposite(fields ...any) (Key, error)
func EncodeStorageComposite(fields ...any) (Key, error)

func DecodeUint64(k Key) (uint64, error)
func DecodeInt64(k Key) (int64, error)
func DecodeString(k Key) (string, error)
func DecodeBytes(k Key) ([]byte, error)
func DecodeSortComposite(k Key) ([]any, error)
func DecodeStorageComposite(k Key) ([]any, error)

func ParseKey(data []byte, kinds []Kind) (Key, error)
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)

func (k Key) Bytes() []byte
func (k Key) Compare(other Key) int
func (k Key) Fields() []Field
```

Existing public errors must include:

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

Existing behavior must already include:

* big-endian sort-safe uint64 and int64 encoding
* sign-bit-flip int64 sort order
* null-terminated standalone strings
* sort-safe integer-only composite via EncodeSortComposite
* non-sort-safe variable-length composite via EncodeStorageComposite
* schema-driven decode via ParseKey and ParseStorageCompositeKey
* scalar decoders reject composite keys (len(fields) == 1 check)
* integer decoders reject fields with Length != 8
* zero internal imports
* no little-endian variants
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes

If this starting state is not true, stop and report that
`sql_key_setup.md` is incomplete.

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
SQL key encoding:               done
```

Current stage:

```text
SQL key encoding hardening
```

Current feature area:

```text
sql/key enterprise hardening
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

Use only Go standard library.

Do not add external dependencies.

Go's built-in fuzzing (`go test -fuzz`) has been stable since Go 1.18
and is fully available under Go 1.22.

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
* Do not remove or rename any existing public API.
* Do not change the byte format of any existing encoding.
* Do not import internal/config, internal/logger, internal/lifecycle,
  or internal/runtime from the key package.
* Do not add external dependencies.
* Use only Go standard library.
* Keep tests deterministic except where fuzz testing is explicitly
  required.
* Use table-driven tests where useful.
* Do not create a root-level `tests/` directory.
* Do not add a KV store, WAL, or storage pages in this plan.
* Do not add little-endian variants.

---

## Dependency Direction Rules

The key package must continue to have zero internal imports:

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
testing
```

`testing` is allowed only in `_test.go` files for benchmarks and
fuzz tests.

---

## Enterprise Hardening Goals

This plan adds:

* centralized field validation helper
* refactored decoders using the shared helper
* Go fuzz tests for ParseKey
* Go fuzz tests for ParseStorageCompositeKey
* benchmarks for scalar encode/decode hot paths
* benchmarks for composite encode/decode hot paths
* hardened documentation
* final scope-control review

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
* little-endian encoding
* new Kind values
* new composite formats
* changes to existing byte wire formats
* config, logger, lifecycle, or runtime integration

---

## Design Decisions

### Centralized Field Validation

Add an unexported helper:

```go
func validateField(data []byte, f Field) error
```

Behavior:

* if `f.Offset < 0` or `f.Length < 0`: return ErrInvalidKey
* if `f.Offset > len(data)`: return ErrInvalidKey
* if `f.Length > len(data)-f.Offset`: return ErrInvalidKey
* if `f.Kind == KindUint64` or `f.Kind == KindInt64`:
  * if `f.Length != 8`: return ErrInvalidKey
* return nil

Bounds checks must use this subtraction-based pattern, never
`f.Offset+f.Length > len(data)`, which can overflow and wrap
around for very large Offset and Length values, incorrectly
passing validation. Checking `Offset` against `len(data)` first,
then `Length` against the remaining space `len(data)-f.Offset`,
avoids this overflow entirely.

This single function replaces the inline bounds and length checks
currently duplicated across `DecodeUint64`, `DecodeInt64`,
`DecodeString`, `DecodeBytes`, `DecodeSortComposite`, and
`DecodeStorageComposite`.

Every decoder calls `validateField` once per field before reading
any bytes from that field.

String and bytes kinds are validated by the bounds check alone,
since they are variable-length by design and have no fixed-length
requirement.

This is a pure refactor. No decoder's externally observable
behavior changes. Every existing test must continue to pass
unmodified.

### Fuzz Testing

Go's native fuzzing support is used to fuzz the two functions that
accept raw, potentially untrusted or corrupted byte input:

```go
func ParseKey(data []byte, kinds []Kind) (Key, error)
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)
```

These are the persistence boundary functions: any byte sequence
that could come from disk, a corrupted write, or a future network
transport eventually flows through one of these two functions.

Fuzz target behavior requirement:

```text
Neither ParseKey nor ParseStorageCompositeKey may panic for any
input, regardless of how malformed. They must always either return
a valid Key and nil error, or return a zero Key and a non-nil error.
```

Fuzz tests use a fixed, small set of `Kind` slices as seed corpus
combined with fuzzed `data []byte`, since `kinds` is a caller-known
schema and is not itself the untrusted input in this design — only
`data` is potentially corrupted.

### Benchmarks

Benchmarks are added only for the hot paths that will be called
on every key construction and lookup once a KV store exists:

```text
BenchmarkEncodeUint64
BenchmarkDecodeUint64
BenchmarkEncodeSortComposite
BenchmarkDecodeSortComposite
BenchmarkEncodeStorageComposite
BenchmarkDecodeStorageComposite
```

This is intentionally a small, fixed set. Do not add a benchmark
for every function in the package. The goal is a regression guard
on the paths that matter for throughput, not exhaustive coverage.

---

## Task Plan

---

## TASK 01 — Add validateField helper

### Goal

Add the centralized field validation helper without yet wiring it
into any decoder.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

Add unexported helper:

```go
func validateField(data []byte, f Field) error {
    if f.Offset < 0 || f.Length < 0 {
        return ErrInvalidKey
    }
    if f.Offset > len(data) {
        return ErrInvalidKey
    }
    if f.Length > len(data)-f.Offset {
        return ErrInvalidKey
    }
    switch f.Kind {
    case KindUint64, KindInt64:
        if f.Length != 8 {
            return ErrInvalidKey
        }
    }
    return nil
}
```

Overflow safety rule:

```text
Do not check bounds using f.Offset+f.Length > len(data).
If Offset and Length are both very large (for example near
math.MaxInt), their sum can overflow and wrap to a small or
negative number, which would incorrectly pass the bounds check
and let a malformed field slip through.

Instead check Offset against len(data) first, then check Length
against the remaining space (len(data)-f.Offset), which is already
known to be non-negative at that point and cannot overflow in this
direction.
```

Do not call this helper from any decoder yet in this task.

Do not change any existing decoder behavior in this task.

Do not remove the existing inline checks yet.

### Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

```text
All existing tests pass unmodified.
Build succeeds.
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

## TASK 02 — Add validateField tests

### Goal

Verify `validateField` behavior directly before it is wired into
any decoder.

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

Add table-driven tests for `validateField`:

```text
valid uint64 field, exact bounds            → nil
valid int64 field, exact bounds              → nil
valid string field, length less than data    → nil
valid bytes field, zero length               → nil
negative Offset                              → ErrInvalidKey
negative Length                              → ErrInvalidKey
Offset+Length exceeds len(data)               → ErrInvalidKey
KindUint64 with Length == 7                   → ErrInvalidKey
KindUint64 with Length == 9                   → ErrInvalidKey
KindInt64 with Length == 4                    → ErrInvalidKey
KindString with Length == 100, short data     → ErrInvalidKey
```

Overflow-proof test cases:

```text
Offset == math.MaxInt, Length == 1            → ErrInvalidKey
Offset == math.MaxInt, Length == math.MaxInt  → ErrInvalidKey
Offset == 0, Length == math.MaxInt            → ErrInvalidKey
Offset == math.MaxInt-1, Length == 2          → ErrInvalidKey
```

These cases specifically prove that very large Offset and Length
values, which would overflow if checked using
`f.Offset+f.Length > len(data)`, are still correctly rejected and
do not bypass validation through integer wraparound.

Use `errors.Is` for all error checks.

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
- internal/engine/sql/key/key_internal_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Wire validateField into scalar decoders

### Goal

Refactor `DecodeUint64`, `DecodeInt64`, `DecodeString`, and
`DecodeBytes` to use `validateField` instead of inline checks.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

For each of the four scalar decoders:

* keep the existing `len(k.fields) != 1` check as-is
* keep the existing `Kind` mismatch check as-is
* replace the inline bounds/length check with a single call:

```go
if err := validateField(k.data, field); err != nil {
    return zeroValue, err
}
```

Required order of checks per decoder:

```text
1. zero Key check (no fields and no data) → ErrInvalidKey
2. len(k.fields) != 1                     → ErrInvalidKey
3. Kind mismatch                          → ErrKindMismatch
4. validateField(k.data, field)           → propagate its error
5. proceed to read and decode bytes
```

This order must not change. Kind mismatch must still be reported
before bounds/length errors, since a caller passing the wrong kind
of key should see ErrKindMismatch, not a bounds error that could be
confusing in that context.

Do not change the public signature of any decoder.

Do not change any returned error type for any existing passing test
case. This is a pure internal refactor.

### Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

```text
All existing tests from sql_key_setup.md continue to pass
unmodified, since this is a behavior-preserving refactor.
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

## TASK 04 — Wire validateField into composite decoders

### Goal

Refactor `DecodeSortComposite` and `DecodeStorageComposite` to use
`validateField` for each field instead of inline checks.

### Files

Modify:

```text
internal/engine/sql/key/key.go
```

### Requirements

For both composite decoders:

* keep the existing `len(k.fields) == 0` → ErrNotComposite check
* inside the per-field loop, call `validateField` first, before the
  Kind switch that reconstructs the value:

```go
for _, field := range k.Fields() {
    if err := validateField(k.data, field); err != nil {
        return nil, err
    }
    switch field.Kind {
    // existing Kind switch unchanged
    }
}
```

* keep the existing `Kind` switch for value reconstruction unchanged
* keep the existing `default: return nil, ErrKindMismatch` case
* `validateField` must run before the Kind switch on every iteration,
  so no field is decoded before its bounds and length are confirmed
  valid

This removes the duplicated bounds/length logic that previously
existed separately in both functions.

Do not change the public signature of either decoder.

Do not change any returned error type for any existing passing test
case.

The defensive tests added in `sql_key_setup.md` TASK 12B and
TASK 14B (which construct malformed Keys directly via unexported
fields) must continue to pass against the refactored code, since
`validateField` enforces the same rules those tests expect.

### Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

```text
All existing tests, including the TASK 12B and TASK 14B defensive
tests from sql_key_setup.md, continue to pass unmodified.
```

### Completion Report

```text
TASK 04 completed.
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

## TASK 05 — Add ParseKey fuzz test

### Goal

Add a Go native fuzz test for `ParseKey` that proves it never panics
on arbitrary byte input.

### Files

Create:

```text
internal/engine/sql/key/fuzz_test.go
```

### Package

```go
package key_test
```

### Requirements

Add a fuzz test function:

```go
func FuzzParseKey(f *testing.F)
```

Seed corpus must include at least these cases:

```text
empty data with []Kind{KindUint64}
8 bytes of zeros with []Kind{KindUint64}
8 bytes of zeros with []Kind{KindInt64}
a valid encoded uint64 key's raw bytes with []Kind{KindUint64}
a valid encoded string key's raw bytes with []Kind{KindString}
a valid encoded bytes key's raw bytes with []Kind{KindBytes}
truncated string data with no null terminator, []Kind{KindString}
random short byte slices (1-3 bytes) with []Kind{KindUint64}
```

Because `ParseKey` takes both `data []byte` and `kinds []Kind`,
and Go's fuzzer only fuzzes primitive and byte/string types
directly, structure the fuzz target to fuzz `data` while cycling
through a small fixed set of representative `kinds` slices defined
in the test itself:

```go
func FuzzParseKey(f *testing.F) {
    kindSets := [][]key.Kind{
        {key.KindUint64},
        {key.KindInt64},
        {key.KindString},
        {key.KindBytes},
        {key.KindUint64, key.KindString},
        {key.KindString, key.KindBytes},
    }

    f.Add([]byte{})
    f.Add(make([]byte, 8))
    // add remaining seed corpus bytes here

    f.Fuzz(func(t *testing.T, data []byte) {
        for _, kinds := range kindSets {
            k, err := key.ParseKey(data, kinds)
            if err != nil {
                continue
            }
            // if ParseKey succeeded, decoding via the matching
            // Decode function must not panic either
            if len(kinds) == 1 {
                switch kinds[0] {
                case key.KindUint64:
                    _, _ = key.DecodeUint64(k)
                case key.KindInt64:
                    _, _ = key.DecodeInt64(k)
                case key.KindString:
                    _, _ = key.DecodeString(k)
                case key.KindBytes:
                    _, _ = key.DecodeBytes(k)
                }
            } else {
                _, _ = key.DecodeSortComposite(k)
            }
        }
    })
}
```

Rules:

* the fuzz function must never call `t.Fatal` or `t.Error` for a
  returned error alone — only a panic is a failure
* a returned error from `ParseKey` or any `DecodeXxx` call is
  expected and acceptable for malformed input
* only an actual Go panic (caught by the fuzzer itself) is a
  reportable failure
* do not assert specific error types in the fuzz function body,
  only that no panic occurs

### Verification

Run:

```bash
go test ./...
go build ./...
go test -fuzz=FuzzParseKey -fuzztime=30s ./internal/engine/sql/key/
```

Expected:

```text
go test ./... passes (fuzz tests run as regular tests using only
  the seed corpus when invoked without -fuzz).
The explicit fuzz run for 30 seconds finds no panics.
```

### Completion Report

```text
TASK 05 completed.
Files changed:
- internal/engine/sql/key/fuzz_test.go

Verification:
- go test ./...
- go build ./...
- go test -fuzz=FuzzParseKey -fuzztime=30s ./internal/engine/sql/key/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add ParseStorageCompositeKey fuzz test

### Goal

Add a Go native fuzz test for `ParseStorageCompositeKey` that
proves it never panics on arbitrary byte input.

### Files

Modify:

```text
internal/engine/sql/key/fuzz_test.go
```

### Requirements

Add a second fuzz test function:

```go
func FuzzParseStorageCompositeKey(f *testing.F)
```

Seed corpus must include at least these cases:

```text
empty data with []Kind{KindUint64}
a valid encoded storage composite's raw bytes with matching kinds
4 bytes claiming a huge length (e.g. 0xFFFFFFFF) followed by
  short actual data, with []Kind{KindString}
truncated length prefix (1-3 bytes) with []Kind{KindBytes}
a valid composite truncated mid-field
```

Use the same kind-set cycling approach as TASK 05:

```go
func FuzzParseStorageCompositeKey(f *testing.F) {
    kindSets := [][]key.Kind{
        {key.KindUint64},
        {key.KindString},
        {key.KindBytes},
        {key.KindUint64, key.KindString, key.KindBytes},
    }

    f.Add([]byte{})
    // add remaining seed corpus bytes here

    f.Fuzz(func(t *testing.T, data []byte) {
        for _, kinds := range kindSets {
            k, err := key.ParseStorageCompositeKey(data, kinds)
            if err != nil {
                continue
            }
            _, _ = key.DecodeStorageComposite(k)
        }
    })
}
```

Rules:

* same as TASK 05: only a panic is a reportable failure
* returned errors are expected and acceptable
* do not assert specific error types

This fuzz target is particularly important because the 4-byte
length prefix in storage composite framing is exactly the kind of
field that, if read without bounds checking, could attempt to
allocate or read a huge or negative-looking length from corrupted
data.

### Verification

Run:

```bash
go test ./...
go build ./...
go test -fuzz=FuzzParseStorageCompositeKey -fuzztime=30s ./internal/engine/sql/key/
```

Expected:

```text
go test ./... passes.
The explicit fuzz run for 30 seconds finds no panics.
```

### Completion Report

```text
TASK 06 completed.
Files changed:
- internal/engine/sql/key/fuzz_test.go

Verification:
- go test ./...
- go build ./...
- go test -fuzz=FuzzParseStorageCompositeKey -fuzztime=30s ./internal/engine/sql/key/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Add scalar encode/decode benchmarks

### Goal

Add benchmarks for the scalar encode and decode hot paths.

### Files

Create:

```text
internal/engine/sql/key/bench_test.go
```

### Package

```go
package key_test
```

### Requirements

Add benchmarks:

```go
func BenchmarkEncodeUint64(b *testing.B)
func BenchmarkDecodeUint64(b *testing.B)
```

Standard benchmark shape:

```go
func BenchmarkEncodeUint64(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _ = key.EncodeUint64(uint64(i))
    }
}

func BenchmarkDecodeUint64(b *testing.B) {
    k := key.EncodeUint64(42)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = key.DecodeUint64(k)
    }
}
```

Rules:

* construct any fixed setup data before the benchmark loop starts
* call `b.ResetTimer()` after setup if setup work is non-trivial
* do not include `b.N`-independent setup inside the timed loop
* benchmarks must compile and run under `go test -bench=. -run=^$`

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=. -run=^$ ./internal/engine/sql/key/
```

Expected:

```text
go test ./... passes.
Benchmarks run and report ns/op without errors.
```

### Completion Report

```text
TASK 07 completed.
Files changed:
- internal/engine/sql/key/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=. -run=^$ ./internal/engine/sql/key/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add composite encode/decode benchmarks

### Goal

Add benchmarks for the sort composite and storage composite hot
paths.

### Files

Modify:

```text
internal/engine/sql/key/bench_test.go
```

### Requirements

Add benchmarks:

```go
func BenchmarkEncodeSortComposite(b *testing.B)
func BenchmarkDecodeSortComposite(b *testing.B)
func BenchmarkEncodeStorageComposite(b *testing.B)
func BenchmarkDecodeStorageComposite(b *testing.B)
```

Use representative field counts and types:

```text
EncodeSortComposite benchmark: 3 fields, mix of uint64 and int64
EncodeStorageComposite benchmark: 3 fields, mix of uint64, string,
  and []byte, with the string around 16 bytes and the []byte
  around 32 bytes
```

Follow the same benchmark shape pattern as TASK 07: construct fixed
inputs before the loop, call `b.ResetTimer()` if setup is non-trivial,
only time the actual encode or decode call inside the loop.

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=. -run=^$ ./internal/engine/sql/key/
```

Expected:

```text
go test ./... passes.
All six benchmarks from TASK 07 and TASK 08 run and report ns/op.
```

### Completion Report

```text
TASK 08 completed.
Files changed:
- internal/engine/sql/key/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=. -run=^$ ./internal/engine/sql/key/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 09 — Harden documentation

### Goal

Document the enterprise hardening additions in the existing key
encoding documentation.

### Files

Modify:

```text
docs/sql_key.md
```

### Requirements

Add a new section documenting:

* centralized field validation via validateField
* fuzz testing for ParseKey and ParseStorageCompositeKey
* the guarantee that these parsers never panic on malformed input
* benchmarks exist for hot encode/decode paths
* this hardening did not change any existing public API
* this hardening did not change any existing byte wire format

The documentation must include these exact strings because TASK 10
checks them:

```text
enterprise hardening
validateField
fuzz testing
never panic
benchmarks
no API changes
no wire format changes
```

Do not document future behavior as already implemented.

Do not remove any existing required strings from the original
`sql_key_setup.md` documentation task. This is an addition, not a
replacement.

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
- docs/sql_key.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Add documentation tests for hardening section

### Goal

Verify the new hardening documentation section exists and contains
required content.

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

Extend the existing documentation test (added in `sql_key_setup.md`
TASK 17) to also check for these additional strings:

```text
enterprise hardening
validateField
fuzz testing
never panic
benchmarks
no API changes
no wire format changes
```

Do not remove any of the existing required string checks from the
original documentation test. Add to the same check, do not create
a second documentation test function.

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

## TASK 11 — Final hardening review

### Goal

Review the hardening work for correctness, completeness, scope
control, and project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
internal/engine/sql/key/key_internal_test.go
internal/engine/sql/key/fuzz_test.go
internal/engine/sql/key/bench_test.go
docs/sql_key.md
go.mod
go.sum
```

### Requirements

Confirm:

* `validateField` exists and is unexported
* `validateField` checks Offset and Length against len(data) using
  the overflow-safe subtraction pattern, not addition
* `validateField` is used by all six decoders
* no decoder contains the old duplicated inline bounds/length checks
* scalar decoders preserve their check order:
  zero-key check, then len(fields)==1 check, then Kind check, then
  validateField, then decode
* composite decoders preserve their existing order:
  len(fields)==0 → ErrNotComposite check first, then for each field
  in the loop: validateField before decoding or switching on Kind
* all existing public API signatures are unchanged
* all existing byte wire formats are unchanged
* all tests from `sql_key_setup.md` still pass unmodified
* the TASK 12B and TASK 14B defensive tests from `sql_key_setup.md`
  still pass against the refactored decoders
* `FuzzParseKey` exists with a non-trivial seed corpus
* `FuzzParseStorageCompositeKey` exists with a non-trivial seed corpus
* both fuzz functions only fail on panic, never on returned errors
* a 30-second fuzz run of each target completes with no panics found
* six benchmarks exist: scalar encode/decode, sort composite
  encode/decode, storage composite encode/decode
* benchmarks compile and run under `go test -bench=. -run=^$`
* documentation includes the new hardening section
* documentation test covers the new hardening strings
* original documentation strings from `sql_key_setup.md` are still
  present and tested
* no new Kind values were added
* no new composite formats were added
* no LE variants were added
* no external dependencies were added
* zero internal imports confirmed
* `go.mod` unchanged
* `go.sum` unchanged

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
go test -fuzz=FuzzParseKey -fuzztime=30s ./internal/engine/sql/key/
go test -fuzz=FuzzParseStorageCompositeKey -fuzztime=30s ./internal/engine/sql/key/
go test -bench=. -run=^$ ./internal/engine/sql/key/
go mod tidy
go test ./...
```

### Completion Report

```text
TASK 11 completed.
Files reviewed:
- internal/engine/sql/key/key.go
- internal/engine/sql/key/key_test.go
- internal/engine/sql/key/key_internal_test.go
- internal/engine/sql/key/fuzz_test.go
- internal/engine/sql/key/bench_test.go
- docs/sql_key.md
- go.mod
- go.sum

Final verification:
- go test ./...
- go build ./...
- go test -race ./...
- go test -fuzz=FuzzParseKey -fuzztime=30s
- go test -fuzz=FuzzParseStorageCompositeKey -fuzztime=30s
- go test -bench=. -run=^$
- go mod tidy
- go test ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable

Final status:
- sql/key enterprise hardening complete
- field validation centralized
- no panics found via 30s fuzz runs on both parsers
- benchmarks established for all hot paths
- no public API or wire format changes
- no non-goal systems introduced
```

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
│               ├── key_internal_test.go
│               ├── fuzz_test.go
│               └── bench_test.go
├── docs/
│   └── sql_key.md
```

No new folders are required.

---

## Completion Criteria

This plan is complete only when:

* `validateField` exists, is unexported, and is used by all six
  decoders
* `validateField` uses overflow-safe bounds checks and never uses
  `f.Offset+f.Length` for validation
* check ordering in each decoder is preserved
* no public API signature changed
* no existing byte wire format changed
* all `sql_key_setup.md` tests still pass unmodified
* `FuzzParseKey` exists and a 30-second run finds no panics
* `FuzzParseStorageCompositeKey` exists and a 30-second run finds
  no panics
* six benchmarks exist and run successfully
* documentation hardening section exists and is tested
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted changes
* final `go test ./...` passes
* no non-goal systems introduced

---

## Recommended Next Step After Completion

After `sql_key_enterprise.md` is completed and verified, the SQL
key encoding foundation is complete and hardened.

Do not automatically move to the next feature. Continue with the
selected database foundation roadmap as confirmed by the project
owner.

Possible next directions:

```text
kv_store_setup.md       — in-memory KV store using sql/key
storage_setup.md        — page-based file storage
```