# Plomvix SQL Engine Key Encoding

The key-encoding layer turns logical table identifiers and primary-key column
values into order-preserving byte keys suitable for sorted KV stores.

## Key Layout

Every table row key has four parts concatenated in fixed order:

```
[ keyspace tag 1B ][ tableID 8B BE ][ encoded PK columns ][ version 8B ]
```

### 1. Keyspace Tag

One byte separating key categories so they never collide:

| Tag | Purpose |
|-----|---------|
| `0x01` | Table row data |
| `0x02` | Engine metadata (reserved) |
| `0x03` | Secondary index data (reserved) |

### 2. Table ID

8 bytes, big-endian `uint64`. Big-endian so byte comparison equals numeric
order. Table ID assignment is external to this layer.

### 3. Encoded Primary-Key Columns

Each column is encoded with a **1-byte type tag** followed by an
order-preserving payload:

| Tag | Type | Encoding |
|-----|------|----------|
| `0x10` | Null | No payload |
| `0x20` | Bool | 1 byte: `0x00` false, `0x01` true |
| `0x30` | Int64 | 8 bytes BE of `value XOR 0x8000000000000000` (sign-bit flip) |
| `0x40` | Uint64 | 8 bytes BE, no transformation |
| `0x50` | String | Escape-terminated (see below) |
| `0x60` | Bytes | Escape-terminated (see below) |

Tags ascend so that: null < bool < int64 < uint64 < string < bytes under raw
byte comparison.

### Escape-and-Terminate Scheme

Variable-length fields (string/bytes) use this encoding:

- Each input byte `0x00` → encoded as `0x00 0xFF` (escape)
- All other bytes → written unchanged
- Field terminated by `0x00 0x01`

This is **unambiguous**: the terminator `0x00 0x01` can never be confused with
an escaped zero `0x00 0xFF`, since `0x01 != 0xFF`.

It is **order-preserving**: a shorter string is a prefix of a longer one. At
the position where the shorter terminates (`0x00 0x01`), the longer has its
next real byte, and `0x01 < any non-zero byte`, so shorter sorts first.

### 4. Version

8 bytes. Stored as bitwise-inverted big-endian `uint64` (encode `v` as `^v`).
This makes **newer versions sort before older** ones, so a forward scan finds
the latest version first.

## Decoding Requires Known PK Arity

A row key does **not** store the number of PK columns. The caller must supply
the expected PK column kinds (from the table schema). The decoder:

1. Reads exactly `len(expectedKinds)` PK columns
2. Verifies each column's type tag matches the expected kind
3. Reads exactly the 8-byte version
4. Requires full input consumption (extra bytes → `ErrTrailingBytes`)

## Worked Example

Table 7, PK = `("ab", int64 5)`, version 1:

```
01                          keyspace tag
00 00 00 00 00 00 00 07     tableID = 7
50                          string tag
61 62                       "ab"
00 01                       terminator
30                          int64 tag
80 00 00 00 00 00 00 05     int64 5 (sign-bit flipped)
FF FF FF FF FF FF FF FE     version (^1)
```

## Sentinel Errors

- `ErrEmptyKey` — empty input to decode
- `ErrBadTag` — unknown keyspace tag
- `ErrBadTypeTag` — unknown column type tag
- `ErrKindMismatch` — decoded column kind doesn't match expected
- `ErrTruncated` — input too short
- `ErrBadField` — malformed variable-length field (invalid escape byte)
- `ErrTrailingBytes` — extra bytes after a valid key
- `ErrNoPKColumns` — at least one PK column required
- `ErrNotCanonical` — encoding is not in canonical form

## Format Stability

The key format defined in this document **is frozen**. The golden vectors in
`internal/engine/sql/key/key_test.go` are the **authoritative lock**: any change
that alters encoded output will cause those tests to fail — this is the intended
early warning.

### Rules

1. The existing `0x01` (table row data) layout must **never change meaning**.
2. Any intentional format change requires a **new keyspace tag** (e.g. `0x04`)
   or an explicit **versioned-format migration plan**.
3. The reserved tags `0x02` (metadata) and `0x03` (index) remain reserved
   for future features with their own format specs.

### Backward Compatibility

Keys written by this version of the `0x01` format must remain **decodable**
by all future versions of the same format. New fields, if ever needed, must be
added through new keyspace tags rather than by altering the existing layout.
