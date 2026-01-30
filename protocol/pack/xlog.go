package pack

import (
	"scouter.client.qt/protocol/io"
)

// XLogPack represents a transaction log (XLog)
// Field order matches Java scouter.lang.pack.XLogPack exactly
type XLogPack struct {
	EndTime        int64  // Transaction end time (ms)
	ObjHash        int32  // Object(agent) hash
	Service        int32  // Service hash
	TxID           int64  // Transaction ID
	Caller         int64  // Caller transaction ID
	GxID           int64  // Global transaction ID
	Elapsed        int32  // Response time (ms)
	Error          int32  // Error hash (0 if no error)
	CPU            int32  // CPU time (ms)
	SQLCount       int32  // SQL call count
	SQLTime        int32  // SQL total time (ms)
	IPAddr         []byte // Client IP address (blob)
	KBytes         int32  // Traffic KB
	Status         int32  // HTTP status code
	UserID         int64  // User ID
	UserAgent      int32  // User agent hash
	Referer        int32  // Referer hash
	Group          int32  // Group hash
	APICallCount   int32  // API call count
	APICallTime    int32  // API call total time (ms)
	CountryCode    string // Country code (text)
	City           int32  // City hash
	XType          byte   // XLog type (WebService:0, AppService:1, BackgroundThread:2)
	Login          int32  // Login hash
	Desc           int32  // Description hash
	WebHash        int32  // Web server object hash
	WebTime        int32  // Web server -> WAS time (ms)
	HasDump        byte   // Profile dump flag
	ThreadName     int32  // Thread name hash
	Text1          string // Additional text 1
	Text2          string // Additional text 2
	QueuingHostHash    int32 // Queuing host hash
	QueuingTime        int32 // Queuing time
	Queuing2ndHostHash int32 // Queuing 2nd host hash
	Queuing2ndTime     int32 // Queuing 2nd time
	Text3          string // Additional text 3
	Text4          string // Additional text 4
	Text5          string // Additional text 5
	ProfileCount   int32  // Profile count
	B3Mode         bool   // B3 tracing mode
	ProfileSize    int32  // Profile size
	DiscardType    byte   // Discard type
	IgnoreGlobalConsequentSampling bool
}

func (p *XLogPack) PackType() byte {
	return XLOG
}

func (p *XLogPack) Write(out *io.DataOutputX) {
	// Java XLogPack wraps all fields in a blob
	o := io.NewDataOutputX()

	o.WriteDecimal(p.EndTime)
	o.WriteDecimal(int64(p.ObjHash))
	o.WriteDecimal(int64(p.Service))
	o.WriteInt64(p.TxID)
	o.WriteInt64(p.Caller)
	o.WriteInt64(p.GxID)
	o.WriteDecimal(int64(p.Elapsed))
	o.WriteDecimal(int64(p.Error))
	o.WriteDecimal(int64(p.CPU))
	o.WriteDecimal(int64(p.SQLCount))
	o.WriteDecimal(int64(p.SQLTime))
	o.WriteBlob(p.IPAddr)
	o.WriteDecimal(int64(p.KBytes))
	o.WriteDecimal(int64(p.Status))
	o.WriteDecimal(p.UserID)
	o.WriteDecimal(int64(p.UserAgent))
	o.WriteDecimal(int64(p.Referer))
	o.WriteDecimal(int64(p.Group))
	o.WriteDecimal(int64(p.APICallCount))
	o.WriteDecimal(int64(p.APICallTime))
	o.WriteText(p.CountryCode)
	o.WriteDecimal(int64(p.City))
	o.WriteByte(p.XType)
	o.WriteDecimal(int64(p.Login))
	o.WriteDecimal(int64(p.Desc))
	o.WriteDecimal(int64(p.WebHash))
	o.WriteDecimal(int64(p.WebTime))
	o.WriteByte(p.HasDump)
	o.WriteDecimal(int64(p.ThreadName))
	o.WriteText(p.Text1)
	o.WriteText(p.Text2)
	o.WriteDecimal(int64(p.QueuingHostHash))
	o.WriteDecimal(int64(p.QueuingTime))
	o.WriteDecimal(int64(p.Queuing2ndHostHash))
	o.WriteDecimal(int64(p.Queuing2ndTime))
	o.WriteText(p.Text3)
	o.WriteText(p.Text4)
	o.WriteText(p.Text5)
	o.WriteDecimal(int64(p.ProfileCount))
	o.WriteBoolean(p.B3Mode)
	o.WriteDecimal(int64(p.ProfileSize))
	o.WriteByte(p.DiscardType)
	o.WriteBoolean(p.IgnoreGlobalConsequentSampling)

	out.WriteBlob(o.Bytes())
}

// ReadXLogPack reads an XLogPack from DataInputX
// Matches Java XLogPack.read() - reads a blob first, then parses fields
func ReadXLogPack(in *io.DataInputX) (*XLogPack, error) {
	// Java XLogPack.read() reads all fields from a blob
	blobData, err := in.ReadBlob()
	if err != nil {
		return nil, err
	}

	d := io.NewDataInputX(blobData)
	p := &XLogPack{}

	if p.EndTime, err = d.ReadDecimal(); err != nil {
		return nil, err
	}
	if p.ObjHash, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Service, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.TxID, err = d.ReadInt64(); err != nil {
		return nil, err
	}
	if p.Caller, err = d.ReadInt64(); err != nil {
		return nil, err
	}
	if p.GxID, err = d.ReadInt64(); err != nil {
		return nil, err
	}
	if p.Elapsed, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Error, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.CPU, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.SQLCount, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.SQLTime, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.IPAddr, err = d.ReadBlob(); err != nil {
		return nil, err
	}
	if p.KBytes, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Status, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.UserID, err = d.ReadDecimal(); err != nil {
		return nil, err
	}
	if p.UserAgent, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Referer, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Group, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.APICallCount, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.APICallTime, err = d.ReadDecimal32(); err != nil {
		return nil, err
	}

	// Optional fields - check remaining bytes like Java's d.available() > 0
	if d.Remaining() > 0 {
		if p.CountryCode, err = d.ReadText(); err != nil {
			return nil, err
		}
		if p.City, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.XType, err = d.ReadByte(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.Login, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.Desc, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.WebHash, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.WebTime, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.HasDump, err = d.ReadByte(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.ThreadName, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.Text1, err = d.ReadText(); err != nil {
			return nil, err
		}
		if p.Text2, err = d.ReadText(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.QueuingHostHash, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.QueuingTime, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.Queuing2ndHostHash, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.Queuing2ndTime, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.Text3, err = d.ReadText(); err != nil {
			return nil, err
		}
		if p.Text4, err = d.ReadText(); err != nil {
			return nil, err
		}
		if p.Text5, err = d.ReadText(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.ProfileCount, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.B3Mode, err = d.ReadBoolean(); err != nil {
			return nil, err
		}
	}
	if d.Remaining() > 0 {
		if p.ProfileSize, err = d.ReadDecimal32(); err != nil {
			return nil, err
		}
		if p.DiscardType, err = d.ReadByte(); err != nil {
			return nil, err
		}
		var ignore bool
		if ignore, err = d.ReadBoolean(); err != nil {
			return nil, err
		}
		p.IgnoreGlobalConsequentSampling = ignore
	}

	return p, nil
}

// IsError returns true if this transaction has an error
func (p *XLogPack) IsError() bool {
	return p.Error != 0
}

// IPString returns the client IP as a string
func (p *XLogPack) IPString() string {
	if len(p.IPAddr) < 4 {
		return ""
	}
	return io.NewIPValue(p.IPAddr).String()
}
