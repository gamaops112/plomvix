package key

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/rand"
	"testing"
)

func TestTagConstants(t *testing.T) {
	if TagTableData != 0x01 {
		t.Errorf("TagTableData = %#x, want 0x01", TagTableData)
	}
	if TagMetadata != 0x02 {
		t.Errorf("TagMetadata = %#x, want 0x02", TagMetadata)
	}
	if TagIndex != 0x03 {
		t.Errorf("TagIndex = %#x, want 0x03", TagIndex)
	}
}

func TestKindConstants(t *testing.T) {
	if KindNull != 0 {
		t.Errorf("KindNull = %d, want 0", KindNull)
	}
	if KindBool != 1 {
		t.Errorf("KindBool = %d, want 1", KindBool)
	}
	if KindInt64 != 2 {
		t.Errorf("KindInt64 = %d, want 2", KindInt64)
	}
	if KindUint64 != 3 {
		t.Errorf("KindUint64 = %d, want 3", KindUint64)
	}
	if KindString != 4 {
		t.Errorf("KindString = %d, want 4", KindString)
	}
	if KindBytes != 5 {
		t.Errorf("KindBytes = %d, want 5", KindBytes)
	}
}

func TestValueConstructorsAndAccessors(t *testing.T) {
	t.Run("Null", func(t *testing.T) {
		if Null().Kind() != KindNull {
			t.Error("kind mismatch")
		}
	})
	t.Run("Bool", func(t *testing.T) {
		b, ok := Bool(true).AsBool()
		if !ok || !b {
			t.Errorf("AsBool = (%v,%v)", b, ok)
		}
		b, ok = Bool(false).AsBool()
		if !ok || b {
			t.Errorf("AsBool = (%v,%v)", b, ok)
		}
	})
	t.Run("Int64", func(t *testing.T) {
		n, ok := Int64(-42).AsInt64()
		if !ok || n != -42 {
			t.Errorf("AsInt64 = (%d,%v)", n, ok)
		}
	})
	t.Run("Uint64", func(t *testing.T) {
		n, ok := Uint64(42).AsUint64()
		if !ok || n != 42 {
			t.Errorf("AsUint64 = (%d,%v)", n, ok)
		}
	})
	t.Run("String", func(t *testing.T) {
		s, ok := String("hello").AsString()
		if !ok || s != "hello" {
			t.Errorf("AsString = (%q,%v)", s, ok)
		}
	})
	t.Run("Bytes", func(t *testing.T) {
		b, ok := Bytes([]byte{1, 2, 3}).AsBytes()
		if !ok || string(b) != "\x01\x02\x03" {
			t.Errorf("AsBytes = (%v,%v)", b, ok)
		}
	})
	t.Run("wrong kind returns false", func(t *testing.T) {
		if _, ok := Int64(1).AsBool(); ok {
			t.Error("Int64.AsBool should return false")
		}
		if _, ok := String("x").AsInt64(); ok {
			t.Error("String.AsInt64 should return false")
		}
	})
}

func TestValueEqual(t *testing.T) {
	if !Int64(5).Equal(Int64(5)) {
		t.Error("Int64(5) == Int64(5)")
	}
	if Int64(5).Equal(Int64(6)) {
		t.Error("Int64(5) != Int64(6)")
	}
	if Int64(0).Equal(Uint64(0)) {
		t.Error("different kinds")
	}
	if !Null().Equal(Null()) {
		t.Error("Null == Null")
	}
	if !String("x").Equal(String("x")) {
		t.Error("String(x) == String(x)")
	}
}

func TestValueBytesMutationSafety(t *testing.T) {
	orig := []byte{1, 2, 3}
	v := Bytes(orig)
	orig[0] = 99
	b, _ := v.AsBytes()
	if b[0] != 1 {
		t.Error("Bytes constructor must copy input")
	}
	b2, _ := v.AsBytes()
	b2[0] = 99
	b3, _ := v.AsBytes()
	if b3[0] != 1 {
		t.Error("AsBytes must return a copy")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  Value
		kind Kind
	}{
		{"null", Null(), KindNull},
		{"bool true", Bool(true), KindBool},
		{"bool false", Bool(false), KindBool},
		{"int64 zero", Int64(0), KindInt64},
		{"int64 pos", Int64(42), KindInt64},
		{"int64 neg", Int64(-42), KindInt64},
		{"int64 min", Int64(-9223372036854775808), KindInt64},
		{"int64 max", Int64(9223372036854775807), KindInt64},
		{"uint64 zero", Uint64(0), KindUint64},
		{"uint64", Uint64(42), KindUint64},
		{"uint64 max", Uint64(18446744073709551615), KindUint64},
		{"string empty", String(""), KindString},
		{"string simple", String("hello"), KindString},
		{"string with zero", String("ab\x00cd"), KindString},
		{"bytes empty", Bytes([]byte{}), KindBytes},
		{"bytes simple", Bytes([]byte{1, 2, 3}), KindBytes},
		{"bytes with zero", Bytes([]byte{0}), KindBytes},
		{"bytes with zeros", Bytes([]byte{0, 1, 0}), KindBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := encodeValue(tt.val)
			if len(enc) == 0 {
				t.Fatal("encoded output is empty")
			}
			dec, consumed, err := decodeValue(enc, tt.kind)
			if err != nil {
				t.Fatalf("decodeValue: %v", err)
			}
			if !dec.Equal(tt.val) {
				t.Errorf("round-trip mismatch: got %#v, want %#v", dec, tt.val)
			}
			if consumed != len(enc) {
				t.Errorf("consumed %d, want %d", consumed, len(enc))
			}
		})
	}
}

func assertOrder(t *testing.T, smaller, larger Value) {
	t.Helper()
	encSmall := encodeValue(smaller)
	encLarge := encodeValue(larger)
	if bytes.Compare(encSmall, encLarge) >= 0 {
		t.Errorf("%v should sort before %v", smaller, larger)
	}
}

func TestInt64EncodingOrder(t *testing.T) {
	vals := []int64{-9223372036854775808, -5, -1, 0, 1, 5, 9223372036854775807}
	for i := 0; i < len(vals)-1; i++ {
		assertOrder(t, Int64(vals[i]), Int64(vals[i+1]))
	}
}

func TestUint64EncodingOrder(t *testing.T) {
	assertOrder(t, Uint64(0), Uint64(1))
	assertOrder(t, Uint64(1), Uint64(256))
	assertOrder(t, Uint64(256), Uint64(18446744073709551615))
}

func TestStringEncodingOrder(t *testing.T) {
	assertOrder(t, String(""), String("a"))
	assertOrder(t, String("a"), String("ab"))
	assertOrder(t, String("ab"), String("b"))
}

func TestBytesEncodingOrder(t *testing.T) {
	assertOrder(t, Bytes([]byte{}), Bytes([]byte{0}))
	assertOrder(t, Bytes([]byte{0}), Bytes([]byte{0, 0}))
	assertOrder(t, Bytes([]byte{0, 0}), Bytes([]byte{0, 1}))
	assertOrder(t, Bytes([]byte{0, 1}), Bytes([]byte{1}))
}

func TestBoolEncodingOrder(t *testing.T) {
	assertOrder(t, Bool(false), Bool(true))
}

func TestEncodeTableRowKeyWorkedExample(t *testing.T) {
	// table 7, PK = ("ab", int64 5), version 1
	key, err := EncodeTableRowKey(7, []Value{String("ab"), Int64(5)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// keyspace tag
	if key[0] != 0x01 {
		t.Errorf("tag = %x", key[0])
	}
	// tableID big-endian
	expectedTableID := []byte{0, 0, 0, 0, 0, 0, 0, 7}
	if !bytes.Equal(key[1:9], expectedTableID) {
		t.Errorf("tableID = %x", key[1:9])
	}
	// verify starts with TablePrefix
	prefix := TablePrefix(7)
	if !bytes.Equal(key[:len(prefix)], prefix) {
		t.Errorf("prefix mismatch")
	}
}

func TestEncodeTableRowKeyEmptyPK(t *testing.T) {
	_, err := EncodeTableRowKey(1, nil, 1)
	if err != ErrNoPKColumns {
		t.Errorf("error = %v, want ErrNoPKColumns", err)
	}
}

func TestTablePrefix(t *testing.T) {
	p := TablePrefix(42)
	if p[0] != 0x01 {
		t.Errorf("tag = %x", p[0])
	}
	if binary.BigEndian.Uint64(p[1:]) != 42 {
		t.Error("tableID mismatch")
	}
}

func TestDecodeTableRowKeyFullRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		pk   []Value
		kind []Kind
		ver  uint64
	}{
		{"single null", 1, []Value{Null()}, []Kind{KindNull}, 42},
		{"single bool", 3, []Value{Bool(true)}, []Kind{KindBool}, 0},
		{"single int64", 5, []Value{Int64(-100)}, []Kind{KindInt64}, 1},
		{"single uint64", 7, []Value{Uint64(999)}, []Kind{KindUint64}, 100},
		{"single string", 9, []Value{String("hello")}, []Kind{KindString}, 1 << 63},
		{"single bytes", 11, []Value{Bytes([]byte{0, 1, 0})}, []Kind{KindBytes}, 0},
		{"composite 2", 13, []Value{Int64(1), String("x")}, []Kind{KindInt64, KindString}, 5},
		{"composite 3", 15, []Value{Int64(1), String("ab"), Bytes([]byte{0})}, []Kind{KindInt64, KindString, KindBytes}, 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := EncodeTableRowKey(tt.id, tt.pk, tt.ver)
			if err != nil {
				t.Fatal(err)
			}
			id, pk, ver, err := DecodeTableRowKey(key, tt.kind)
			if err != nil {
				t.Fatalf("DecodeTableRowKey: %v", err)
			}
			if id != tt.id {
				t.Errorf("tableID = %d, want %d", id, tt.id)
			}
			if ver != tt.ver {
				t.Errorf("version = %d, want %d", ver, tt.ver)
			}
			if len(pk) != len(tt.pk) {
				t.Fatalf("len(pk) = %d, want %d", len(pk), len(tt.pk))
			}
			for i := range pk {
				if !pk[i].Equal(tt.pk[i]) {
					t.Errorf("pk[%d] = %#v, want %#v", i, pk[i], tt.pk[i])
				}
			}
		})
	}
}

func TestFullKeyOrdering(t *testing.T) {
	t.Run("same table+pk newer version first", func(t *testing.T) {
		k1, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 10)
		k2, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 1)
		if bytes.Compare(k1, k2) >= 0 {
			t.Error("newer version (10) should sort before older (1)")
		}
	})

	t.Run("same table order by pk ascending", func(t *testing.T) {
		k1, _ := EncodeTableRowKey(1, []Value{Int64(1)}, 100) // older version
		k2, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 1)   // newer version
		if bytes.Compare(k1, k2) >= 0 {
			t.Error("pk=1 should sort before pk=5 regardless of version")
		}
	})

	t.Run("order by tableID ascending", func(t *testing.T) {
		k1, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 100)
		k2, _ := EncodeTableRowKey(2, []Value{Int64(1)}, 1)
		if bytes.Compare(k1, k2) >= 0 {
			t.Error("tableID 1 should sort before tableID 2")
		}
	})

	t.Run("composite pk lexicographic", func(t *testing.T) {
		k1, _ := EncodeTableRowKey(1, []Value{Int64(1), String("a")}, 0)
		k2, _ := EncodeTableRowKey(1, []Value{Int64(1), String("b")}, 0)
		if bytes.Compare(k1, k2) >= 0 {
			t.Error("(1,a) should sort before (1,b)")
		}
		k3, _ := EncodeTableRowKey(1, []Value{Int64(2), String("a")}, 0)
		if bytes.Compare(k1, k3) >= 0 {
			t.Error("(1,a) should sort before (2,a)")
		}
	})
}

func TestErrorPaths(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		_, _, _, err := DecodeTableRowKey(nil, []Kind{KindInt64})
		if !errors.Is(err, ErrEmptyKey) {
			t.Errorf("want ErrEmptyKey, got %v", err)
		}
	})
	t.Run("bad keyspace tag", func(t *testing.T) {
		_, _, _, err := DecodeTableRowKey([]byte{0xFF, 0, 0, 0, 0, 0, 0, 0, 5, 0x30, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}, []Kind{KindInt64})
		if !errors.Is(err, ErrBadTag) {
			t.Errorf("want ErrBadTag, got %v", err)
		}
	})
	t.Run("empty expectedKinds", func(t *testing.T) {
		_, _, _, err := DecodeTableRowKey([]byte{0x01}, nil)
		if !errors.Is(err, ErrNoPKColumns) {
			t.Errorf("want ErrNoPKColumns, got %v", err)
		}
	})
	t.Run("truncated tableID", func(t *testing.T) {
		_, _, _, err := DecodeTableRowKey([]byte{0x01, 0, 0, 0}, []Kind{KindInt64})
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("want ErrTruncated, got %v", err)
		}
	})
	t.Run("kind mismatch", func(t *testing.T) {
		k, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
		_, _, _, err := DecodeTableRowKey(k, []Kind{KindString})
		if !errors.Is(err, ErrKindMismatch) {
			t.Errorf("want ErrKindMismatch, got %v", err)
		}
	})
	t.Run("unknown type tag", func(t *testing.T) {
		b := []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x99, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
		_, _, _, err := DecodeTableRowKey(b, []Kind{KindInt64})
		if !errors.Is(err, ErrBadTypeTag) {
			t.Errorf("want ErrBadTypeTag, got %v", err)
		}
	})
	t.Run("truncated variable length", func(t *testing.T) {
		k, _ := EncodeTableRowKey(1, []Value{String("hello")}, 0)
		trunc := k[:len(k)-5]
		_, _, _, err := DecodeTableRowKey(trunc, []Kind{KindString})
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("want ErrTruncated, got %v", err)
		}
	})
	t.Run("bad field", func(t *testing.T) {
		b := []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x50, 0x00, 0x02, 0, 0, 0, 0, 0, 0, 0, 0}
		_, _, _, err := DecodeTableRowKey(b, []Kind{KindString})
		if !errors.Is(err, ErrBadField) {
			t.Errorf("want ErrBadField, got %v", err)
		}
	})
	t.Run("truncated version", func(t *testing.T) {
		k, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
		trunc := k[:len(k)-5]
		_, _, _, err := DecodeTableRowKey(trunc, []Kind{KindInt64})
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("want ErrTruncated, got %v", err)
		}
	})
	t.Run("trailing bytes", func(t *testing.T) {
		k, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
		k = append(k, 0x99)
		_, _, _, err := DecodeTableRowKey(k, []Kind{KindInt64})
		if !errors.Is(err, ErrTrailingBytes) {
			t.Errorf("want ErrTrailingBytes, got %v", err)
		}
	})
}

func TestRandomizedRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	kinds := []Kind{KindNull, KindBool, KindInt64, KindUint64, KindString, KindBytes}
	for i := 0; i < 200; i++ {
		tableID := rng.Uint64()
		arity := rng.Intn(6) + 1
		pk := make([]Value, arity)
		expectedKinds := make([]Kind, arity)
		for j := 0; j < arity; j++ {
			k := kinds[rng.Intn(len(kinds))]
			expectedKinds[j] = k
			switch k {
			case KindNull:
				pk[j] = Null()
			case KindBool:
				pk[j] = Bool(rng.Intn(2) == 1)
			case KindInt64:
				pk[j] = Int64(rng.Int63())
			case KindUint64:
				pk[j] = Uint64(rng.Uint64())
			case KindString:
				n := rng.Intn(16)
				b := make([]byte, n)
				for x := 0; x < n; x++ {
					b[x] = byte(rng.Intn(256))
				}
				pk[j] = String(string(b))
			case KindBytes:
				n := rng.Intn(16)
				b := make([]byte, n)
				for x := 0; x < n; x++ {
					b[x] = byte(rng.Intn(256))
				}
				pk[j] = Bytes(b)
			}
		}
		version := rng.Uint64()
		key, err := EncodeTableRowKey(tableID, pk, version)
		if err != nil {
			t.Fatalf("EncodeTableRowKey: %v", err)
		}
		id, decPK, ver, err := DecodeTableRowKey(key, expectedKinds)
		if err != nil {
			t.Fatalf("DecodeTableRowKey: %v", err)
		}
		if id != tableID {
			t.Errorf("tableID mismatch")
		}
		if ver != version {
			t.Errorf("version mismatch")
		}
		for j := range pk {
			if !decPK[j].Equal(pk[j]) {
				t.Errorf("pk[%d] mismatch", j)
			}
		}
	}
}

func TestIsCanonicalValidKeys(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		pk   []Value
		kind []Kind
		ver  uint64
	}{
		{"null", 1, []Value{Null()}, []Kind{KindNull}, 0},
		{"bool", 2, []Value{Bool(true)}, []Kind{KindBool}, 1},
		{"int64", 3, []Value{Int64(-42)}, []Kind{KindInt64}, 5},
		{"uint64", 4, []Value{Uint64(999)}, []Kind{KindUint64}, 10},
		{"string", 5, []Value{String("hello")}, []Kind{KindString}, 100},
		{"bytes", 6, []Value{Bytes([]byte{0, 1, 0})}, []Kind{KindBytes}, 0},
		{"composite", 7, []Value{Int64(1), String("x")}, []Kind{KindInt64, KindString}, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := EncodeTableRowKey(tt.id, tt.pk, tt.ver)
			if err != nil {
				t.Fatal(err)
			}
			ok, err := IsCanonical(key, tt.kind)
			if !ok || err != nil {
				t.Errorf("IsCanonical = (%v, %v), want (true, nil)", ok, err)
			}
		})
	}
}

func TestIsCanonicalMalformed(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		ok, err := IsCanonical([]byte{0x01, 0, 0}, []Kind{KindInt64})
		if ok {
			t.Error("expected false for truncated")
		}
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("want ErrTruncated, got %v", err)
		}
	})
	t.Run("bad tag", func(t *testing.T) {
		ok, err := IsCanonical([]byte{0xFF, 0, 0, 0, 0, 0, 0, 0, 1,
			0x30, 0, 0, 0, 0, 0, 0, 0, 1,
			0, 0, 0, 0, 0, 0, 0, 0}, []Kind{KindInt64})
		if ok {
			t.Error("expected false")
		}
		if !errors.Is(err, ErrBadTag) {
			t.Errorf("want ErrBadTag, got %v", err)
		}
	})
	t.Run("bad field", func(t *testing.T) {
		b := []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x50, 0x00, 0x02, 0, 0, 0, 0, 0, 0, 0, 0}
		ok, err := IsCanonical(b, []Kind{KindString})
		if ok {
			t.Error("expected false")
		}
		if !errors.Is(err, ErrBadField) {
			t.Errorf("want ErrBadField, got %v", err)
		}
	})
}

func TestExhaustiveMalformedKeys(t *testing.T) {
	// validEncode produces a valid int64-key for tableID 1, version 0.
	mustKey := func() []byte {
		k, err := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}

	type tc struct {
		name    string
		input   []byte
		kinds   []Kind
		wantErr error
	}
	tests := []tc{
		{name: "empty input", input: nil, kinds: []Kind{KindInt64}, wantErr: ErrEmptyKey},
		{name: "empty input bytes", input: []byte{}, kinds: []Kind{KindInt64}, wantErr: ErrEmptyKey},
		{name: "wrong tag 0x00", input: append([]byte{0x00}, mustKey()[1:]...), kinds: []Kind{KindInt64}, wantErr: ErrBadTag},
		{name: "wrong tag 0x02", input: append([]byte{0x02}, mustKey()[1:]...), kinds: []Kind{KindInt64}, wantErr: ErrBadTag},
		{name: "wrong tag 0x03", input: append([]byte{0x03}, mustKey()[1:]...), kinds: []Kind{KindInt64}, wantErr: ErrBadTag},
		{name: "wrong tag 0xFF", input: append([]byte{0xFF}, mustKey()[1:]...), kinds: []Kind{KindInt64}, wantErr: ErrBadTag},
		{name: "expectedKinds nil", input: mustKey(), kinds: nil, wantErr: ErrNoPKColumns},
		{name: "expectedKinds empty", input: mustKey(), kinds: []Kind{}, wantErr: ErrNoPKColumns},
		{name: "tableID truncated len 0", input: []byte{0x01}, kinds: []Kind{KindInt64}, wantErr: ErrTruncated},
		{name: "tableID truncated len 2", input: []byte{0x01, 0, 0}, kinds: []Kind{KindInt64}, wantErr: ErrTruncated},
		{name: "tableID truncated len 7", input: []byte{0x01, 0, 0, 0, 0, 0, 0, 0}, kinds: []Kind{KindInt64}, wantErr: ErrTruncated},
		{name: "kind mismatch string for int64", input: mustKey(), kinds: []Kind{KindString}, wantErr: ErrKindMismatch},
		{name: "kind mismatch bool for int64", input: mustKey(), kinds: []Kind{KindBool}, wantErr: ErrKindMismatch},
		{name: "kind mismatch null for int64", input: mustKey(), kinds: []Kind{KindNull}, wantErr: ErrKindMismatch},
		{name: "unknown type tag", input: []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x99, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}, kinds: []Kind{KindInt64}, wantErr: ErrBadTypeTag},
		{name: "varfield no terminator", input: func() []byte {
			k, _ := EncodeTableRowKey(1, []Value{String("hello")}, 0)
			return k[:len(k)-5]
		}(), kinds: []Kind{KindString}, wantErr: ErrTruncated},
		{name: "varfield trailing lone 0x00", input: []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x50, 0x61, 0x00}, kinds: []Kind{KindString}, wantErr: ErrTruncated},
		{name: "varfield bad escape 0x00 0x02", input: []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 1, 0x50, 0x00, 0x02, 0, 0, 0, 0, 0, 0, 0, 0}, kinds: []Kind{KindString}, wantErr: ErrBadField},
		{name: "version truncated len 0", input: mustKey()[:len(mustKey())-8], kinds: []Kind{KindInt64}, wantErr: ErrTruncated},
		{name: "version truncated len 4", input: mustKey()[:len(mustKey())-4], kinds: []Kind{KindInt64}, wantErr: ErrTruncated},
		{name: "trailing 1 byte", input: append(mustKey(), 0x99), kinds: []Kind{KindInt64}, wantErr: ErrTrailingBytes},
		{name: "trailing 8 bytes", input: append(mustKey(), 0, 0, 0, 0, 0, 0, 0, 0), kinds: []Kind{KindInt64}, wantErr: ErrTrailingBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := DecodeTableRowKey(tt.input, tt.kinds)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want error matching %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrefixEnd(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if PrefixEnd(nil) != nil {
			t.Error("nil input should return nil")
		}
		if PrefixEnd([]byte{}) != nil {
			t.Error("empty input should return nil")
		}
	})
	t.Run("all 0xFF returns nil", func(t *testing.T) {
		if PrefixEnd([]byte{0xFF, 0xFF}) != nil {
			t.Error("all 0xFF should return nil")
		}
	})
	t.Run("simple increment", func(t *testing.T) {
		got := PrefixEnd([]byte{0x01, 0x00, 0x02})
		want := []byte{0x01, 0x00, 0x03}
		if !bytes.Equal(got, want) {
			t.Errorf("got %x, want %x", got, want)
		}
	})
	t.Run("carry", func(t *testing.T) {
		got := PrefixEnd([]byte{0x01, 0xFF})
		want := []byte{0x02}
		if !bytes.Equal(got, want) {
			t.Errorf("got %x, want %x", got, want)
		}
	})
	t.Run("prefix start < end", func(t *testing.T) {
		p := TablePrefix(5)
		end := PrefixEnd(p)
		if bytes.Compare(p, end) >= 0 {
			t.Error("prefix should be < PrefixEnd")
		}
	})
}

func TestTableRange(t *testing.T) {
	start, end := TableRange(3)
	if !bytes.Equal(start, TablePrefix(3)) {
		t.Error("start != TablePrefix")
	}
	if end == nil {
		t.Fatal("end is nil")
	}
	// Key for table 3 should be between start and end
	k, _ := EncodeTableRowKey(3, []Value{Int64(5)}, 0)
	if bytes.Compare(k, start) < 0 {
		t.Error("key < start")
	}
	if bytes.Compare(k, end) >= 0 {
		t.Error("key >= end")
	}
	// Key for table 4 should NOT be between
	k4, _ := EncodeTableRowKey(4, []Value{Int64(1)}, 0)
	if bytes.Compare(k4, start) >= 0 && bytes.Compare(k4, end) < 0 {
		t.Error("table 4 key should not be in table 3 range")
	}
	// Table 4 start should be >= table 3 end
	start4, _ := TableRange(4)
	if bytes.Compare(start4, end) < 0 {
		t.Error("table 4 start should be >= table 3 end")
	}
}

func TestGoldenVectors(t *testing.T) {
	type golden struct {
		name  string
		id    uint64
		pk    []Value
		ver   uint64
		kinds []Kind
		hex   string
	}
	goldens := []golden{
		{name: "single_int64", id: 1, pk: []Value{Int64(5)}, ver: 0, kinds: []Kind{KindInt64},
			hex: "010000000000000001308000000000000005ffffffffffffffff"},
		{name: "single_string", id: 2, pk: []Value{String("ab")}, ver: 1, kinds: []Kind{KindString},
			hex: "0100000000000000025061620001fffffffffffffffe"},
		{name: "composite", id: 7, pk: []Value{String("ab"), Int64(5)}, ver: 1, kinds: []Kind{KindString, KindInt64},
			hex: "0100000000000000075061620001308000000000000005fffffffffffffffe"},
		{name: "bytes_with_zero", id: 11, pk: []Value{Bytes([]byte{0, 1, 0})}, ver: 0, kinds: []Kind{KindBytes},
			hex: "01000000000000000b6000ff0100ff0001ffffffffffffffff"},
		{name: "uint64", id: 4, pk: []Value{Uint64(999)}, ver: 10, kinds: []Kind{KindUint64},
			hex: "0100000000000000044000000000000003e7fffffffffffffff5"},
		{name: "bool", id: 3, pk: []Value{Bool(true)}, ver: 0, kinds: []Kind{KindBool},
			hex: "0100000000000000032001ffffffffffffffff"},
		{name: "null", id: 5, pk: []Value{Null()}, ver: 0, kinds: []Kind{KindNull},
			hex: "01000000000000000510ffffffffffffffff"},
		{name: "int64_min", id: 6, pk: []Value{Int64(-9223372036854775808)}, ver: 0, kinds: []Kind{KindInt64},
			hex: "010000000000000006300000000000000000ffffffffffffffff"},
		{name: "int64_max", id: 7, pk: []Value{Int64(9223372036854775807)}, ver: 0, kinds: []Kind{KindInt64},
			hex: "01000000000000000730ffffffffffffffffffffffffffffffff"},
		{name: "version_max", id: 1, pk: []Value{Int64(0)}, ver: 0xFFFFFFFFFFFFFFFF, kinds: []Kind{KindInt64},
			hex: "0100000000000000013080000000000000000000000000000000"},
	}

	for _, g := range goldens {
		t.Run(g.name, func(t *testing.T) {
			want, err := hex.DecodeString(g.hex)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EncodeTableRowKey(g.id, g.pk, g.ver)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("encode mismatch\n got: %x\nwant: %x", got, want)
			}
			// Decode round-trip the hand-written bytes
			id, pk, ver, err := DecodeTableRowKey(want, g.kinds)
			if err != nil {
				t.Fatalf("decode golden: %v", err)
			}
			if id != g.id {
				t.Errorf("id = %d, want %d", id, g.id)
			}
			if ver != g.ver {
				t.Errorf("ver = %d, want %d", ver, g.ver)
			}
			if len(pk) != len(g.pk) {
				t.Fatalf("pk len mismatch")
			}
			for i := range pk {
				if !pk[i].Equal(g.pk[i]) {
					t.Errorf("pk[%d] mismatch", i)
				}
			}
		})
	}
}

func TestPropertyOrdering(t *testing.T) {
	rng := rand.New(rand.NewSource(99))

	// Ordering matches logical order for single-column values.
	t.Run("int64 matches numeric order", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			a := int64(rng.Uint64()>>1) * (1 - 2*int64(rng.Intn(2)))
			b := int64(rng.Uint64()>>1) * (1 - 2*int64(rng.Intn(2)))
			ea := encodeValue(Int64(a))
			eb := encodeValue(Int64(b))
			cmpEnc := bytes.Compare(ea, eb)
			var cmpLog int
			if a < b {
				cmpLog = -1
			} else if a > b {
				cmpLog = 1
			}
			if (cmpEnc < 0) != (cmpLog < 0) || (cmpEnc == 0) != (cmpLog == 0) {
				t.Errorf("int64 a=%d b=%d: byte cmp=%d logical cmp=%d", a, b, cmpEnc, cmpLog)
			}
		}
	})

	t.Run("uint64 matches numeric order", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			a := rng.Uint64()
			b := rng.Uint64()
			ea := encodeValue(Uint64(a))
			eb := encodeValue(Uint64(b))
			cmp := bytes.Compare(ea, eb)
			exp := cmpInt(a, b)
			if (cmp < 0) != (exp < 0) || (cmp == 0) != (exp == 0) {
				t.Errorf("uint64 a=%d b=%d: cmp=%d exp=%d", a, b, cmp, exp)
			}
		}
	})

	t.Run("string matches lex order", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			la := rng.Intn(8)
			sa := make([]byte, la)
			for x := 0; x < la; x++ {
				sa[x] = byte(rng.Intn(256))
			}
			lb := rng.Intn(8)
			sb := make([]byte, lb)
			for x := 0; x < lb; x++ {
				sb[x] = byte(rng.Intn(256))
			}
			ea := encodeValue(String(string(sa)))
			eb := encodeValue(String(string(sb)))
			cmp := bytes.Compare(ea, eb)
			exp := bytes.Compare(sa, sb)
			if (cmp < 0) != (exp < 0) || (cmp == 0) != (exp == 0) {
				t.Errorf("cmp=%d exp=%d", cmp, exp)
			}
		}
	})

	// Composite monotonicity: changing later column never inverts earlier order.
	t.Run("composite monotonicity", func(t *testing.T) {
		// a=(1,x), b=(2,*) always: b sorts after a regardless of second column
		ka, _ := EncodeTableRowKey(1, []Value{Int64(1), Int64(999)}, 0)
		kb, _ := EncodeTableRowKey(1, []Value{Int64(2), Int64(-999)}, 0)
		if bytes.Compare(ka, kb) >= 0 {
			t.Error("(1,999) should sort before (2,-999)")
		}
	})

	// Version inversion
	t.Run("newer version sorts earlier", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			vOld := rng.Uint64() >> 1
			vNew := vOld + 1 + uint64(rng.Int63())
			ko, _ := EncodeTableRowKey(1, []Value{Int64(5)}, vOld)
			kn, _ := EncodeTableRowKey(1, []Value{Int64(5)}, vNew)
			if bytes.Compare(kn, ko) >= 0 {
				t.Error("newer version should sort before older")
			}
		}
	})
}

func cmpInt(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func TestEncodeOutputImmutability(t *testing.T) {
	// Mutating one encode output must not affect another.
	k1, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
	k2, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
	k1[0] = 0xFF
	if bytes.Equal(k1, k2) {
		t.Error("mutating returned slice should not affect other encodes")
	}
}

func TestTablePrefixFreshSlice(t *testing.T) {
	p1 := TablePrefix(1)
	p2 := TablePrefix(1)
	p1[1] = 0xFF
	if bytes.Equal(p1, p2) {
		t.Error("mutating one TablePrefix should not affect another")
	}
}

func TestTableRangeFreshSlice(t *testing.T) {
	s1, e1 := TableRange(1)
	s1[1] = 0xFF
	e1[1] = 0xEE
	s2, _ := TableRange(1)
	if bytes.Equal(s1, s2) {
		t.Error("mutating TableRange start should not affect later calls")
	}
}

func BenchmarkEncodeInt64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeTableRowKey(1, []Value{Int64(5)}, 0)
	}
}

func BenchmarkEncodeString(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeTableRowKey(2, []Value{String("hello")}, 0)
	}
}

func BenchmarkEncodeComposite(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeTableRowKey(7, []Value{String("ab"), Int64(5)}, 1)
	}
}

func BenchmarkDecodeInt64(b *testing.B) {
	key, _ := EncodeTableRowKey(1, []Value{Int64(5)}, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = DecodeTableRowKey(key, []Kind{KindInt64})
	}
}

func BenchmarkDecodeString(b *testing.B) {
	key, _ := EncodeTableRowKey(2, []Value{String("hello")}, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = DecodeTableRowKey(key, []Kind{KindString})
	}
}

func BenchmarkDecodeComposite(b *testing.B) {
	key, _ := EncodeTableRowKey(7, []Value{String("ab"), Int64(5)}, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = DecodeTableRowKey(key, []Kind{KindString, KindInt64})
	}
}
