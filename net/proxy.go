package net

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"

	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/pack"
)

// Proxy provides a high-level interface for Scouter server communication.
// It manages sessions and creates per-request TCP connections, matching
// the Java client's connection pool behavior where each request gets
// its own connection. This avoids the server's 8-second idle timeout
// (net_tcp_client_so_timeout_ms) that kills persistent connections.
type Proxy struct {
	sessionID int64
	userID    string
	password  string
	host      string
	port      int

	// Server info from login response
	serverVersion  string
	serverTime     int64
	serverTimezone string

	mu sync.RWMutex
}

// NewProxy creates a new Proxy
func NewProxy(host string, port int) *Proxy {
	return &Proxy{
		host: host,
		port: port,
	}
}

// NewProxyWithConfig creates a new Proxy with custom pool config
func NewProxyWithConfig(host string, port int, config PoolConfig) *Proxy {
	return &Proxy{
		host: host,
		port: port,
	}
}

// Close closes the proxy
func (p *Proxy) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessionID = 0
}

// newConnection creates a fresh TCP connection with handshake
func (p *Proxy) newConnection() (*Client, error) {
	client := NewClient(p.host, p.port)
	if err := client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}

// SetCredentials sets login credentials
func (p *Proxy) SetCredentials(userID, password string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userID = userID
	p.password = password
}

// SessionID returns current session ID
func (p *Proxy) SessionID() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionID
}

// ServerVersion returns the server version from login response
func (p *Proxy) ServerVersion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serverVersion
}

// ServerTime returns the server time from login response
func (p *Proxy) ServerTime() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serverTime
}

// ServerTimezone returns the server timezone from login response
func (p *Proxy) ServerTimezone() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serverTimezone
}

// Login performs authentication with the server.
// Creates a temporary connection for the login exchange.
func (p *Proxy) Login() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, err := p.newConnection()
	if err != nil {
		return err
	}
	defer client.Close()

	// Hash password with SHA256 (same as Java client)
	hashedPassword := sha256Hash(p.password)

	// Get hostname
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Get local IP address
	localIP := getLocalIP()

	param := pack.NewMapPack()
	param.PutText(protocol.ParamLogin, p.userID)
	param.PutText(protocol.ParamPassword, hashedPassword)
	param.PutText("version", "2.20.0") // Client version
	param.PutText("hostname", hostname)
	param.PutBoolean("isSocks", false)
	param.PutText("ip", localIP)

	// Session ID = 0 for login request
	resp, err := client.SendRequest(protocol.CMD_LOGIN, param, 0)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Check for error
	errMsg := resp.GetText("error")
	if errMsg != "" {
		return fmt.Errorf("login error: %s", errMsg)
	}

	// Extract session ID (long value)
	session := resp.GetDecimalLong("session")
	if session == 0 {
		return fmt.Errorf("no session received (keys: %v)", resp.Keys())
	}

	p.sessionID = session

	// Store server info from login response (like Java client)
	p.serverVersion = resp.GetText("version")
	p.serverTime = resp.GetDecimalLong("time")
	p.serverTimezone = resp.GetText("timezone")

	return nil
}

// sha256Hash returns SHA256 hash of the input string as hex
// Uses the same salt as Java Scouter client
func sha256Hash(s string) string {
	salt := "qwertyuiop!@#$%^&*()zxcvbnm,."
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// getLocalIP returns the local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// Request sends a request to the server with session management.
// Each call creates a fresh TCP connection (avoiding server idle timeout).
func (p *Proxy) Request(cmd string, param *pack.MapPack) (*pack.MapPack, error) {
	// Server handlers expect to read a MapPack even if empty
	if param == nil {
		param = pack.NewMapPack()
	}

	p.mu.RLock()
	sessionID := p.sessionID
	p.mu.RUnlock()

	if sessionID == 0 {
		return nil, ErrInvalidSession
	}

	client, err := p.newConnection()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.SendRequest(cmd, param, sessionID)
	if err != nil {
		if isInvalidSessionError(err) {
			p.mu.Lock()
			p.sessionID = 0
			p.mu.Unlock()
		}
		return nil, err
	}

	return resp, nil
}

// RequestStream sends a streaming request to the server.
// Each call creates a fresh TCP connection (avoiding server idle timeout).
func (p *Proxy) RequestStream(cmd string, param *pack.MapPack, callback func(pack.Pack) bool) error {
	p.mu.RLock()
	sessionID := p.sessionID
	p.mu.RUnlock()

	if sessionID == 0 {
		return ErrInvalidSession
	}

	client, err := p.newConnection()
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.SendRequestStream(cmd, param, sessionID, callback)
	if err != nil {
		if isInvalidSessionError(err) {
			p.mu.Lock()
			p.sessionID = 0
			p.mu.Unlock()
		}
		return err
	}

	return nil
}

// isInvalidSessionError checks if the error indicates an invalid session
func isInvalidSessionError(err error) bool {
	return err == ErrInvalidSession
}

// --- Convenience methods for common operations ---

// GetObjectList retrieves the list of monitored objects
func (p *Proxy) GetObjectList() ([]*pack.ObjectPack, error) {
	var objects []*pack.ObjectPack

	err := p.RequestStream(protocol.CMD_OBJECT_LIST_REAL_TIME, nil, func(pk pack.Pack) bool {
		if obj, ok := pk.(*pack.ObjectPack); ok {
			objects = append(objects, obj)
		}
		return true
	})

	if err != nil {
		return nil, err
	}

	return objects, nil
}

// GetPerfCounter retrieves real-time performance counters
func (p *Proxy) GetPerfCounter(objHash int32, counter string) (*pack.PerfCounterPack, error) {
	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, objHash)
	param.PutText(protocol.ParamCounter, counter)

	resp, err := p.Request(protocol.CMD_COUNTER_REAL_TIME, param)
	if err != nil {
		return nil, err
	}

	// The response might be a PerfCounterPack directly
	// or embedded in a MapPack - handle both cases
	_ = resp
	return nil, fmt.Errorf("not implemented")
}

// GetText retrieves text by hash
func (p *Proxy) GetText(textType string, hash int32) (string, error) {
	param := pack.NewMapPack()
	param.PutText(protocol.ParamTextType, textType)
	param.PutDecimal(protocol.ParamHashValue, hash)

	resp, err := p.Request(protocol.CMD_GET_TEXT, param)
	if err != nil {
		return "", err
	}

	return resp.GetText("text"), nil
}

// GetXLogByTxID retrieves XLog by transaction ID
func (p *Proxy) GetXLogByTxID(txID int64) (*pack.XLogPack, error) {
	param := pack.NewMapPack()
	param.PutDecimalLong(protocol.ParamTxID, txID)

	var xlog *pack.XLogPack
	err := p.RequestStream(protocol.CMD_XLOG_READ_BY_TXID, param, func(pk pack.Pack) bool {
		if x, ok := pk.(*pack.XLogPack); ok {
			xlog = x
			return false // Stop after first XLog
		}
		return true
	})

	if err != nil {
		return nil, err
	}

	return xlog, nil
}

// GetXLogProfile retrieves profile data for a transaction
func (p *Proxy) GetXLogProfile(txID int64) (*pack.XLogProfilePack, error) {
	param := pack.NewMapPack()
	param.PutDecimalLong(protocol.ParamTxID, txID)

	var profile *pack.XLogProfilePack
	err := p.RequestStream(protocol.CMD_XLOG_PROFILE, param, func(pk pack.Pack) bool {
		if pr, ok := pk.(*pack.XLogProfilePack); ok {
			profile = pr
			return false
		}
		return true
	})

	if err != nil {
		return nil, err
	}

	return profile, nil
}
