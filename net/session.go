package net

import (
	"sync"
	"time"

	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/pack"
)

// Session represents a connection session to a Scouter server
type Session struct {
	proxy     *Proxy
	serverID  int
	host      string
	port      int
	connected bool
	lastCheck time.Time

	// Credentials for re-login
	userID   string
	password string

	// Server info
	version    string
	serverTime int64
	timezone   string

	// Session observer
	stopObserver chan struct{}

	mu sync.RWMutex
}

// NewSession creates a new session
func NewSession(serverID int, host string, port int) *Session {
	return &Session{
		serverID: serverID,
		host:     host,
		port:     port,
		proxy:    NewProxy(host, port),
	}
}

// ServerID returns the server ID
func (s *Session) ServerID() int {
	return s.serverID
}

// Host returns the server host
func (s *Session) Host() string {
	return s.host
}

// Port returns the server port
func (s *Session) Port() int {
	return s.port
}

// Version returns the server version
func (s *Session) Version() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// IsConnected returns connection status
func (s *Session) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// Connect establishes a session with the server
func (s *Session) Connect(userID, password string) error {
	s.mu.Lock()
	s.userID = userID
	s.password = password
	s.mu.Unlock()

	s.proxy.SetCredentials(userID, password)

	if err := s.proxy.Login(); err != nil {
		return err
	}

	s.mu.Lock()
	s.connected = true
	s.lastCheck = time.Now()
	// Server info comes from login response (like Java client)
	s.version = s.proxy.ServerVersion()
	s.serverTime = s.proxy.ServerTime()
	s.timezone = s.proxy.ServerTimezone()
	s.mu.Unlock()

	// Start session observer for automatic re-login
	s.startObserver()

	return nil
}

// Disconnect closes the session
func (s *Session) Disconnect() {
	// Stop observer
	s.stopObserverIfRunning()

	s.mu.Lock()
	s.connected = false
	s.mu.Unlock()

	s.proxy.Close()
}

// startObserver starts the background session observer
func (s *Session) startObserver() {
	s.mu.Lock()
	if s.stopObserver != nil {
		s.mu.Unlock()
		return // Already running
	}
	s.stopObserver = make(chan struct{})
	stop := s.stopObserver
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.checkAndRelogin()
			}
		}
	}()
}

// stopObserverIfRunning stops the observer if it's running
func (s *Session) stopObserverIfRunning() {
	s.mu.Lock()
	if s.stopObserver != nil {
		close(s.stopObserver)
		s.stopObserver = nil
	}
	s.mu.Unlock()
}

// checkAndRelogin checks if session needs re-login and performs it
func (s *Session) checkAndRelogin() {
	sessionID := s.proxy.SessionID()
	if sessionID == 0 {
		s.mu.RLock()
		hasCredentials := s.userID != ""
		s.mu.RUnlock()

		if !hasCredentials {
			return // No credentials
		}

		// Try to re-login (proxy has credentials from initial connect)
		if err := s.proxy.Login(); err != nil {
			// Login failed, will retry next cycle
			return
		}

		s.mu.Lock()
		s.connected = true
		s.lastCheck = time.Now()
		s.mu.Unlock()
	}
}

// Proxy returns the underlying proxy
func (s *Session) Proxy() *Proxy {
	return s.proxy
}

// CheckConnection checks if the session is still valid
func (s *Session) CheckConnection() bool {
	_, err := s.proxy.Request(protocol.CMD_CHECK_SESSION, nil)
	if err != nil {
		s.mu.Lock()
		s.connected = false
		s.mu.Unlock()
		return false
	}

	s.mu.Lock()
	s.lastCheck = time.Now()
	s.mu.Unlock()
	return true
}

// Request sends a request through this session
func (s *Session) Request(cmd string, param *pack.MapPack) (*pack.MapPack, error) {
	return s.proxy.Request(cmd, param)
}

// RequestStream sends a streaming request through this session
func (s *Session) RequestStream(cmd string, param *pack.MapPack, callback func(pack.Pack) bool) error {
	return s.proxy.RequestStream(cmd, param, callback)
}

// GetObjectList retrieves the list of monitored objects
func (s *Session) GetObjectList() ([]*pack.ObjectPack, error) {
	return s.proxy.GetObjectList()
}
