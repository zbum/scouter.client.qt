package pack

import (
	"scouter.client.qt/protocol/io"
)

// ObjectPack represents an agent object information
type ObjectPack struct {
	ObjHash   int32  // Object hash (unique identifier)
	ObjName   string // Object name (e.g., "/host1/tomcat1")
	ObjType   string // Object type (e.g., "javaee", "host", "golang")
	Address   string // IP address
	Version   string // Agent version
	Alive     bool   // Whether the agent is alive
	WakeUp    int64  // Last wake up time
	Tags      *io.MapValue
}

// NewObjectPack creates a new ObjectPack
func NewObjectPack() *ObjectPack {
	return &ObjectPack{
		Tags: io.NewMapValue(),
	}
}

func (p *ObjectPack) PackType() byte {
	return OBJECT
}

func (p *ObjectPack) Write(out *io.DataOutputX) {
	// Java order: objType, objHash, objName, address, version, alive, wakeup, tags
	out.WriteText(p.ObjType)
	out.WriteDecimal(int64(p.ObjHash))
	out.WriteText(p.ObjName)
	out.WriteText(p.Address)
	out.WriteText(p.Version)
	out.WriteBoolean(p.Alive)
	out.WriteDecimal(p.WakeUp)
	p.Tags.Write(out)
}

// ReadObjectPack reads an ObjectPack from DataInputX
// Field order must match Java: objType, objHash, objName, address, version, alive, wakeup, tags
func ReadObjectPack(in *io.DataInputX) (*ObjectPack, error) {
	p := &ObjectPack{}
	var err error

	// Java order: objType first, then objHash
	if p.ObjType, err = in.ReadText(); err != nil {
		return nil, err
	}
	// Java uses readDecimal() which returns long, then casts to int
	hash, err := in.ReadDecimal()
	if err != nil {
		return nil, err
	}
	p.ObjHash = int32(hash)

	if p.ObjName, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Address, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Version, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Alive, err = in.ReadBoolean(); err != nil {
		return nil, err
	}
	if p.WakeUp, err = in.ReadDecimal(); err != nil {
		return nil, err
	}

	// Read Tags MapValue
	val, err := io.ReadValue(in)
	if err != nil {
		return nil, err
	}
	if mv, ok := val.(*io.MapValue); ok {
		p.Tags = mv
	} else {
		p.Tags = io.NewMapValue()
	}

	return p, nil
}

// GetTag retrieves a tag value
func (p *ObjectPack) GetTag(name string) string {
	return p.Tags.GetText(name)
}

// SetTag sets a tag value
func (p *ObjectPack) SetTag(name string, value string) {
	p.Tags.PutText(name, value)
}

// ExtractObjFamily extracts the family part from object name
// e.g., "/host/tomcat1" -> "/host"
func (p *ObjectPack) ExtractObjFamily() string {
	name := p.ObjName
	if len(name) == 0 {
		return ""
	}
	// Find last '/'
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			return name[:i]
		}
	}
	return ""
}

// ExtractShortName extracts the short name from object name
// e.g., "/host/tomcat1" -> "tomcat1"
func (p *ObjectPack) ExtractShortName() string {
	name := p.ObjName
	if len(name) == 0 {
		return ""
	}
	// Find last '/'
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			if i+1 < len(name) {
				return name[i+1:]
			}
			return ""
		}
	}
	return name
}
