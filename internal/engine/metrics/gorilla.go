// Package metrics provides a time-series metrics engine for Plomvix.
// gorilla.go implements Gorilla-style double-delta timestamp compression
// and XOR float bit-packing for compact metric point storage.
package metrics

import (
	"encoding/binary"
	"math"
)

// gorillaEncoder writes Gorilla-compressed timestamps and values to a
// byte buffer using bit-level packing.
type gorillaEncoder struct {
	buf          []byte
	bitPos       int // next bit position to write (0 = MSB of byte 0)
	prevTime     int64
	prevDelta    int64
	prevValue    float64
	prevLeading  int
	prevTrailing int
	hasPrevTime  bool
	hasPrevValue bool
}

// newGorillaEncoder creates a new encoder with initial capacity.
func newGorillaEncoder(cap int) *gorillaEncoder {
	return &gorillaEncoder{
		buf: make([]byte, cap),
	}
}

// bytes returns the current encoded bytes up to the written bit position.
func (e *gorillaEncoder) bytes() []byte {
	byteLen := (e.bitPos + 7) / 8
	return e.buf[:byteLen]
}

// ensureCapacity grows the buffer if needed for n additional bits.
func (e *gorillaEncoder) ensureCapacity(bits int) {
	needed := (e.bitPos + bits + 7) / 8
	if needed <= len(e.buf) {
		return
	}
	newBuf := make([]byte, needed*2)
	copy(newBuf, e.buf)
	e.buf = newBuf
}

// writeBit writes a single bit (0 or 1).
func (e *gorillaEncoder) writeBit(bit byte) {
	e.ensureCapacity(1)
	byteIdx := e.bitPos / 8
	bitIdx := 7 - (e.bitPos % 8)
	if bit != 0 {
		e.buf[byteIdx] |= 1 << bitIdx
	}
	e.bitPos++
}

// writeBits writes nBits from value (LSB-aligned) to the stream.
func (e *gorillaEncoder) writeBits(value uint64, nBits int) {
	e.ensureCapacity(nBits)
	for i := nBits - 1; i >= 0; i-- {
		e.writeBit(byte((value >> i) & 1))
	}
}

// writeTimestamp encodes a timestamp using double-delta compression.
// First timestamp is stored raw (64 bits). Subsequent timestamps use
// delta-of-delta encoding.
func (e *gorillaEncoder) writeTimestamp(ts int64) {
	if !e.hasPrevTime {
		e.writeBits(uint64(ts), 64)
		e.prevTime = ts
		e.prevDelta = 0
		e.hasPrevTime = true
		return
	}

	delta := ts - e.prevTime
	dod := delta - e.prevDelta

	switch {
	case dod == 0:
		e.writeBit(0)
	case dod >= -63 && dod <= 64:
		e.writeBits(0x02, 2) // '10'
		e.writeBits(uint64(dod)&0x7F, 7)
	case dod >= -255 && dod <= 256:
		e.writeBits(0x06, 3) // '110'
		e.writeBits(uint64(dod)&0x1FF, 9)
	case dod >= -2047 && dod <= 2048:
		e.writeBits(0x0E, 4) // '1110'
		e.writeBits(uint64(dod)&0xFFF, 12)
	default:
		e.writeBits(0x0F, 4) // '1111'
		e.writeBits(uint64(dod), 32)
	}

	e.prevTime = ts
	e.prevDelta = delta
}

// writeFloat encodes a float64 value using XOR compression.
// First value is stored raw (64 bits). Subsequent values use XOR
// with leading/trailing zero optimization.
func (e *gorillaEncoder) writeFloat(val float64) {
	bits := math.Float64bits(val)

	if !e.hasPrevValue {
		e.writeBits(bits, 64)
		e.prevValue = val
		e.hasPrevValue = true
		return
	}

	prevBits := math.Float64bits(e.prevValue)
	xor := bits ^ prevBits

	if xor == 0 {
		e.writeBit(0)
	} else {
		e.writeBit(1)

		leading := leadingZeros64(xor)
		trailing := trailingZeros64(xor)

		if leading >= e.prevLeading && trailing >= e.prevTrailing {
			e.writeBit(0)
			meaningfulLen := 64 - e.prevLeading - e.prevTrailing
			meaningful := (xor >> e.prevTrailing) & ((1 << meaningfulLen) - 1)
			e.writeBits(meaningful, meaningfulLen)
		} else {
			e.writeBit(1)
			e.writeBits(uint64(leading), 5)
			meaningfulLen := 64 - leading - trailing
			e.writeBits(uint64(meaningfulLen), 6)
			meaningful := (xor >> trailing) & ((1 << meaningfulLen) - 1)
			e.writeBits(meaningful, meaningfulLen)
			e.prevLeading = leading
			e.prevTrailing = trailing
		}
	}

	e.prevValue = val
}

// --- Gorilla Decoder ---

// gorillaDecoder reads Gorilla-compressed timestamps and values from a
// byte buffer using bit-level reading.
type gorillaDecoder struct {
	buf          []byte
	bitPos       int // next bit position to read
	prevTime     int64
	prevDelta    int64
	prevValue    float64
	prevLeading  int
	prevTrailing int
	hasPrevTime  bool
	hasPrevValue bool
}

// newGorillaDecoder creates a decoder over the given byte slice.
func newGorillaDecoder(data []byte) *gorillaDecoder {
	return &gorillaDecoder{buf: data}
}

// readBit reads a single bit.
func (d *gorillaDecoder) readBit() byte {
	byteIdx := d.bitPos / 8
	bitIdx := 7 - (d.bitPos % 8)
	bit := (d.buf[byteIdx] >> bitIdx) & 1
	d.bitPos++
	return bit
}

// readBits reads nBits and returns as uint64 (LSB-aligned).
func (d *gorillaDecoder) readBits(nBits int) uint64 {
	var v uint64
	for i := 0; i < nBits; i++ {
		v = (v << 1) | uint64(d.readBit())
	}
	return v
}

// readTimestamp decodes a Gorilla-compressed timestamp.
func (d *gorillaDecoder) readTimestamp() int64 {
	var ts int64
	if !d.hasPrevTime {
		ts = int64(d.readBits(64))
		d.prevTime = ts
		d.prevDelta = 0
		d.hasPrevTime = true
		return ts
	}

	control := d.readBit()
	var dod int64
	if control == 0 {
		dod = 0
	} else {
		control2 := d.readBit()
		if control2 == 0 {
			dod = int64(d.readBits(7))
			// Sign-extend 7-bit value (bit 6 is sign)
			if dod&0x40 != 0 {
				dod -= 128
			}
		} else {
			control3 := d.readBit()
			if control3 == 0 {
				dod = int64(d.readBits(9))
				if dod > 256 {
					dod -= 512
				}
			} else {
				control4 := d.readBit()
				if control4 == 0 {
					dod = int64(d.readBits(12))
					if dod > 2048 {
						dod -= 4096
					}
				} else {
					dod = int64(d.readBits(32))
					// Sign-extend 32-bit value
					if dod > 0x7FFFFFFF {
						dod -= 0x100000000
					}
				}
			}
		}
	}

	dod += d.prevDelta
	ts = d.prevTime + dod
	d.prevDelta = dod
	d.prevTime = ts
	return ts
}

// readFloat decodes a Gorilla-compressed float64 value.
func (d *gorillaDecoder) readFloat() float64 {
	if !d.hasPrevValue {
		bits := d.readBits(64)
		d.prevValue = math.Float64frombits(bits)
		d.hasPrevValue = true
		return d.prevValue
	}

	bit := d.readBit()
	var val float64
	if bit == 0 {
		val = d.prevValue
	} else {
		control := d.readBit()
		var xor uint64
		if control == 0 {
			meaningfulLen := 64 - d.prevLeading - d.prevTrailing
			meaningful := d.readBits(meaningfulLen)
			xor = meaningful << d.prevTrailing
		} else {
			leading := int(d.readBits(5))
			meaningfulLen := int(d.readBits(6))
			trailing := 64 - leading - meaningfulLen
			meaningful := d.readBits(meaningfulLen)
			xor = meaningful << trailing
			d.prevLeading = leading
			d.prevTrailing = trailing
		}
		val = math.Float64frombits(math.Float64bits(d.prevValue) ^ xor)
	}

	d.prevValue = val
	return val
}

// remaining returns the number of unread bits.
func (d *gorillaDecoder) remaining() int {
	return len(d.buf)*8 - d.bitPos
}

// --- serialization helpers for the rollup store ---

// encodeRawPoint serializes a Point into the raw (uncompressed) binary format.
func encodeRawPoint(pt Point) []byte {
	size := pt.serializedSize()
	buf := make([]byte, size)
	off := 0

	// timestamp
	binary.LittleEndian.PutUint64(buf[off:], uint64(pt.Timestamp))
	off += 8
	// tags length + tags
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(pt.Tags)))
	off += 2
	copy(buf[off:], pt.Tags)
	off += len(pt.Tags)
	// name length + name
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(pt.MetricName)))
	off += 2
	copy(buf[off:], pt.MetricName)
	off += len(pt.MetricName)
	// value
	binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(pt.Value))

	return buf
}

// decodeRawPoint decodes a raw binary record into a Point.
// Returns the point and number of bytes consumed.
func decodeRawPoint(data []byte) (Point, int) {
	if len(data) < 8+2+2+8 {
		return Point{}, 0
	}
	pt := Point{}
	off := 0

	pt.Timestamp = int64(binary.LittleEndian.Uint64(data[off:]))
	off += 8

	tagsLen := int(binary.LittleEndian.Uint16(data[off:]))
	off += 2
	if off+tagsLen > len(data) {
		return Point{}, 0
	}
	pt.Tags = string(data[off : off+tagsLen])
	off += tagsLen

	nameLen := int(binary.LittleEndian.Uint16(data[off:]))
	off += 2
	if off+nameLen > len(data) {
		return Point{}, 0
	}
	pt.MetricName = string(data[off : off+nameLen])
	off += nameLen

	if off+8 > len(data) {
		return Point{}, 0
	}
	pt.Value = math.Float64frombits(binary.LittleEndian.Uint64(data[off:]))
	off += 8

	return pt, off
}

// leadingZeros64 counts the number of leading zeros in a uint64.
func leadingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x>>32 == 0 {
		n += 32
		x <<= 32
	}
	if x>>48 == 0 {
		n += 16
		x <<= 16
	}
	if x>>56 == 0 {
		n += 8
		x <<= 8
	}
	if x>>60 == 0 {
		n += 4
		x <<= 4
	}
	if x>>62 == 0 {
		n += 2
		x <<= 2
	}
	if x>>63 == 0 {
		n++
	}
	return n
}

// trailingZeros64 counts the number of trailing zeros in a uint64.
func trailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x&1 == 0 {
		for x&1 == 0 {
			n++
			x >>= 1
		}
	}
	return n
}
