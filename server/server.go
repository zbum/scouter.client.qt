// Package server provides server management for Scouter client
package server

import (
	"scouter.client.qt/net"
	"scouter.client.qt/protocol/pack"
)

// Server represents a Scouter collector server
type Server struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	UserID   string `json:"userId"`
	Password string `json:"password"`
	AutoConnect bool `json:"autoConnect"`

	session *net.Session
}

// NewServer creates a new Server
func NewServer(id int, name, host string, port int) *Server {
	return &Server{
		ID:   id,
		Name: name,
		Host: host,
		Port: port,
	}
}

// SetCredentials sets login credentials
func (s *Server) SetCredentials(userID, password string) {
	s.UserID = userID
	s.Password = password
}

// Connect connects to the server
func (s *Server) Connect() error {
	if s.session != nil && s.session.IsConnected() {
		return nil
	}

	s.session = net.NewSession(s.ID, s.Host, s.Port)
	return s.session.Connect(s.UserID, s.Password)
}

// Disconnect disconnects from the server
func (s *Server) Disconnect() {
	if s.session != nil {
		s.session.Disconnect()
		s.session = nil
	}
}

// IsConnected returns connection status
func (s *Server) IsConnected() bool {
	return s.session != nil && s.session.IsConnected()
}

// Session returns the current session
func (s *Server) Session() *net.Session {
	return s.session
}

// Version returns the server version
func (s *Server) Version() string {
	if s.session != nil {
		return s.session.Version()
	}
	return ""
}

// DisplayName returns a display name for the server
func (s *Server) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.Host
}

// GetObjects retrieves all objects from this server
func (s *Server) GetObjects() []*pack.ObjectPack {
	if s.session == nil || !s.session.IsConnected() {
		return nil
	}
	objects, _ := s.session.GetObjectList()
	return objects
}
