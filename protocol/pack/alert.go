package pack

import (
	"scouter.client.qt/protocol/io"
)

// Alert level constants
const (
	AlertLevelInfo  byte = 0
	AlertLevelWarn  byte = 1
	AlertLevelError byte = 2
	AlertLevelFatal byte = 3
)

// AlertPack represents an alert notification
type AlertPack struct {
	Time      int64  // Alert time (ms since epoch)
	ObjHash   int32  // Object hash
	ObjType   string // Object type
	Level     byte   // Alert level (INFO, WARN, ERROR, FATAL)
	Title     string // Alert title
	Message   string // Alert message
	Tags      *io.MapValue
}

// NewAlertPack creates a new AlertPack
func NewAlertPack() *AlertPack {
	return &AlertPack{
		Tags: io.NewMapValue(),
	}
}

func (p *AlertPack) PackType() byte {
	return ALERT
}

func (p *AlertPack) Write(out *io.DataOutputX) {
	out.WriteDecimal(p.Time)
	out.WriteDecimal32(p.ObjHash)
	out.WriteText(p.ObjType)
	out.WriteByte(p.Level)
	out.WriteText(p.Title)
	out.WriteText(p.Message)
	p.Tags.Write(out)
}

// ReadAlertPack reads an AlertPack from DataInputX
func ReadAlertPack(in *io.DataInputX) (*AlertPack, error) {
	p := &AlertPack{}
	var err error

	if p.Time, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if p.ObjHash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.ObjType, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Level, err = in.ReadByte(); err != nil {
		return nil, err
	}
	if p.Title, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Message, err = in.ReadText(); err != nil {
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

// LevelName returns the human-readable level name
func (p *AlertPack) LevelName() string {
	switch p.Level {
	case AlertLevelInfo:
		return "INFO"
	case AlertLevelWarn:
		return "WARN"
	case AlertLevelError:
		return "ERROR"
	case AlertLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// IsError returns true if level is ERROR or FATAL
func (p *AlertPack) IsError() bool {
	return p.Level >= AlertLevelError
}

// GetTag retrieves a tag value
func (p *AlertPack) GetTag(name string) string {
	return p.Tags.GetText(name)
}

// SetTag sets a tag value
func (p *AlertPack) SetTag(name string, value string) {
	p.Tags.PutText(name, value)
}
