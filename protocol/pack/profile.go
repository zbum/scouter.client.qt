package pack

import (
	"scouter.client.qt/protocol/io"
)

// Profile step type constants
const (
	StepTypeNone       byte = 0
	StepTypeMethod     byte = 1
	StepTypeSql        byte = 2
	StepTypeSql2       byte = 8
	StepTypeSql3       byte = 16
	StepTypeApiCall    byte = 5
	StepTypeApiCall2   byte = 15
	StepTypeSocket     byte = 6
	StepTypeThread     byte = 7
	StepTypeHashed     byte = 10
	StepTypeParam      byte = 3
	StepTypeError      byte = 4
	StepTypeDispatch   byte = 9
	StepTypeDump       byte = 11
	StepTypeMessage    byte = 12
	StepTypeControlSpan byte = 13
	StepTypeSpan       byte = 14
	StepTypeMethodSum  byte = 17
	StepTypeSqlSum     byte = 18
	StepTypeApiCallSum byte = 19
	StepTypeSocketSum  byte = 20
)

// Step represents a profile step
type Step interface {
	StepType() byte
	Write(out *io.DataOutputX)
}

// BaseStep provides common step fields
type BaseStep struct {
	StartTime int32 // Relative time from transaction start (ms)
}

// MethodStep represents a method call
type MethodStep struct {
	BaseStep
	Hash    int32 // Method name hash
	Elapsed int32 // Execution time (ms)
}

func (s *MethodStep) StepType() byte { return StepTypeMethod }
func (s *MethodStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeMethod)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteDecimal32(s.Elapsed)
}

// SqlStep represents a SQL execution
type SqlStep struct {
	BaseStep
	Hash     int32  // SQL hash
	Elapsed  int32  // Execution time (ms)
	Param    string // SQL parameters
	Error    int32  // Error hash
	BindHash int32  // Bind variable hash
}

func (s *SqlStep) StepType() byte { return StepTypeSql }
func (s *SqlStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeSql)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteDecimal32(s.Elapsed)
	out.WriteText(s.Param)
	out.WriteDecimal32(s.Error)
}

// SqlStep2 represents a SQL execution with extended info
type SqlStep2 struct {
	SqlStep
}

func (s *SqlStep2) StepType() byte { return StepTypeSql2 }
func (s *SqlStep2) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeSql2)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteDecimal32(s.Elapsed)
	out.WriteText(s.Param)
	out.WriteDecimal32(s.Error)
	out.WriteDecimal32(s.BindHash)
}

// ApiCallStep represents an API call
type ApiCallStep struct {
	BaseStep
	Hash    int32 // API call hash (URL)
	Elapsed int32 // Execution time (ms)
	TxID    int64 // Called transaction ID (if available)
	Error   int32 // Error hash
	Opt     byte  // Options
}

func (s *ApiCallStep) StepType() byte { return StepTypeApiCall }
func (s *ApiCallStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeApiCall)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteDecimal32(s.Elapsed)
}

// ApiCallStep2 represents an API call with extended info
type ApiCallStep2 struct {
	ApiCallStep
}

func (s *ApiCallStep2) StepType() byte { return StepTypeApiCall2 }
func (s *ApiCallStep2) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeApiCall2)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteDecimal32(s.Elapsed)
	out.WriteInt64(s.TxID)
	out.WriteDecimal32(s.Error)
	out.WriteByte(s.Opt)
}

// SocketStep represents a socket operation
type SocketStep struct {
	BaseStep
	IPAddr  []byte // Remote IP (4 bytes)
	Port    int32  // Remote port
	Elapsed int32  // Execution time (ms)
	Error   int32  // Error hash
}

func (s *SocketStep) StepType() byte { return StepTypeSocket }
func (s *SocketStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeSocket)
	out.WriteDecimal32(s.StartTime)
	if len(s.IPAddr) >= 4 {
		out.WriteBytes(s.IPAddr[:4])
	} else {
		out.WriteBytes(make([]byte, 4))
	}
	out.WriteDecimal32(s.Port)
	out.WriteDecimal32(s.Elapsed)
	out.WriteDecimal32(s.Error)
}

// MessageStep represents a message/log
type MessageStep struct {
	BaseStep
	Hash    int32  // Message hash
	Value   string // Additional value
	Elapsed int32  // Execution time (ms)
}

func (s *MessageStep) StepType() byte { return StepTypeMessage }
func (s *MessageStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeMessage)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteText(s.Value)
	out.WriteDecimal32(s.Elapsed)
}

// ErrorStep represents an error
type ErrorStep struct {
	BaseStep
	Hash    int32  // Error message hash
	Message string // Error details
}

func (s *ErrorStep) StepType() byte { return StepTypeError }
func (s *ErrorStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeError)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteText(s.Message)
}

// HashedStep represents a generic hashed step
type HashedStep struct {
	BaseStep
	Hash  int32 // Hash value
	Value string
	Time  int32 // Elapsed time
}

func (s *HashedStep) StepType() byte { return StepTypeHashed }
func (s *HashedStep) Write(out *io.DataOutputX) {
	out.WriteByte(StepTypeHashed)
	out.WriteDecimal32(s.StartTime)
	out.WriteDecimal32(s.Hash)
	out.WriteText(s.Value)
	out.WriteDecimal32(s.Time)
}

// XLogProfilePack represents profile data for an XLog
type XLogProfilePack struct {
	TxID  int64  // Transaction ID
	Steps []Step // Profile steps
}

func (p *XLogProfilePack) PackType() byte {
	return XLOG_PROFILE
}

func (p *XLogProfilePack) Write(out *io.DataOutputX) {
	out.WriteInt64(p.TxID)
	out.WriteDecimal32(int32(len(p.Steps)))
	for _, step := range p.Steps {
		step.Write(out)
	}
}

// ReadXLogProfilePack reads an XLogProfilePack from DataInputX
func ReadXLogProfilePack(in *io.DataInputX) (*XLogProfilePack, error) {
	p := &XLogProfilePack{}
	var err error

	if p.TxID, err = in.ReadInt64(); err != nil {
		return nil, err
	}

	count, err := in.ReadDecimal32()
	if err != nil {
		return nil, err
	}

	p.Steps = make([]Step, 0, count)
	for i := int32(0); i < count; i++ {
		step, err := ReadStep(in)
		if err != nil {
			return nil, err
		}
		if step != nil {
			p.Steps = append(p.Steps, step)
		}
	}

	return p, nil
}

// ReadStep reads a single step from DataInputX
func ReadStep(in *io.DataInputX) (Step, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, err
	}

	switch t {
	case StepTypeMethod:
		return readMethodStep(in)
	case StepTypeSql:
		return readSqlStep(in)
	case StepTypeSql2:
		return readSqlStep2(in)
	case StepTypeApiCall:
		return readApiCallStep(in)
	case StepTypeApiCall2:
		return readApiCallStep2(in)
	case StepTypeSocket:
		return readSocketStep(in)
	case StepTypeMessage:
		return readMessageStep(in)
	case StepTypeError:
		return readErrorStep(in)
	case StepTypeHashed:
		return readHashedStep(in)
	default:
		// Unknown step type, try to skip
		return nil, nil
	}
}

func readMethodStep(in *io.DataInputX) (*MethodStep, error) {
	s := &MethodStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSqlStep(in *io.DataInputX) (*SqlStep, error) {
	s := &SqlStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Param, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSqlStep2(in *io.DataInputX) (*SqlStep2, error) {
	s := &SqlStep2{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Param, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.BindHash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readApiCallStep(in *io.DataInputX) (*ApiCallStep, error) {
	s := &ApiCallStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readApiCallStep2(in *io.DataInputX) (*ApiCallStep2, error) {
	s := &ApiCallStep2{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.TxID, err = in.ReadInt64(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Opt, err = in.ReadByte(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSocketStep(in *io.DataInputX) (*SocketStep, error) {
	s := &SocketStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.IPAddr, err = in.ReadBytes(4); err != nil {
		return nil, err
	}
	if s.Port, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readMessageStep(in *io.DataInputX) (*MessageStep, error) {
	s := &MessageStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Value, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readErrorStep(in *io.DataInputX) (*ErrorStep, error) {
	s := &ErrorStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Message, err = in.ReadText(); err != nil {
		return nil, err
	}
	return s, nil
}

func readHashedStep(in *io.DataInputX) (*HashedStep, error) {
	s := &HashedStep{}
	var err error
	if s.StartTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Value, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Time, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}
