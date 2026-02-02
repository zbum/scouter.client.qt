// Package io provides binary serialization for Scouter protocol
package io

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
)

// DataOutputX is a binary writer for Scouter protocol
// It is equivalent to scouter.io.DataOutputX in Java client
type DataOutputX struct {
	buf *bytes.Buffer
}

// NewDataOutputX creates a new DataOutputX
func NewDataOutputX() *DataOutputX {
	return &DataOutputX{
		buf: new(bytes.Buffer),
	}
}

// Bytes returns the underlying byte slice
func (d *DataOutputX) Bytes() []byte {
	return d.buf.Bytes()
}

// Len returns the current length
func (d *DataOutputX) Len() int {
	return d.buf.Len()
}

// WriteTo writes all data to the given writer
func (d *DataOutputX) WriteTo(w io.Writer) (int64, error) {
	return d.buf.WriteTo(w)
}

// WriteByte writes a single byte
func (d *DataOutputX) WriteByte(b byte) error {
	return d.buf.WriteByte(b)
}

// WriteBytes writes a byte slice directly (no length prefix)
func (d *DataOutputX) WriteBytes(b []byte) {
	d.buf.Write(b)
}

// WriteBoolean writes a boolean as a single byte (0 or 1)
func (d *DataOutputX) WriteBoolean(v bool) {
	if v {
		d.buf.WriteByte(1)
	} else {
		d.buf.WriteByte(0)
	}
}

// WriteInt16 writes a 16-bit integer in big-endian
func (d *DataOutputX) WriteInt16(v int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	d.buf.Write(b[:])
}

// WriteInt32 writes a 32-bit integer in big-endian
func (d *DataOutputX) WriteInt32(v int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	d.buf.Write(b[:])
}

// WriteInt64 writes a 64-bit integer in big-endian
func (d *DataOutputX) WriteInt64(v int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	d.buf.Write(b[:])
}

// WriteFloat32 writes a 32-bit float in big-endian
func (d *DataOutputX) WriteFloat32(v float32) {
	d.WriteInt32(int32(math.Float32bits(v)))
}

// WriteFloat64 writes a 64-bit float in big-endian
func (d *DataOutputX) WriteFloat64(v float64) {
	d.WriteInt64(int64(math.Float64bits(v)))
}

// WriteDecimal writes a variable-length integer (1-9 bytes)
// This matches Java Scouter's DataOutputX.writeDecimal format:
// - 0: value is 0
// - 1 + 1 byte: value fits in byte (-128 to 127)
// - 2 + 2 bytes: value fits in short (-32768 to 32767)
// - 3 + 3 bytes: value fits in int3 (-8388608 to 8388607)
// - 4 + 4 bytes: value fits in int
// - 5 + 5 bytes: value fits in long5
// - 8 + 8 bytes: full int64
func (d *DataOutputX) WriteDecimal(v int64) {
	const (
		INT3_MIN  = -8388608        // 0xff800000 as signed
		INT3_MAX  = 8388607         // 0x007fffff
		LONG5_MIN = -549755813888   // 0xffffff8000000000 as signed
		LONG5_MAX = 549755813887    // 0x0000007fffffffff
	)

	if v == 0 {
		d.buf.WriteByte(0)
	} else if v >= -128 && v <= 127 {
		// 1 + 1 byte
		d.buf.WriteByte(1)
		d.buf.WriteByte(byte(v))
	} else if v >= -32768 && v <= 32767 {
		// 2 + 2 bytes (short)
		d.buf.WriteByte(2)
		d.WriteInt16(int16(v))
	} else if v >= INT3_MIN && v <= INT3_MAX {
		// 3 + 3 bytes (int3)
		d.buf.WriteByte(3)
		d.buf.WriteByte(byte(v >> 16))
		d.buf.WriteByte(byte(v >> 8))
		d.buf.WriteByte(byte(v))
	} else if v >= -2147483648 && v <= 2147483647 {
		// 4 + 4 bytes (int)
		d.buf.WriteByte(4)
		d.WriteInt32(int32(v))
	} else if v >= LONG5_MIN && v <= LONG5_MAX {
		// 5 + 5 bytes (long5)
		d.buf.WriteByte(5)
		d.buf.WriteByte(byte(v >> 32))
		d.buf.WriteByte(byte(v >> 24))
		d.buf.WriteByte(byte(v >> 16))
		d.buf.WriteByte(byte(v >> 8))
		d.buf.WriteByte(byte(v))
	} else {
		// 8 + 8 bytes (full long)
		d.buf.WriteByte(8)
		d.WriteInt64(v)
	}
}

// WriteDecimal32 writes a variable-length 32-bit integer
func (d *DataOutputX) WriteDecimal32(v int32) {
	d.WriteDecimal(int64(v))
}

// WriteBlob writes a length-prefixed byte array
// Matches Java Scouter encoding:
// - len <= 253: 1 byte length
// - len > 253 && len <= 65535: byte 255 + 2 bytes length (big-endian)
func (d *DataOutputX) WriteBlob(b []byte) {
	if b == nil || len(b) == 0 {
		d.buf.WriteByte(0)
		return
	}
	length := len(b)
	if length <= 253 {
		d.buf.WriteByte(byte(length))
		d.buf.Write(b)
	} else if length <= 65535 {
		d.buf.WriteByte(255)
		d.WriteInt16(int16(length))
		d.buf.Write(b)
	} else {
		// For very large blobs, use 254 marker + 4-byte length
		d.buf.WriteByte(254)
		d.WriteInt32(int32(length))
		d.buf.Write(b)
	}
}

// WriteText writes a UTF-8 string with length prefix
func (d *DataOutputX) WriteText(s string) {
	d.WriteBlob([]byte(s))
}

// WriteTextShort writes a string with 2-byte length prefix
func (d *DataOutputX) WriteTextShort(s string) {
	b := []byte(s)
	d.WriteInt16(int16(len(b)))
	d.buf.Write(b)
}

// Reset clears the buffer
func (d *DataOutputX) Reset() {
	d.buf.Reset()
}
