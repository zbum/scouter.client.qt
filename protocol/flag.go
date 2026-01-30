package protocol

// TCP flag constants - matches Scouter NetCafe
const (
	// Response flags
	FlagOK             byte = 0x01 // OK, more data coming (deprecated)
	FlagHasNEXT        byte = 0x03 // Has more data
	FlagNoNEXT         byte = 0x04 // No more data (end of stream)
	FlagEOF            byte = 0x05 // End of file
	FlagError          byte = 0x06 // Error occurred
	FlagInvalidSession byte = 0x44 // Session is invalid (ASCII 'D')
)

// Connection type constants - NetCafe TCP types
const (
	// TCP_CLIENT = 0xCAFE2001 - client request a service to server
	TCPClient uint32 = 0xCAFE2001
)

// Default ports
const (
	DefaultCollectorPort = 6100
	DefaultHttpPort      = 6180
)

// Connection timeout defaults (milliseconds)
const (
	DefaultConnectTimeout = 5000
	DefaultReadTimeout    = 30000
	DefaultWriteTimeout   = 5000
)
