package pack

import (
	"fmt"
	"scouter.client.qt/protocol/io"
)

// Profile step type constants - must match Java StepEnum exactly
const (
	StepTypeMethod              byte = 1
	StepTypeSql                 byte = 2
	StepTypeMessage             byte = 3
	StepTypeSocket              byte = 5
	StepTypeApiCall             byte = 6
	StepTypeThreadSubmit        byte = 7
	StepTypeSql2                byte = 8
	StepTypeHashedMessage       byte = 9
	StepTypeMethod2             byte = 10
	StepTypeMethodSum           byte = 11
	StepTypeDump                byte = 12
	StepTypeDispatch            byte = 13
	StepTypeThreadCallPossible  byte = 14
	StepTypeApiCall2            byte = 15
	StepTypeSql3                byte = 16
	StepTypeParameterizedMsg    byte = 17
	StepTypeSqlSum              byte = 21
	StepTypeMessageSum          byte = 31
	StepTypeSocketSum           byte = 42
	StepTypeApiCallSum          byte = 43
	StepTypeSpan                byte = 51
	StepTypeSpanCall            byte = 52
	StepTypeControl             byte = 99
)

// Step represents a profile step
type Step interface {
	StepType() byte
}

// BaseStep provides common StepSingle fields (parent, index, start_time, start_cpu)
type BaseStep struct {
	Parent    int32
	Index     int32
	StartTime int32
	StartCPU  int32
}

// readBaseStep reads the 4 StepSingle fields
func readBaseStep(in *io.DataInputX) (BaseStep, error) {
	var b BaseStep
	var err error
	if b.Parent, err = in.ReadDecimal32(); err != nil {
		return b, err
	}
	if b.Index, err = in.ReadDecimal32(); err != nil {
		return b, err
	}
	if b.StartTime, err = in.ReadDecimal32(); err != nil {
		return b, err
	}
	if b.StartCPU, err = in.ReadDecimal32(); err != nil {
		return b, err
	}
	return b, nil
}

// MethodStep represents a method call (type 1)
type MethodStep struct {
	BaseStep
	Hash    int32
	Elapsed int32
	CPUTime int32
}

func (s *MethodStep) StepType() byte { return StepTypeMethod }

// Method2Step represents a method call with error (type 10)
type Method2Step struct {
	MethodStep
	Error int32
}

func (s *Method2Step) StepType() byte { return StepTypeMethod2 }

// SqlStep represents a SQL execution (type 2)
type SqlStep struct {
	BaseStep
	Hash    int32
	Elapsed int32
	CPUTime int32
	Param   string
	Error   int32
}

func (s *SqlStep) StepType() byte { return StepTypeSql }

// SqlStep2 represents SQL with extended info (type 8)
type SqlStep2 struct {
	SqlStep
	XType byte
}

func (s *SqlStep2) StepType() byte { return StepTypeSql2 }

// SqlStep3 represents SQL v3 with updated count (type 16)
type SqlStep3 struct {
	SqlStep2
	Updated int32
}

func (s *SqlStep3) StepType() byte { return StepTypeSql3 }

// MessageStep represents a message/log (type 3) - just a text string, no hash
type MessageStep struct {
	BaseStep
	Message string
}

func (s *MessageStep) StepType() byte { return StepTypeMessage }

// HashedMessageStep represents a hashed message (type 9)
type HashedMessageStep struct {
	BaseStep
	Hash  int32
	Time  int32
	Value int32
}

func (s *HashedMessageStep) StepType() byte { return StepTypeHashedMessage }

// SocketStep represents a socket operation (type 5)
type SocketStep struct {
	BaseStep
	IPAddr  []byte
	Port    int32
	Elapsed int32
	Error   int32
}

func (s *SocketStep) StepType() byte { return StepTypeSocket }

// ApiCallStep represents an API call (type 6)
type ApiCallStep struct {
	BaseStep
	TxID    int64
	Hash    int32
	Elapsed int32
	CPUTime int32
	Error   int32
	Opt     byte
	Address string
}

func (s *ApiCallStep) StepType() byte { return StepTypeApiCall }

// ApiCallStep2 represents an API call v2 (type 15)
type ApiCallStep2 struct {
	ApiCallStep
	Async byte
}

func (s *ApiCallStep2) StepType() byte { return StepTypeApiCall2 }

// ThreadSubmitStep represents a thread submit (type 7)
type ThreadSubmitStep struct {
	BaseStep
	TxID    int64
	Hash    int32
	Elapsed int32
	CPUTime int32
	Error   int32
}

func (s *ThreadSubmitStep) StepType() byte { return StepTypeThreadSubmit }

// DispatchStep represents a dispatch (type 13)
type DispatchStep struct {
	BaseStep
	TxID    int64
	Hash    int32
	Elapsed int32
	CPUTime int32
	Error   int32
	Opt     byte
	Address string
}

func (s *DispatchStep) StepType() byte { return StepTypeDispatch }

// ThreadCallPossibleStep (type 14)
type ThreadCallPossibleStep struct {
	BaseStep
	TxID     int64
	Hash     int32
	Elapsed  int32
	Threaded byte
}

func (s *ThreadCallPossibleStep) StepType() byte { return StepTypeThreadCallPossible }

// DumpStep represents a thread dump (type 12)
type DumpStep struct {
	BaseStep
	Stacks        []int32
	ThreadID      int64
	ThreadName    string
	ThreadState   string
	LockOwnerID   int64
	LockName      string
	LockOwnerName string
}

func (s *DumpStep) StepType() byte { return StepTypeDump }

// ParameterizedMessageStep (type 17)
type ParameterizedMessageStep struct {
	BaseStep
	Hash        int32
	Elapsed     int32
	Level       int32
	ParamString string
}

func (s *ParameterizedMessageStep) StepType() byte { return StepTypeParameterizedMsg }

// MethodSum represents aggregated method stats (type 11) - StepSummary, no parent fields
type MethodSum struct {
	Hash    int32
	Count   int32
	Elapsed int64
	CPUTime int64
}

func (s *MethodSum) StepType() byte { return StepTypeMethodSum }

// SqlSum represents aggregated SQL stats (type 21)
type SqlSum struct {
	Hash       int32
	Count      int32
	Elapsed    int64
	CPUTime    int64
	Error      int32
	Param      string
	ParamError string
}

func (s *SqlSum) StepType() byte { return StepTypeSqlSum }

// ApiCallSum represents aggregated API call stats (type 43)
type ApiCallSum struct {
	Hash    int32
	Count   int32
	Elapsed int64
	CPUTime int64
	Error   int32
	Opt     byte
}

func (s *ApiCallSum) StepType() byte { return StepTypeApiCallSum }

// SocketSum represents aggregated socket stats (type 42)
type SocketSum struct {
	IPAddr  []byte
	Port    int32
	Count   int32
	Elapsed int64
	Error   int32
}

func (s *SocketSum) StepType() byte { return StepTypeSocketSum }

// MessageSum represents aggregated message stats (type 31)
type MessageSum struct {
	Hash    int32
	Count   int32
	Elapsed int64
	CPUTime int64
}

func (s *MessageSum) StepType() byte { return StepTypeMessageSum }

// StepControl represents a control message (type 99) - StepSummary, no parent fields
type StepControl struct {
	Message string
	Code    int32
}

func (s *StepControl) StepType() byte { return StepTypeControl }

// XLogProfilePack represents profile data for an XLog
type XLogProfilePack struct {
	Time    int64
	ObjHash int32
	Service int32
	TxID    int64
	Profile []byte
	Steps   []Step
}

func (p *XLogProfilePack) PackType() byte {
	return XLOG_PROFILE
}

func (p *XLogProfilePack) Write(out *io.DataOutputX) {
	out.WriteDecimal(p.Time)
	out.WriteDecimal(int64(p.ObjHash))
	out.WriteDecimal(int64(p.Service))
	out.WriteInt64(p.TxID)
	out.WriteBlob(p.Profile)
}

// ReadXLogProfilePack reads an XLogProfilePack from DataInputX
func ReadXLogProfilePack(in *io.DataInputX) (*XLogProfilePack, error) {
	p := &XLogProfilePack{}
	var err error

	time, err := in.ReadDecimal()
	if err != nil {
		return nil, err
	}
	p.Time = time

	objHash, err := in.ReadDecimal()
	if err != nil {
		return nil, err
	}
	p.ObjHash = int32(objHash)

	service, err := in.ReadDecimal()
	if err != nil {
		return nil, err
	}
	p.Service = int32(service)

	if p.TxID, err = in.ReadInt64(); err != nil {
		return nil, err
	}

	if p.Profile, err = in.ReadBlob(); err != nil {
		return nil, err
	}

	if len(p.Profile) > 0 {
		p.Steps, err = parseProfileSteps(p.Profile)
		if err != nil {
			p.Steps = nil
		}
	}

	return p, nil
}

// parseProfileSteps parses step records from a profile blob
func parseProfileSteps(data []byte) ([]Step, error) {
	in := io.NewDataInputX(data)
	var steps []Step

	for in.Remaining() > 0 {
		step, err := ReadStep(in)
		if err != nil {
			break
		}
		if step != nil {
			steps = append(steps, step)
		}
	}

	return steps, nil
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
	case StepTypeMethod2:
		return readMethod2Step(in)
	case StepTypeSql:
		return readSqlStep(in)
	case StepTypeSql2:
		return readSqlStep2(in)
	case StepTypeSql3:
		return readSqlStep3(in)
	case StepTypeMessage:
		return readMessageStep(in)
	case StepTypeSocket:
		return readSocketStep(in)
	case StepTypeApiCall:
		return readApiCallStep(in)
	case StepTypeApiCall2:
		return readApiCallStep2(in)
	case StepTypeThreadSubmit:
		return readThreadSubmitStep(in)
	case StepTypeHashedMessage:
		return readHashedMessageStep(in)
	case StepTypeMethodSum:
		return readMethodSum(in)
	case StepTypeDump:
		return readDumpStep(in)
	case StepTypeDispatch:
		return readDispatchStep(in)
	case StepTypeThreadCallPossible:
		return readThreadCallPossibleStep(in)
	case StepTypeParameterizedMsg:
		return readParameterizedMessageStep(in)
	case StepTypeSqlSum:
		return readSqlSum(in)
	case StepTypeMessageSum:
		return readMessageSum(in)
	case StepTypeSocketSum:
		return readSocketSum(in)
	case StepTypeApiCallSum:
		return readApiCallSum(in)
	case StepTypeControl:
		return readStepControl(in)
	default:
		// Unknown step type - cannot safely skip without knowing size
		return nil, fmt.Errorf("unknown step type: %d", t)
	}
}

func readMethodStep(in *io.DataInputX) (*MethodStep, error) {
	s := &MethodStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readMethod2Step(in *io.DataInputX) (*Method2Step, error) {
	s := &Method2Step{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSqlStep(in *io.DataInputX) (*SqlStep, error) {
	s := &SqlStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
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
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Param, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.XType, err = in.ReadByte(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSqlStep3(in *io.DataInputX) (*SqlStep3, error) {
	s := &SqlStep3{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Param, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.XType, err = in.ReadByte(); err != nil {
		return nil, err
	}
	if s.Updated, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readMessageStep(in *io.DataInputX) (*MessageStep, error) {
	s := &MessageStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Message, err = in.ReadText(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSocketStep(in *io.DataInputX) (*SocketStep, error) {
	s := &SocketStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.IPAddr, err = in.ReadBlob(); err != nil {
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

func readApiCallStep(in *io.DataInputX) (*ApiCallStep, error) {
	s := &ApiCallStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	var txid int64
	if txid, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	s.TxID = txid
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Opt, err = in.ReadByte(); err != nil {
		return nil, err
	}
	if s.Opt == 1 {
		if s.Address, err = in.ReadText(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func readApiCallStep2(in *io.DataInputX) (*ApiCallStep2, error) {
	s := &ApiCallStep2{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	var txid int64
	if txid, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	s.TxID = txid
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Opt, err = in.ReadByte(); err != nil {
		return nil, err
	}
	if s.Opt == 1 {
		if s.Address, err = in.ReadText(); err != nil {
			return nil, err
		}
	}
	if s.Async, err = in.ReadByte(); err != nil {
		return nil, err
	}
	return s, nil
}

func readThreadSubmitStep(in *io.DataInputX) (*ThreadSubmitStep, error) {
	s := &ThreadSubmitStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	var txid int64
	if txid, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	s.TxID = txid
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readHashedMessageStep(in *io.DataInputX) (*HashedMessageStep, error) {
	s := &HashedMessageStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Time, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Value, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readDumpStep(in *io.DataInputX) (*DumpStep, error) {
	s := &DumpStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	// Read int array (stacks)
	count, err := in.ReadDecimal32()
	if err != nil {
		return nil, err
	}
	s.Stacks = make([]int32, count)
	for i := int32(0); i < count; i++ {
		if s.Stacks[i], err = in.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if s.ThreadID, err = in.ReadInt64(); err != nil {
		return nil, err
	}
	if s.ThreadName, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.ThreadState, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.LockOwnerID, err = in.ReadInt64(); err != nil {
		return nil, err
	}
	if s.LockName, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.LockOwnerName, err = in.ReadText(); err != nil {
		return nil, err
	}
	return s, nil
}

func readDispatchStep(in *io.DataInputX) (*DispatchStep, error) {
	s := &DispatchStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	var txid int64
	if txid, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	s.TxID = txid
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Opt, err = in.ReadByte(); err != nil {
		return nil, err
	}
	if s.Opt == 1 {
		if s.Address, err = in.ReadText(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func readThreadCallPossibleStep(in *io.DataInputX) (*ThreadCallPossibleStep, error) {
	s := &ThreadCallPossibleStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	var txid int64
	if txid, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	s.TxID = txid
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Threaded, err = in.ReadByte(); err != nil {
		return nil, err
	}
	return s, nil
}

func readParameterizedMessageStep(in *io.DataInputX) (*ParameterizedMessageStep, error) {
	s := &ParameterizedMessageStep{}
	var err error
	if s.BaseStep, err = readBaseStep(in); err != nil {
		return nil, err
	}
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	level, err := in.ReadDecimal()
	if err != nil {
		return nil, err
	}
	s.Level = int32(level)
	if s.ParamString, err = in.ReadText(); err != nil {
		return nil, err
	}
	return s, nil
}

func readMethodSum(in *io.DataInputX) (*MethodSum, error) {
	s := &MethodSum{}
	var err error
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Count, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSqlSum(in *io.DataInputX) (*SqlSum, error) {
	s := &SqlSum{}
	var err error
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Count, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Param, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.ParamError, err = in.ReadText(); err != nil {
		return nil, err
	}
	return s, nil
}

func readMessageSum(in *io.DataInputX) (*MessageSum, error) {
	s := &MessageSum{}
	var err error
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Count, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	return s, nil
}

func readSocketSum(in *io.DataInputX) (*SocketSum, error) {
	s := &SocketSum{}
	var err error
	if s.IPAddr, err = in.ReadBlob(); err != nil {
		return nil, err
	}
	if s.Port, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Count, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.Error, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}

func readApiCallSum(in *io.DataInputX) (*ApiCallSum, error) {
	s := &ApiCallSum{}
	var err error
	if s.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Count, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if s.Elapsed, err = in.ReadDecimal(); err != nil {
		return nil, err
	}
	if s.CPUTime, err = in.ReadDecimal(); err != nil {
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

func readStepControl(in *io.DataInputX) (*StepControl, error) {
	s := &StepControl{}
	var err error
	if s.Message, err = in.ReadText(); err != nil {
		return nil, err
	}
	if s.Code, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	return s, nil
}
