# Plomvix SQL Key Encoding

The `sql/key` package is the single authoritative key encoder for the Plomvix
SQL engine. It has **zero internal imports** and uses only the Go standard
library.

## Key struct

The `Key` struct holds encoded bytes and field metadata:

- `Bytes()` — returns a copy of the encoded bytes
- `Fields()` — returns a copy of `[]Field` metadata
- `Compare(other)` — lexicographic comparison of encoded bytes

## Field struct

Each `Field` describes one encoded component:

- `Kind` — type identifier
- `Offset` — byte offset into `Key.data`
- `Length` — number of bytes this field occupies

## Kind

| Kind | Value | Encoding |
|------|-------|----------|
| `KindUint64` | 1 | 8-byte big-endian |
| `KindInt64` | 2 | 8-byte big-endian with sign bit flip |
| `KindString` | 3 | null-terminated |
| `KindBytes` | 4 | raw bytes, no framing |

## uint64

Encoded as 8 bytes big-endian. Lexicographic byte order equals numeric order.

## int64

Encoded as 8 bytes big-endian with the **sign bit flip**: `v XOR (1 << 63)`.
This makes negatives sort before positives under unsigned byte comparison.

## String (null-terminated)

Standalone `string` keys are null-terminated (`0x00`). Strings containing
embedded null bytes are rejected with `ErrNullByteInString`.

## Bytes

Raw `[]byte` keys have no framing or delimiter. Empty and nil slices are valid.

## EncodeSortComposite

Fixed-width integer-only composite keys. Accepts only `uint64` and `int64`.
Strings and bytes are rejected with `ErrUnsupportedSortType`.

Sort order: fixed-width concatenation preserves lexicographic order across
fields. Compare compares first field, then second, etc.

## EncodeStorageComposite

Variable-length composite keys with **length-prefix** framing. Each field is
prefixed with a 4-byte big-endian `uint32` length. String fields do **not**
use null termination inside storage composites.

**storage composite is not sort-safe** — do not use as index keys or scan
boundaries.

## ParseKey

Schema-driven decode from raw bytes. The caller provides the expected `Kind`
slice. Used when reading keys from persistent storage where only raw bytes
are available.

## ParseStorageCompositeKey

Schema-driven decode for storage composites, reading 4-byte length-prefix
headers to reconstruct field metadata.

## Compare

Uses `bytes.Compare` on the raw encoded data for sort order and equality
checks.

## Enterprise Hardening

The key encoding package has been hardened with **enterprise hardening**:

- **validateField** — centralized overflow-safe bounds and length validation
  used by all decoders
- **fuzz testing** — `ParseKey` and `ParseStorageCompositeKey` are fuzzed to
  guarantee they **never panic** on arbitrary malformed input
- **benchmarks** — hot encode/decode paths are benchmarked for regression
  detection
- **no API changes** — all public API signatures and behavior are unchanged
- **no wire format changes** — all existing byte encodings are unchanged

## Non-Goals

The key encoding package intentionally does not implement:

- KV store
- WAL
- storage pages
- query execution
- indexes
- transaction IDs
- version stamps
- compression
- **no little-endian** variants
