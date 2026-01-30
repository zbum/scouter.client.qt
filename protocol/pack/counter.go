package pack

import (
	"scouter.client.qt/protocol/io"
)

// PerfCounterPack represents a performance counter
type PerfCounterPack struct {
	ObjHash   int32  // Object(agent) hash
	ObjName   string // Object name
	ObjType   string // Object type
	TimeStamp int64  // Collection timestamp (ms)
	Data      *io.MapValue
}

// NewPerfCounterPack creates a new PerfCounterPack
func NewPerfCounterPack() *PerfCounterPack {
	return &PerfCounterPack{
		Data: io.NewMapValue(),
	}
}

func (p *PerfCounterPack) PackType() byte {
	return PERF_COUNTER
}

func (p *PerfCounterPack) Write(out *io.DataOutputX) {
	out.WriteDecimal32(p.ObjHash)
	out.WriteText(p.ObjName)
	out.WriteText(p.ObjType)
	out.WriteDecimal(p.TimeStamp)
	p.Data.Write(out)
}

// ReadPerfCounterPack reads a PerfCounterPack from DataInputX
func ReadPerfCounterPack(in *io.DataInputX) (*PerfCounterPack, error) {
	p := &PerfCounterPack{}
	var err error

	if p.ObjHash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.ObjName, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.ObjType, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.TimeStamp, err = in.ReadDecimal(); err != nil {
		return nil, err
	}

	// Read MapValue (type byte is included in MapValue's Write/Read)
	val, err := io.ReadValue(in)
	if err != nil {
		return nil, err
	}
	if mv, ok := val.(*io.MapValue); ok {
		p.Data = mv
	} else {
		p.Data = io.NewMapValue()
	}

	return p, nil
}

// Put adds a counter value
func (p *PerfCounterPack) Put(name string, val io.Value) {
	p.Data.Put(name, val)
}

// PutLong adds a long counter value
func (p *PerfCounterPack) PutLong(name string, val int64) {
	p.Data.PutDecimalLong(name, val)
}

// PutInt adds an int counter value
func (p *PerfCounterPack) PutInt(name string, val int32) {
	p.Data.PutDecimal(name, val)
}

// PutFloat adds a float counter value
func (p *PerfCounterPack) PutFloat(name string, val float32) {
	p.Data.PutFloat(name, val)
}

// PutDouble adds a double counter value
func (p *PerfCounterPack) PutDouble(name string, val float64) {
	p.Data.PutDouble(name, val)
}

// Get retrieves a counter value
func (p *PerfCounterPack) Get(name string) io.Value {
	return p.Data.Get(name)
}

// GetLong retrieves a long counter value
func (p *PerfCounterPack) GetLong(name string) int64 {
	return p.Data.GetDecimalLong(name)
}

// GetInt retrieves an int counter value
func (p *PerfCounterPack) GetInt(name string) int32 {
	return p.Data.GetDecimal(name)
}

// GetFloat retrieves a float counter value
func (p *PerfCounterPack) GetFloat(name string) float32 {
	return p.Data.GetFloat(name)
}

// GetDouble retrieves a double counter value
func (p *PerfCounterPack) GetDouble(name string) float64 {
	return p.Data.GetDouble(name)
}

// CounterNames returns all counter names
func (p *PerfCounterPack) CounterNames() []string {
	return p.Data.Keys()
}
