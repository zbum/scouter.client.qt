package io

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// DataInputX is a binary reader for Scouter protocol
// It is equivalent to scouter.io.DataInputX in Java client
type DataInputX struct {
	buf    *bytes.Reader
	reader io.Reader // For streaming mode
	data   []byte
	offset int
}

// NewDataInputX creates a new DataInputX from byte slice
func NewDataInputX(data []byte) *DataInputX {
	return &DataInputX{
		buf:    bytes.NewReader(data),
		data:   data,
		offset: 0,
	}
}

// NewDataInputXFromReader creates a new DataInputX that reads from io.Reader on demand
func NewDataInputXFromReader(r io.Reader) (*DataInputX, error) {
	return &DataInputX{
		reader: r,
		offset: 0,
	}, nil
}

// Remaining returns the number of remaining bytes
// Returns -1 for streaming mode (unknown remaining)
func (d *DataInputX) Remaining() int {
	if d.buf != nil {
		return d.buf.Len()
	}
	return -1 // Unknown for streaming mode
}

// Offset returns current read position
func (d *DataInputX) Offset() int {
	return d.offset
}

// ReadByte reads a single byte
func (d *DataInputX) ReadByte() (byte, error) {
	if d.buf != nil {
		b, err := d.buf.ReadByte()
		if err != nil {
			return 0, err
		}
		d.offset++
		return b, nil
	}
	// Streaming mode
	var buf [1]byte
	_, err := io.ReadFull(d.reader, buf[:])
	if err != nil {
		return 0, err
	}
	d.offset++
	return buf[0], nil
}

// ReadBytes reads n bytes
func (d *DataInputX) ReadBytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	b := make([]byte, n)
	if d.buf != nil {
		read, err := d.buf.Read(b)
		if err != nil {
			return nil, err
		}
		if read != n {
			return nil, fmt.Errorf("expected %d bytes, got %d", n, read)
		}
	} else {
		// Streaming mode
		_, err := io.ReadFull(d.reader, b)
		if err != nil {
			return nil, err
		}
	}
	d.offset += n
	return b, nil
}

// ReadBoolean reads a boolean (0 = false, otherwise true)
func (d *DataInputX) ReadBoolean() (bool, error) {
	b, err := d.ReadByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// ReadInt16 reads a 16-bit integer in big-endian
func (d *DataInputX) ReadInt16() (int16, error) {
	b, err := d.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16(b)), nil
}

// ReadInt32 reads a 32-bit integer in big-endian
func (d *DataInputX) ReadInt32() (int32, error) {
	b, err := d.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(b)), nil
}

// ReadInt64 reads a 64-bit integer in big-endian
func (d *DataInputX) ReadInt64() (int64, error) {
	b, err := d.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

// ReadFloat32 reads a 32-bit float in big-endian
func (d *DataInputX) ReadFloat32() (float32, error) {
	v, err := d.ReadInt32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(uint32(v)), nil
}

// ReadFloat64 reads a 64-bit float in big-endian
func (d *DataInputX) ReadFloat64() (float64, error) {
	v, err := d.ReadInt64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(v)), nil
}

// ReadDecimal reads a variable-length integer (1-9 bytes)
// This matches Java Scouter's DataInputX.readDecimal format:
// - 0: value is 0
// - 1: next 1 byte is the value (signed byte)
// - 2: next 2 bytes is the value (signed short)
// - 3: next 3 bytes is the value (signed int3)
// - 4: next 4 bytes is the value (signed int)
// - 5: next 5 bytes is the value (signed long5)
// - 8: next 8 bytes is the value (signed long)
func (d *DataInputX) ReadDecimal() (int64, error) {
	lenByte, err := d.ReadByte()
	if err != nil {
		return 0, err
	}

	switch lenByte {
	case 0:
		return 0, nil
	case 1:
		// 1 byte signed
		b, err := d.ReadByte()
		if err != nil {
			return 0, err
		}
		return int64(int8(b)), nil
	case 2:
		// 2 bytes signed short
		v, err := d.ReadInt16()
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	case 3:
		// 3 bytes signed int3
		b, err := d.ReadBytes(3)
		if err != nil {
			return 0, err
		}
		// Sign extend: shift left 8, then arithmetic right shift 8
		v := int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8
		return int64(v >> 8), nil
	case 4:
		// 4 bytes signed int
		v, err := d.ReadInt32()
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	case 5:
		// 5 bytes signed long5
		b, err := d.ReadBytes(5)
		if err != nil {
			return 0, err
		}
		// First byte keeps sign (cast to int8 first for sign extension)
		v := int64(int8(b[0]))<<32 |
			int64(b[1])<<24 |
			int64(b[2])<<16 |
			int64(b[3])<<8 |
			int64(b[4])
		return v, nil
	default:
		// 8 bytes (full long)
		return d.ReadInt64()
	}
}

// ReadDecimal32 reads a variable-length 32-bit integer
func (d *DataInputX) ReadDecimal32() (int32, error) {
	v, err := d.ReadDecimal()
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// ReadBlob reads a length-prefixed byte array
// Matches Java Scouter encoding:
// - len <= 253: 1 byte length
// - len == 255: next 2 bytes are length (big-endian)
// - len == 254: next 4 bytes are length (big-endian)
func (d *DataInputX) ReadBlob() ([]byte, error) {
	lenByte, err := d.ReadByte()
	if err != nil {
		return nil, err
	}
	if lenByte == 0 {
		return nil, nil
	}

	var length int
	if lenByte <= 253 {
		length = int(lenByte)
	} else if lenByte == 255 {
		// 2-byte length follows
		len16, err := d.ReadInt16()
		if err != nil {
			return nil, err
		}
		length = int(uint16(len16))
	} else if lenByte == 254 {
		// 4-byte length follows
		len32, err := d.ReadInt32()
		if err != nil {
			return nil, err
		}
		length = int(len32)
	} else {
		return nil, fmt.Errorf("unsupported blob length prefix: 0x%02X", lenByte)
	}

	if length == 0 {
		return nil, nil
	}
	return d.ReadBytes(length)
}

// ReadText reads a UTF-8 string with length prefix
func (d *DataInputX) ReadText() (string, error) {
	b, err := d.ReadBlob()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadTextShort reads a string with 2-byte length prefix
func (d *DataInputX) ReadTextShort() (string, error) {
	length, err := d.ReadInt16()
	if err != nil {
		return "", err
	}
	if length <= 0 {
		return "", nil
	}
	b, err := d.ReadBytes(int(length))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Skip skips n bytes
func (d *DataInputX) Skip(n int) error {
	_, err := d.ReadBytes(n)
	return err
}
