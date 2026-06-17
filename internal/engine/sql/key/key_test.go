package key_test

import (
	"bytes"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

func TestKindConstants(t *testing.T) {
	if key.KindUint64 != 1 {
		t.Errorf("KindUint64 = %d", key.KindUint64)
	}
	if key.KindInt64 != 2 {
		t.Errorf("KindInt64 = %d", key.KindInt64)
	}
	if key.KindString != 3 {
		t.Errorf("KindString = %d", key.KindString)
	}
	if key.KindBytes != 4 {
		t.Errorf("KindBytes = %d", key.KindBytes)
	}
}

func TestErrorSentinels(t *testing.T) {
	if key.ErrNullByteInString.Error() != "sql/key: string contains null byte" {
		t.Error("ErrNullByteInString")
	}
	if key.ErrUnsupportedType.Error() != "sql/key: unsupported field type" {
		t.Error("ErrUnsupportedType")
	}
	if key.ErrUnsupportedSortType.Error() != "sql/key: type not allowed in sort composite" {
		t.Error("ErrUnsupportedSortType")
	}
	if key.ErrInvalidKey.Error() != "sql/key: invalid key" {
		t.Error("ErrInvalidKey")
	}
	if key.ErrKindMismatch.Error() != "sql/key: kind mismatch" {
		t.Error("ErrKindMismatch")
	}
	if key.ErrNotComposite.Error() != "sql/key: key is not composite" {
		t.Error("ErrNotComposite")
	}
}

func TestZeroKey(t *testing.T) {
	var k key.Key
	if k.Bytes() != nil {
		t.Error("zero Key Bytes should be nil")
	}
	if k.Compare(k) != 0 {
		t.Error("zero Key Compare(self) should be 0")
	}
	if k.Fields() != nil {
		t.Error("zero Key Fields should be nil")
	}
}

func TestUint64RoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, math.MaxUint64, math.MaxUint64 / 2} {
		k := key.EncodeUint64(v)
		got, err := key.DecodeUint64(k)
		if err != nil || got != v {
			t.Errorf("uint64 %d: got %d, err %v", v, got, err)
		}
	}
}

func TestInt64RoundTrip(t *testing.T) {
	for _, v := range []int64{0, 1, -1, math.MinInt64, math.MaxInt64} {
		k := key.EncodeInt64(v)
		got, err := key.DecodeInt64(k)
		if err != nil || got != v {
			t.Errorf("int64 %d: got %d, err %v", v, got, err)
		}
	}
}

func TestUint64SortOrder(t *testing.T) {
	assertAscending(t, []key.Key{
		key.EncodeUint64(0), key.EncodeUint64(1),
		key.EncodeUint64(255), key.EncodeUint64(256),
	})
}

func TestInt64SortOrder(t *testing.T) {
	assertAscending(t, []key.Key{
		key.EncodeInt64(math.MinInt64), key.EncodeInt64(-1),
		key.EncodeInt64(0), key.EncodeInt64(1),
		key.EncodeInt64(math.MaxInt64),
	})
}

func TestDecodeZeroKey(t *testing.T) {
	var k key.Key
	_, err := key.DecodeUint64(k)
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("DecodeUint64(zero): %v", err)
	}
	_, err = key.DecodeInt64(k)
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("DecodeInt64(zero): %v", err)
	}
}

func TestDecodeKindMismatch(t *testing.T) {
	_, err := key.DecodeUint64(key.EncodeInt64(1))
	if !errors.Is(err, key.ErrKindMismatch) {
		t.Errorf("got %v", err)
	}
	_, err = key.DecodeInt64(key.EncodeUint64(1))
	if !errors.Is(err, key.ErrKindMismatch) {
		t.Errorf("got %v", err)
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, s := range []string{"", "hello", "plomvix", "a\x01b"} {
		k, err := key.EncodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		got, err := key.DecodeString(k)
		if err != nil || got != s {
			t.Errorf("string %q: got %q, err %v", s, got, err)
		}
	}
}

func TestStringNullByteRejected(t *testing.T) {
	if _, err := key.EncodeString("hel\x00lo"); !errors.Is(err, key.ErrNullByteInString) {
		t.Errorf("got %v", err)
	}
	if _, err := key.EncodeString("\x00"); !errors.Is(err, key.ErrNullByteInString) {
		t.Errorf("got %v", err)
	}
}

func TestBytesRoundTrip(t *testing.T) {
	for _, b := range [][]byte{nil, {}, {0x00, 0x01, 0x02}, {0x00}} {
		k := key.EncodeBytes(b)
		got, err := key.DecodeBytes(k)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, b) && !(b == nil && len(got) == 0) {
			t.Errorf("bytes %v: got %v", b, got)
		}
	}
}

func TestStringSortOrder(t *testing.T) {
	ks := make([]key.Key, 0)
	for _, s := range []string{"", "a", "aa", "b", "z"} {
		k, _ := key.EncodeString(s)
		ks = append(ks, k)
	}
	assertAscending(t, ks)
}

func TestDecodeStringWrongKind(t *testing.T) {
	_, err := key.DecodeString(key.EncodeUint64(1))
	if !errors.Is(err, key.ErrKindMismatch) {
		t.Errorf("got %v", err)
	}
}

func TestParseKeyUint64(t *testing.T) {
	raw := key.EncodeUint64(42).Bytes()
	k, err := key.ParseKey(raw, []key.Kind{key.KindUint64})
	if err != nil {
		t.Fatal(err)
	}
	v, err := key.DecodeUint64(k)
	if err != nil || v != 42 {
		t.Errorf("got %d, err %v", v, err)
	}
}

func TestParseKeyString(t *testing.T) {
	raw, _ := key.EncodeString("hi")
	k, err := key.ParseKey(raw.Bytes(), []key.Kind{key.KindString})
	if err != nil {
		t.Fatal(err)
	}
	s, err := key.DecodeString(k)
	if err != nil || s != "hi" {
		t.Errorf("got %q, err %v", s, err)
	}
}

func TestParseKeyErrors(t *testing.T) {
	if _, err := key.ParseKey(nil, nil); !errors.Is(err, key.ErrInvalidKey) {
		t.Error("empty kinds")
	}
	if _, err := key.ParseKey([]byte{0, 0, 0}, []key.Kind{key.KindUint64}); !errors.Is(err, key.ErrInvalidKey) {
		t.Error("too short")
	}
	if _, err := key.ParseKey([]byte("no_null"), []key.Kind{key.KindString}); !errors.Is(err, key.ErrInvalidKey) {
		t.Error("no null terminator")
	}
	if _, err := key.ParseKey([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0x99}, []key.Kind{key.KindUint64}); !errors.Is(err, key.ErrInvalidKey) {
		t.Error("leftover bytes")
	}
}

func TestParseKeyBytesLastOnly(t *testing.T) {
	raw := key.EncodeBytes([]byte{1, 2, 3}).Bytes()
	k, err := key.ParseKey(raw, []key.Kind{key.KindBytes})
	if err != nil {
		t.Fatal(err)
	}
	b, err := key.DecodeBytes(k)
	if err != nil || !bytes.Equal(b, []byte{1, 2, 3}) {
		t.Error("bytes round-trip")
	}
	// KindBytes in non-last position
	_, err = key.ParseKey(raw, []key.Kind{key.KindBytes, key.KindUint64})
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("got %v", err)
	}
}

func TestParseKeyCopySafety(t *testing.T) {
	raw := key.EncodeUint64(42).Bytes()
	k, _ := key.ParseKey(raw, []key.Kind{key.KindUint64})
	b := k.Bytes()
	b[0] = 0xFF
	b2 := k.Bytes()
	if b2[0] == 0xFF {
		t.Error("Bytes() mutation leaked")
	}
	fs := k.Fields()
	fs[0].Offset = 999
	fs2 := k.Fields()
	if fs2[0].Offset == 999 {
		t.Error("Fields() mutation leaked")
	}
}

func TestParseKeyCompare(t *testing.T) {
	k1 := key.EncodeUint64(1)
	k2, _ := key.ParseKey(key.EncodeUint64(2).Bytes(), []key.Kind{key.KindUint64})
	if k1.Compare(k2) >= 0 {
		t.Error("1 should sort before 2")
	}
}

func TestSortCompositeRoundTrip(t *testing.T) {
	k, err := key.EncodeSortComposite(uint64(1), uint64(2))
	if err != nil {
		t.Fatal(err)
	}
	vals, err := key.DecodeSortComposite(k)
	if err != nil {
		t.Fatal(err)
	}
	if vals[0].(uint64) != 1 || vals[1].(uint64) != 2 {
		t.Error("round trip")
	}

	k2, _ := key.EncodeSortComposite(int64(-1), int64(0))
	vals, _ = key.DecodeSortComposite(k2)
	if vals[0].(int64) != -1 || vals[1].(int64) != 0 {
		t.Error("int64 round trip")
	}
}

func TestSortCompositeOrder(t *testing.T) {
	assertAscending(t, []key.Key{
		must(key.EncodeSortComposite(uint64(0), uint64(0))),
		must(key.EncodeSortComposite(uint64(0), uint64(1))),
		must(key.EncodeSortComposite(uint64(0), uint64(999))),
		must(key.EncodeSortComposite(uint64(1), uint64(0))),
	})
	assertAscending(t, []key.Key{
		must(key.EncodeSortComposite(int64(-1), uint64(0))),
		must(key.EncodeSortComposite(int64(0), uint64(0))),
		must(key.EncodeSortComposite(int64(0), uint64(1))),
	})
}

func TestSortCompositeRejectsStrings(t *testing.T) {
	_, err := key.EncodeSortComposite("hello")
	if !errors.Is(err, key.ErrUnsupportedSortType) {
		t.Errorf("got %v", err)
	}
	_, err = key.EncodeSortComposite([]byte{1})
	if !errors.Is(err, key.ErrUnsupportedSortType) {
		t.Errorf("got %v", err)
	}
	_, err = key.EncodeSortComposite(true)
	if !errors.Is(err, key.ErrUnsupportedType) {
		t.Errorf("got %v", err)
	}
}

func TestDecodeZeroSortComposite(t *testing.T) {
	var k key.Key
	_, err := key.DecodeSortComposite(k)
	if !errors.Is(err, key.ErrNotComposite) {
		t.Errorf("got %v", err)
	}
}

func TestScalarRejectsComposite(t *testing.T) {
	c, _ := key.EncodeSortComposite(uint64(1), uint64(2))
	_, err := key.DecodeUint64(c)
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("got %v", err)
	}
}

func TestParseKeySortComposite(t *testing.T) {
	k, _ := key.EncodeSortComposite(uint64(1), int64(-1))
	raw := k.Bytes()
	k2, _ := key.ParseKey(raw, []key.Kind{key.KindUint64, key.KindInt64})
	vals, _ := key.DecodeSortComposite(k2)
	if vals[0].(uint64) != 1 || vals[1].(int64) != -1 {
		t.Error("parse+decode")
	}
}

func TestStorageCompositeRoundTrip(t *testing.T) {
	k, err := key.EncodeStorageComposite(uint64(1), "hello", []byte{0x01, 0x02})
	if err != nil {
		t.Fatal(err)
	}
	vals, err := key.DecodeStorageComposite(k)
	if err != nil {
		t.Fatal(err)
	}
	if vals[0].(uint64) != 1 || vals[1].(string) != "hello" || !bytes.Equal(vals[2].([]byte), []byte{0x01, 0x02}) {
		t.Error("round trip")
	}
}

func TestStorageCompositeNoNullTerminator(t *testing.T) {
	k, _ := key.EncodeStorageComposite("hello")
	if k.Fields()[0].Length != len("hello") {
		t.Error("storage composite string should not include null terminator")
	}
}

func TestStorageCompositeErrors(t *testing.T) {
	_, err := key.EncodeStorageComposite(true)
	if !errors.Is(err, key.ErrUnsupportedType) {
		t.Errorf("got %v", err)
	}
	_, err = key.EncodeStorageComposite("hel\x00lo")
	if !errors.Is(err, key.ErrNullByteInString) {
		t.Errorf("got %v", err)
	}
}

func TestParseStorageCompositeKey(t *testing.T) {
	k, _ := key.EncodeStorageComposite(uint64(1), "hi", []byte{0xFF})
	raw := k.Bytes()
	k2, err := key.ParseStorageCompositeKey(raw, []key.Kind{key.KindUint64, key.KindString, key.KindBytes})
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := key.DecodeStorageComposite(k2)
	if vals[0].(uint64) != 1 || vals[1].(string) != "hi" || !bytes.Equal(vals[2].([]byte), []byte{0xFF}) {
		t.Error("parse+decode storage composite")
	}
}

func TestSortVsStorageByteDistinct(t *testing.T) {
	sort, _ := key.EncodeSortComposite(uint64(1), uint64(2))
	storage, _ := key.EncodeStorageComposite(uint64(1), uint64(2))
	if bytes.Equal(sort.Bytes(), storage.Bytes()) {
		t.Error("formats must differ")
	}
}

func TestScalarRejectsStorageComposite(t *testing.T) {
	sc, _ := key.EncodeStorageComposite("hello", []byte{0x01})
	_, err := key.DecodeString(sc)
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("DecodeString: %v", err)
	}
	_, err = key.DecodeBytes(sc)
	if !errors.Is(err, key.ErrInvalidKey) {
		t.Errorf("DecodeBytes: %v", err)
	}
}

func TestSortOrderAllTypes(t *testing.T) {
	assertAscending(t, []key.Key{
		key.EncodeUint64(0), key.EncodeUint64(1),
		key.EncodeUint64(255), key.EncodeUint64(256),
		key.EncodeUint64(math.MaxUint64 / 2), key.EncodeUint64(math.MaxUint64),
	})
	assertAscending(t, []key.Key{
		key.EncodeInt64(math.MinInt64), key.EncodeInt64(-256),
		key.EncodeInt64(-1), key.EncodeInt64(0),
		key.EncodeInt64(1), key.EncodeInt64(256),
		key.EncodeInt64(math.MaxInt64),
	})
}

func TestSortCompositeMixedAscending(t *testing.T) {
	assertAscending(t, []key.Key{
		must(key.EncodeSortComposite(uint64(0), uint64(0))),
		must(key.EncodeSortComposite(uint64(0), uint64(1))),
		must(key.EncodeSortComposite(uint64(0), uint64(math.MaxUint64))),
		must(key.EncodeSortComposite(uint64(1), uint64(0))),
	})
}

func TestSortCompositeInt64Ascending(t *testing.T) {
	assertAscending(t, []key.Key{
		must(key.EncodeSortComposite(int64(math.MinInt64), uint64(0))),
		must(key.EncodeSortComposite(int64(-1), uint64(0))),
		must(key.EncodeSortComposite(int64(0), uint64(0))),
		must(key.EncodeSortComposite(int64(1), uint64(0))),
		must(key.EncodeSortComposite(int64(math.MaxInt64), uint64(0))),
	})
}

func TestDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/sql_key.md")
	if err != nil {
		t.Fatalf("docs/sql_key.md not found: %v", err)
	}
	c := string(data)
	for _, s := range []string{
		"# Plomvix SQL Key Encoding", "sql/key", "Key struct", "Field struct",
		"Kind", "uint64", "int64", "sign bit flip", "big-endian", "sort order",
		"null-terminated", "EncodeSortComposite", "EncodeStorageComposite",
		"storage composite is not sort-safe", "length-prefix", "ParseKey",
		"ParseStorageCompositeKey", "Compare", "zero internal imports", "no little-endian",
		"KV store", "WAL", "storage pages", "query execution", "indexes",
		"transaction IDs", "version stamps", "compression", "little-endian",
		"enterprise hardening", "validateField", "fuzz testing", "never panic",
		"benchmarks", "no API changes", "no wire format changes",
	} {
		if !bytes.Contains([]byte(c), []byte(s)) {
			t.Errorf("missing: %q", s)
		}
	}
}

func assertAscending(t *testing.T, keys []key.Key) {
	t.Helper()
	for i := 0; i < len(keys)-1; i++ {
		if keys[i].Compare(keys[i+1]) > 0 {
			t.Errorf("keys[%d] should sort before keys[%d]", i, i+1)
		}
	}
}
func must(k key.Key, err error) key.Key {
	if err != nil {
		panic(err)
	}
	return k
}
