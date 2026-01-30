package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"scouter.client.qt/protocol/pack"
)

// Manager manages multiple Scouter servers
type Manager struct {
	servers map[int]*Server
	nextID  int

	mu sync.RWMutex
}

var (
	managerInstance *Manager
	managerOnce     sync.Once
)

// GetManager returns the singleton Manager instance
func GetManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			servers: make(map[int]*Server),
			nextID:  1,
		}
		managerInstance.load()
	})
	return managerInstance
}

// AddServer adds a new server
func (m *Manager) AddServer(name, host string, port int, userID, password string) *Server {
	m.mu.Lock()
	defer m.mu.Unlock()

	server := NewServer(m.nextID, name, host, port)
	server.SetCredentials(userID, password)
	m.servers[m.nextID] = server
	m.nextID++

	go m.save()
	return server
}

// RemoveServer removes a server by ID
func (m *Manager) RemoveServer(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if server, ok := m.servers[id]; ok {
		server.Disconnect()
		delete(m.servers, id)
		go m.save()
	}
}

// GetServer retrieves a server by ID
func (m *Manager) GetServer(id int) *Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[id]
}

// GetServers returns all servers
func (m *Manager) GetServers() []*Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	return servers
}

// GetConnectedServers returns connected servers
func (m *Manager) GetConnectedServers() []*Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]*Server, 0)
	for _, s := range m.servers {
		if s.IsConnected() {
			servers = append(servers, s)
		}
	}
	return servers
}

// ConnectAll connects to all servers with AutoConnect enabled
func (m *Manager) ConnectAll() error {
	servers := m.GetServers()
	var lastErr error

	for _, server := range servers {
		if server.AutoConnect {
			if err := server.Connect(); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

// DisconnectAll disconnects from all servers
func (m *Manager) DisconnectAll() {
	servers := m.GetServers()
	for _, server := range servers {
		server.Disconnect()
	}
}

// GetAllObjects retrieves objects from all connected servers
func (m *Manager) GetAllObjects() []*pack.ObjectPack {
	servers := m.GetConnectedServers()
	var allObjects []*pack.ObjectPack

	for _, server := range servers {
		if server.Session() != nil {
			objects, err := server.Session().GetObjectList()
			if err == nil {
				allObjects = append(allObjects, objects...)
			}
		}
	}

	return allObjects
}

// serverConfig is used for JSON serialization
type serverConfig struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	UserID      string `json:"userId"`
	Password    string `json:"password"`
	AutoConnect bool   `json:"autoConnect"`
}

type managerConfig struct {
	Servers []serverConfig `json:"servers"`
	NextID  int            `json:"nextId"`
}

func (m *Manager) configPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "scouter.client.go", "servers.json")
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.configPath())
	if err != nil {
		return
	}

	var config managerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	m.nextID = config.NextID
	for _, sc := range config.Servers {
		server := NewServer(sc.ID, sc.Name, sc.Host, sc.Port)
		server.SetCredentials(sc.UserID, sc.Password)
		server.AutoConnect = sc.AutoConnect
		m.servers[sc.ID] = server

		if sc.ID >= m.nextID {
			m.nextID = sc.ID + 1
		}
	}
}

func (m *Manager) save() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := managerConfig{
		NextID:  m.nextID,
		Servers: make([]serverConfig, 0, len(m.servers)),
	}

	for _, s := range m.servers {
		config.Servers = append(config.Servers, serverConfig{
			ID:          s.ID,
			Name:        s.Name,
			Host:        s.Host,
			Port:        s.Port,
			UserID:      s.UserID,
			Password:    s.Password,
			AutoConnect: s.AutoConnect,
		})
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	os.WriteFile(m.configPath(), data, 0600)
}

// UpdateServer updates server information
func (m *Manager) UpdateServer(id int, name, host string, port int, userID, password string, autoConnect bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.servers[id]
	if !ok {
		return fmt.Errorf("server not found: %d", id)
	}

	// Disconnect if host/port changed
	if server.Host != host || server.Port != port {
		server.Disconnect()
	}

	server.Name = name
	server.Host = host
	server.Port = port
	server.UserID = userID
	server.Password = password
	server.AutoConnect = autoConnect

	go m.save()
	return nil
}
