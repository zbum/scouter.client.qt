// Package net provides TCP client for Scouter server communication
package net

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
)

var (
	ErrNotConnected   = errors.New("not connected")
	ErrInvalidSession = errors.New("invalid session")
)

// Client represents a TCP connection to Scouter server
type Client struct {
	host string
	port int
	conn net.Conn

	connectTimeout time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration

	mu        sync.Mutex
	connected bool
}

// NewClient creates a new TCP client
func NewClient(host string, port int) *Client {
	return &Client{
		host:           host,
		port:           port,
		connectTimeout: time.Duration(protocol.DefaultConnectTimeout) * time.Millisecond,
		readTimeout:    time.Duration(protocol.DefaultReadTimeout) * time.Millisecond,
		writeTimeout:   time.Duration(protocol.DefaultWriteTimeout) * time.Millisecond,
	}
}

// SetConnectTimeout sets the connection timeout
func (c *Client) SetConnectTimeout(timeout time.Duration) {
	c.connectTimeout = timeout
}

// SetReadTimeout sets the read timeout
func (c *Client) SetReadTimeout(timeout time.Duration) {
	c.readTimeout = timeout
}

// SetWriteTimeout sets the write timeout
func (c *Client) SetWriteTimeout(timeout time.Duration) {
	c.writeTimeout = timeout
}

// Connect establishes a TCP connection to the server
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected && c.conn != nil {
		return nil
	}

	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	conn, err := net.DialTimeout("tcp", addr, c.connectTimeout)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	c.conn = conn

	// Send Scouter protocol handshake
	if err := c.sendHandshake(); err != nil {
		c.conn.Close()
		c.conn = nil
		return fmt.Errorf("handshake failed: %w", err)
	}

	c.connected = true
	return nil
}

// sendHandshake sends the Scouter protocol handshake

func (c *Client) sendHandshake() error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return err
	}

	// Write TCP_CLIENT (0xCAFE2001) as 4-byte big-endian
	handshake := make([]byte, 4)
	handshake[0] = 0xCA
	handshake[1] = 0xFE
	handshake[2] = 0x20
	handshake[3] = 0x01

	if _, err := c.conn.Write(handshake); err != nil {
		return err
	}

	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.connected = false
		return err
	}
	return nil
}

// IsConnected returns connection status
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// SendRequest sends a request and returns response
func (c *Client) SendRequest(cmd string, param *pack.MapPack, sessionID int64) (*pack.MapPack, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil, ErrNotConnected
	}

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return nil, err
	}

	// Build and send request (no length header - Scouter protocol)
	// Format: command(text) + session(long) + param(pack)
	reqData := io.NewDataOutputX()
	reqData.WriteText(cmd)
	reqData.WriteInt64(sessionID)
	if param != nil {
		pack.WritePack(reqData, param)
	}

	if _, err := c.conn.Write(reqData.Bytes()); err != nil {
		c.handleConnectionError()
		return nil, err
	}

	// Read response
	var result *pack.MapPack
	for {
		flag, p, err := c.readResponse()
		if err != nil {
			c.handleConnectionError()
			return nil, err
		}

		// Check response flag
		if flag == protocol.FlagInvalidSession {
			c.handleConnectionError()
			return nil, ErrInvalidSession
		}
		if flag == protocol.FlagError {
			return nil, fmt.Errorf("server error")
		}

		// Store the pack if we got one
		if p != nil {
			if mp, ok := p.(*pack.MapPack); ok {
				result = mp
			}
		}

		// Check if there's more data
		if flag != protocol.FlagHasNEXT {
			break
		}
	}

	if result == nil {
		return pack.NewMapPack(), nil
	}
	return result, nil
}

// SendRequestStream sends a request and returns streaming responses
// The callback is called for each received pack until the stream ends
func (c *Client) SendRequestStream(cmd string, param *pack.MapPack, sessionID int64, callback func(pack.Pack) bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return ErrNotConnected
	}

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return err
	}

	// Build and send request (no length header - Scouter protocol)
	reqData := io.NewDataOutputX()
	reqData.WriteText(cmd)
	reqData.WriteInt64(sessionID)
	if param != nil {
		pack.WritePack(reqData, param)
	}

	if _, err := c.conn.Write(reqData.Bytes()); err != nil {
		c.handleConnectionError()
		return err
	}

	// Read streaming responses
	for {
		flag, p, err := c.readResponse()
		if err != nil {
			c.handleConnectionError()
			return err
		}

		// Check flags
		if flag == protocol.FlagInvalidSession {
			c.handleConnectionError()
			return ErrInvalidSession
		}
		if flag == protocol.FlagError {
			return fmt.Errorf("server error")
		}
		if flag == protocol.FlagNoNEXT || flag == protocol.FlagEOF {
			break
		}

		// Deliver pack to callback
		if p != nil {
			if !callback(p) {
				break // Callback requested stop
			}
		}

		// Check if more data is coming
		if flag != protocol.FlagHasNEXT {
			break
		}
	}

	return nil
}

// readResponse reads a flag byte and pack from the connection
func (c *Client) readResponse() (byte, pack.Pack, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
		return 0, nil, err
	}

	// Read flag (1 byte)
	flagBuf := make([]byte, 1)
	if _, err := readFull(c.conn, flagBuf); err != nil {
		return 0, nil, err
	}
	flag := flagBuf[0]

	// If HasNEXT, read the pack directly from stream
	if flag == protocol.FlagHasNEXT {
		p, err := c.readPackFromStream()
		if err != nil {
			return flag, nil, err
		}
		return flag, p, nil
	}

	return flag, nil, nil
}

// readPackFromStream reads a pack directly from the connection stream
func (c *Client) readPackFromStream() (pack.Pack, error) {
	// Create a streaming DataInputX that reads from the connection
	in, err := io.NewDataInputXFromReader(c.conn)
	if err != nil {
		return nil, err
	}

	return pack.ReadPack(in)
}

func (c *Client) handleConnectionError() {
	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// readFull reads exactly len(buf) bytes
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// Host returns the server host
func (c *Client) Host() string {
	return c.host
}

// Port returns the server port
func (c *Client) Port() int {
	return c.port
}

// Address returns the server address
func (c *Client) Address() string {
	return fmt.Sprintf("%s:%d", c.host, c.port)
}
