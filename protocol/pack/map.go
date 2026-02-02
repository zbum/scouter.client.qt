package pack

import (
	"scouter.client.qt/protocol/io"
)

// MapPack represents a key-value map packet
// This is the most commonly used pack type for request/response
type MapPack struct {
	table map[string]io.Value
	keys  []string
}

// NewMapPack creates a new MapPack
func NewMapPack() *MapPack {
	return &MapPack{
		table: make(map[string]io.Value),
		keys:  make([]string, 0),
	}
}

func (p *MapPack) PackType() byte {
	return MAP
}

func (p *MapPack) Write(out *io.DataOutputX) {
	out.WriteDecimal32(int32(len(p.keys)))
	for _, key := range p.keys {
		out.WriteText(key)
		if val, ok := p.table[key]; ok {
			val.Write(out)
		} else {
			out.WriteByte(io.ValueTypeNULL)
		}
	}
}

// ReadMapPack reads a MapPack from DataInputX
func ReadMapPack(in *io.DataInputX) (*MapPack, error) {
	count, err := in.ReadDecimal32()
	if err != nil {
		return nil, err
	}
	p := NewMapPack()
	for i := int32(0); i < count; i++ {
		key, err := in.ReadText()
		if err != nil {
			return nil, err
		}
		val, err := io.ReadValue(in)
		if err != nil {
			return nil, err
		}
		p.Put(key, val)
	}
	return p, nil
}

// Put adds or updates a key-value pair
func (p *MapPack) Put(key string, val io.Value) {
	if _, exists := p.table[key]; !exists {
		p.keys = append(p.keys, key)
	}
	p.table[key] = val
}

// PutText adds a text value
func (p *MapPack) PutText(key string, text string) {
	p.Put(key, io.NewTextValue(text))
}

// PutDecimal adds a decimal value
func (p *MapPack) PutDecimal(key string, val int32) {
	p.Put(key, io.NewDecimalValue(val))
}

// PutDecimalLong adds a long decimal value
func (p *MapPack) PutDecimalLong(key string, val int64) {
	p.Put(key, io.NewDecimalLongValue(val))
}

// PutFloat adds a float value
func (p *MapPack) PutFloat(key string, val float32) {
	p.Put(key, io.NewFloatValue(val))
}

// PutDouble adds a double value
func (p *MapPack) PutDouble(key string, val float64) {
	p.Put(key, io.NewDoubleValue(val))
}

// PutBoolean adds a boolean value
func (p *MapPack) PutBoolean(key string, val bool) {
	p.Put(key, io.NewBooleanValue(val))
}

// Get retrieves a value by key
func (p *MapPack) Get(key string) io.Value {
	return p.table[key]
}

// GetText retrieves a text value
func (p *MapPack) GetText(key string) string {
	if val, ok := p.table[key].(*io.TextValue); ok {
		return val.Value
	}
	return ""
}

// GetDecimal retrieves a decimal value
func (p *MapPack) GetDecimal(key string) int32 {
	if val, ok := p.table[key].(*io.DecimalValue); ok {
		return val.Value
	}
	if val, ok := p.table[key].(*io.DecimalLongValue); ok {
		return int32(val.Value)
	}
	return 0
}

// GetDecimalLong retrieves a long decimal value
func (p *MapPack) GetDecimalLong(key string) int64 {
	if val, ok := p.table[key].(*io.DecimalLongValue); ok {
		return val.Value
	}
	if val, ok := p.table[key].(*io.DecimalValue); ok {
		return int64(val.Value)
	}
	return 0
}

// GetFloat retrieves a float value
func (p *MapPack) GetFloat(key string) float32 {
	if val, ok := p.table[key].(*io.FloatValue); ok {
		return val.Value
	}
	return 0
}

// GetDouble retrieves a double value
func (p *MapPack) GetDouble(key string) float64 {
	if val, ok := p.table[key].(*io.DoubleValue); ok {
		return val.Value
	}
	return 0
}

// GetBoolean retrieves a boolean value
func (p *MapPack) GetBoolean(key string) bool {
	if val, ok := p.table[key].(*io.BooleanValue); ok {
		return val.Value
	}
	return false
}

// Keys returns all keys in insertion order
func (p *MapPack) Keys() []string {
	return p.keys
}

// Size returns the number of key-value pairs
func (p *MapPack) Size() int {
	return len(p.keys)
}

// Contains checks if a key exists
func (p *MapPack) Contains(key string) bool {
	_, ok := p.table[key]
	return ok
}

// Clear removes all key-value pairs
func (p *MapPack) Clear() {
	p.table = make(map[string]io.Value)
	p.keys = make([]string, 0)
}

// GetListValue retrieves a list value
func (p *MapPack) GetListValue(key string) *io.ListValue {
	if val, ok := p.table[key].(*io.ListValue); ok {
		return val
	}
	return nil
}

// GetMapValue retrieves a map value
func (p *MapPack) GetMapValue(key string) *io.MapValue {
	if val, ok := p.table[key].(*io.MapValue); ok {
		return val
	}
	return nil
}
