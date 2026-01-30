// Package pack provides Pack types for Scouter protocol
package pack

import (
	"fmt"

	"scouter.client.qt/protocol/io"
)

// Pack type constants - matches Scouter Java PackEnum
const (
	CYCLIC_MAP          byte = 1
	CYCLIC_LIST         byte = 2
	MAP                 byte = 10
	PERF_COUNTER        byte = 60
	PERF_INTERACTION_COUNTER byte = 61
	XLOG                byte = 21
	XLOG_PROFILE        byte = 26
	TEXT                byte = 50
	ALERT               byte = 55
	OBJECT              byte = 80
	OBJECT_PERF         byte = 81
	STATUS              byte = 82
	STACK               byte = 90
	SUMMARY             byte = 91
	BATCH               byte = 92
	SPAN_CONTAINER      byte = 93
	INTERACTIONPERF     byte = 94
)

// Pack interface represents a data packet in Scouter protocol
type Pack interface {
	PackType() byte
	Write(out *io.DataOutputX)
}

// ReadPack reads a Pack from DataInputX based on type prefix
func ReadPack(in *io.DataInputX) (Pack, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	return ReadPackOfType(in, t)
}

// ReadPackOfType reads a Pack of given type from DataInputX
func ReadPackOfType(in *io.DataInputX, t byte) (Pack, error) {
	switch t {
	case MAP:
		return ReadMapPack(in)
	case PERF_COUNTER:
		return ReadPerfCounterPack(in)
	case XLOG:
		return ReadXLogPack(in)
	case XLOG_PROFILE:
		return ReadXLogProfilePack(in)
	case TEXT:
		return ReadTextPack(in)
	case ALERT:
		return ReadAlertPack(in)
	case OBJECT:
		return ReadObjectPack(in)
	default:
		return nil, fmt.Errorf("unknown pack type: %d", t)
	}
}

// WritePack writes a Pack to DataOutputX with type prefix
func WritePack(out *io.DataOutputX, p Pack) {
	out.WriteByte(p.PackType())
	p.Write(out)
}
