package io

import (
	"fmt"
)

// Value type constants - matches Scouter Java ValueEnum
const (
	ValueTypeNULL         byte = 0
	ValueTypeBOOLEAN      byte = 10
	ValueTypeDECIMAL      byte = 20
	ValueTypeFLOAT        byte = 30
	ValueTypeDOUBLE       byte = 40
	ValueTypeTEXT         byte = 50
	ValueTypeBLOB         byte = 60
	ValueTypeIP           byte = 61
	ValueTypeLIST         byte = 70
	ValueTypeMAPVALUE     byte = 80
	ValueTypeDECIMAL_LONG byte = 21
)

// Value interface represents a value in Scouter protocol
type Value interface {
	Type() byte
	Write(out *DataOutputX)
}

// ReadValue reads a Value from DataInputX based on type
func ReadValue(in *DataInputX) (Value, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	return ReadValueOfType(in, t)
}

// ReadValueOfType reads a Value of given type from DataInputX
func ReadValueOfType(in *DataInputX, t byte) (Value, error) {
	switch t {
	case ValueTypeNULL:
		return &NullValue{}, nil
	case ValueTypeBOOLEAN:
		v, err := in.ReadBoolean()
		if err != nil {
			return nil, err
		}
		return &BooleanValue{Value: v}, nil
	case ValueTypeDECIMAL:
		// Java DecimalValue stores a long (int64), use ReadDecimal which returns int64
		v, err := in.ReadDecimal()
		if err != nil {
			return nil, err
		}
		return &DecimalLongValue{Value: v}, nil
	case ValueTypeDECIMAL_LONG:
		v, err := in.ReadDecimal()
		if err != nil {
			return nil, err
		}
		return &DecimalLongValue{Value: v}, nil
	case ValueTypeFLOAT:
		v, err := in.ReadFloat32()
		if err != nil {
			return nil, err
		}
		return &FloatValue{Value: v}, nil
	case ValueTypeDOUBLE:
		v, err := in.ReadFloat64()
		if err != nil {
			return nil, err
		}
		return &DoubleValue{Value: v}, nil
	case ValueTypeTEXT:
		v, err := in.ReadText()
		if err != nil {
			return nil, err
		}
		return &TextValue{Value: v}, nil
	case ValueTypeBLOB:
		v, err := in.ReadBlob()
		if err != nil {
			return nil, err
		}
		return &BlobValue{Value: v}, nil
	case ValueTypeIP:
		v, err := in.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		return &IPValue{Value: v}, nil
	case ValueTypeLIST:
		return readListValue(in)
	case ValueTypeMAPVALUE:
		return readMapValue(in)
	default:
		return nil, fmt.Errorf("unknown value type: %d", t)
	}
}

// NullValue represents a null value
type NullValue struct{}

func (v *NullValue) Type() byte { return ValueTypeNULL }
func (v *NullValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeNULL)
}

// BooleanValue represents a boolean value
type BooleanValue struct {
	Value bool
}

func (v *BooleanValue) Type() byte { return ValueTypeBOOLEAN }
func (v *BooleanValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeBOOLEAN)
	out.WriteBoolean(v.Value)
}

// DecimalValue represents a 32-bit integer value
type DecimalValue struct {
	Value int32
}

func (v *DecimalValue) Type() byte { return ValueTypeDECIMAL }
func (v *DecimalValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeDECIMAL)
	out.WriteDecimal32(v.Value)
}

// DecimalLongValue represents a 64-bit integer value
// Note: Java Scouter only has DECIMAL (type 20) which stores long.
// There is no separate DECIMAL_LONG type in the Java protocol.
// We use this Go type for int64 storage but always serialize as DECIMAL (type 20).
type DecimalLongValue struct {
	Value int64
}

func (v *DecimalLongValue) Type() byte { return ValueTypeDECIMAL }
func (v *DecimalLongValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeDECIMAL)
	out.WriteDecimal(v.Value)
}

// FloatValue represents a 32-bit float value
type FloatValue struct {
	Value float32
}

func (v *FloatValue) Type() byte { return ValueTypeFLOAT }
func (v *FloatValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeFLOAT)
	out.WriteFloat32(v.Value)
}

// DoubleValue represents a 64-bit float value
type DoubleValue struct {
	Value float64
}

func (v *DoubleValue) Type() byte { return ValueTypeDOUBLE }
func (v *DoubleValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeDOUBLE)
	out.WriteFloat64(v.Value)
}

// TextValue represents a string value
type TextValue struct {
	Value string
}

func (v *TextValue) Type() byte { return ValueTypeTEXT }
func (v *TextValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeTEXT)
	out.WriteText(v.Value)
}

// BlobValue represents a binary blob value
type BlobValue struct {
	Value []byte
}

func (v *BlobValue) Type() byte { return ValueTypeBLOB }
func (v *BlobValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeBLOB)
	out.WriteBlob(v.Value)
}

// IPValue represents an IP address value (4 bytes)
type IPValue struct {
	Value []byte
}

func (v *IPValue) Type() byte { return ValueTypeIP }
func (v *IPValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeIP)
	if len(v.Value) >= 4 {
		out.WriteBytes(v.Value[:4])
	} else {
		out.WriteBytes(make([]byte, 4))
	}
}

func (v *IPValue) String() string {
	if len(v.Value) < 4 {
		return "0.0.0.0"
	}
	return fmt.Sprintf("%d.%d.%d.%d", v.Value[0], v.Value[1], v.Value[2], v.Value[3])
}

// ListValue represents a list of values
type ListValue struct {
	Values []Value
}

func (v *ListValue) Type() byte { return ValueTypeLIST }
func (v *ListValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeLIST)
	out.WriteDecimal32(int32(len(v.Values)))
	for _, val := range v.Values {
		val.Write(out)
	}
}

func (v *ListValue) Add(val Value) {
	v.Values = append(v.Values, val)
}

func (v *ListValue) Size() int {
	return len(v.Values)
}

func (v *ListValue) Get(index int) Value {
	if index < 0 || index >= len(v.Values) {
		return nil
	}
	return v.Values[index]
}

// GetInt32 returns int32 value at index
func (v *ListValue) GetInt32(index int) int32 {
	if index < 0 || index >= len(v.Values) {
		return 0
	}
	if dv, ok := v.Values[index].(*DecimalValue); ok {
		return dv.Value
	}
	if dlv, ok := v.Values[index].(*DecimalLongValue); ok {
		return int32(dlv.Value)
	}
	return 0
}

// GetInt64 returns int64 value at index
func (v *ListValue) GetInt64(index int) int64 {
	if index < 0 || index >= len(v.Values) {
		return 0
	}
	if dlv, ok := v.Values[index].(*DecimalLongValue); ok {
		return dlv.Value
	}
	if dv, ok := v.Values[index].(*DecimalValue); ok {
		return int64(dv.Value)
	}
	return 0
}

// GetString returns string value at index
func (v *ListValue) GetString(index int) string {
	if index < 0 || index >= len(v.Values) {
		return ""
	}
	if tv, ok := v.Values[index].(*TextValue); ok {
		return tv.Value
	}
	return ""
}

// GetFloat64 returns float64 value at index
func (v *ListValue) GetFloat64(index int) float64 {
	if index < 0 || index >= len(v.Values) {
		return 0
	}
	if dv, ok := v.Values[index].(*DoubleValue); ok {
		return dv.Value
	}
	if fv, ok := v.Values[index].(*FloatValue); ok {
		return float64(fv.Value)
	}
	return 0
}

func readListValue(in *DataInputX) (*ListValue, error) {
	count, err := in.ReadDecimal32()
	if err != nil {
		return nil, err
	}
	lv := &ListValue{
		Values: make([]Value, 0, count),
	}
	for i := int32(0); i < count; i++ {
		val, err := ReadValue(in)
		if err != nil {
			return nil, err
		}
		lv.Values = append(lv.Values, val)
	}
	return lv, nil
}

// MapValue represents a key-value map (string keys, Value values)
type MapValue struct {
	table map[string]Value
	keys  []string // maintain insertion order
}

func NewMapValue() *MapValue {
	return &MapValue{
		table: make(map[string]Value),
		keys:  make([]string, 0),
	}
}

func (v *MapValue) Type() byte { return ValueTypeMAPVALUE }

func (v *MapValue) Write(out *DataOutputX) {
	out.WriteByte(ValueTypeMAPVALUE)
	out.WriteDecimal32(int32(len(v.keys)))
	for _, key := range v.keys {
		out.WriteText(key)
		if val, ok := v.table[key]; ok {
			val.Write(out)
		} else {
			out.WriteByte(ValueTypeNULL)
		}
	}
}

func (v *MapValue) Put(key string, val Value) {
	if _, exists := v.table[key]; !exists {
		v.keys = append(v.keys, key)
	}
	v.table[key] = val
}

func (v *MapValue) PutText(key string, text string) {
	v.Put(key, &TextValue{Value: text})
}

func (v *MapValue) PutDecimal(key string, val int32) {
	v.Put(key, &DecimalValue{Value: val})
}

func (v *MapValue) PutDecimalLong(key string, val int64) {
	v.Put(key, &DecimalLongValue{Value: val})
}

func (v *MapValue) PutFloat(key string, val float32) {
	v.Put(key, &FloatValue{Value: val})
}

func (v *MapValue) PutDouble(key string, val float64) {
	v.Put(key, &DoubleValue{Value: val})
}

func (v *MapValue) PutBoolean(key string, val bool) {
	v.Put(key, &BooleanValue{Value: val})
}

func (v *MapValue) Get(key string) Value {
	return v.table[key]
}

func (v *MapValue) GetText(key string) string {
	if val, ok := v.table[key].(*TextValue); ok {
		return val.Value
	}
	return ""
}

func (v *MapValue) GetDecimal(key string) int32 {
	if val, ok := v.table[key].(*DecimalValue); ok {
		return val.Value
	}
	return 0
}

func (v *MapValue) GetDecimalLong(key string) int64 {
	if val, ok := v.table[key].(*DecimalLongValue); ok {
		return val.Value
	}
	// Also support DecimalValue
	if val, ok := v.table[key].(*DecimalValue); ok {
		return int64(val.Value)
	}
	return 0
}

func (v *MapValue) GetFloat(key string) float32 {
	if val, ok := v.table[key].(*FloatValue); ok {
		return val.Value
	}
	return 0
}

func (v *MapValue) GetDouble(key string) float64 {
	if val, ok := v.table[key].(*DoubleValue); ok {
		return val.Value
	}
	return 0
}

func (v *MapValue) GetBoolean(key string) bool {
	if val, ok := v.table[key].(*BooleanValue); ok {
		return val.Value
	}
	return false
}

func (v *MapValue) Keys() []string {
	return v.keys
}

func (v *MapValue) Size() int {
	return len(v.keys)
}

func (v *MapValue) Contains(key string) bool {
	_, ok := v.table[key]
	return ok
}

func readMapValue(in *DataInputX) (*MapValue, error) {
	count, err := in.ReadDecimal32()
	if err != nil {
		return nil, err
	}
	mv := NewMapValue()
	for i := int32(0); i < count; i++ {
		key, err := in.ReadText()
		if err != nil {
			return nil, err
		}
		val, err := ReadValue(in)
		if err != nil {
			return nil, err
		}
		mv.Put(key, val)
	}
	return mv, nil
}

// Helper functions for creating values

func NewTextValue(s string) *TextValue {
	return &TextValue{Value: s}
}

func NewDecimalValue(v int32) *DecimalValue {
	return &DecimalValue{Value: v}
}

func NewDecimalLongValue(v int64) *DecimalLongValue {
	return &DecimalLongValue{Value: v}
}

func NewFloatValue(v float32) *FloatValue {
	return &FloatValue{Value: v}
}

func NewDoubleValue(v float64) *DoubleValue {
	return &DoubleValue{Value: v}
}

func NewBooleanValue(v bool) *BooleanValue {
	return &BooleanValue{Value: v}
}

func NewBlobValue(v []byte) *BlobValue {
	return &BlobValue{Value: v}
}

func NewIPValue(ip []byte) *IPValue {
	return &IPValue{Value: ip}
}
