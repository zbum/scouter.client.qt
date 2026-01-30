package net

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// PoolConfig holds connection pool configuration
type PoolConfig struct {
	MaxSize       int           // Maximum pool size
	MinIdle       int           // Minimum idle connections
	MaxIdleTime   time.Duration // Maximum idle time before eviction
	CheckInterval time.Duration // Health check interval
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:       10,
		MinIdle:       2,
		MaxIdleTime:   5 * time.Minute,
		CheckInterval: 30 * time.Second,
	}
}

// pooledClient wraps a Client with pool metadata
type pooledClient struct {
	client     *Client
	lastUsed   time.Time
	createTime time.Time
}

// Pool manages a pool of connections to a server
type Pool struct {
	host   string
	port   int
	config PoolConfig

	mu      sync.Mutex
	idle    *list.List // List of *pooledClient
	active  int
	closed  bool
	closeCh chan struct{}
}

// NewPool creates a new connection pool
func NewPool(host string, port int, config PoolConfig) *Pool {
	p := &Pool{
		host:    host,
		port:    port,
		config:  config,
		idle:    list.New(),
		closeCh: make(chan struct{}),
	}

	// Start background maintenance
	go p.maintenance()

	return p
}

// Get retrieves a connection from the pool
func (p *Pool) Get() (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	// Try to get an idle connection
	for e := p.idle.Front(); e != nil; {
		pc := e.Value.(*pooledClient)
		next := e.Next()
		p.idle.Remove(e)
		e = next

		// Check if connection is still valid
		if pc.client.IsConnected() {
			p.active++
			return pc.client, nil
		}
		// Connection is dead, close it
		pc.client.Close()
	}

	// Check if we can create a new connection
	if p.active >= p.config.MaxSize {
		return nil, fmt.Errorf("pool exhausted")
	}

	// Create new connection
	client := NewClient(p.host, p.port)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	p.active++
	return client, nil
}

// Put returns a connection to the pool
func (p *Pool) Put(client *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		client.Close()
		return
	}

	p.active--

	// If connection is dead, don't return it
	if !client.IsConnected() {
		client.Close()
		return
	}

	// Check if pool is full
	if p.idle.Len() >= p.config.MaxSize {
		client.Close()
		return
	}

	// Return to pool
	p.idle.PushBack(&pooledClient{
		client:   client,
		lastUsed: time.Now(),
	})
}

// Close closes all connections and the pool
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	close(p.closeCh)

	// Close all idle connections
	for e := p.idle.Front(); e != nil; e = e.Next() {
		pc := e.Value.(*pooledClient)
		pc.client.Close()
	}
	p.idle.Init()
}

// Size returns current pool size
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.idle.Len()
}

// Active returns number of active connections
func (p *Pool) Active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// maintenance runs periodic maintenance tasks
func (p *Pool) maintenance() {
	ticker := time.NewTicker(p.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.closeCh:
			return
		case <-ticker.C:
			p.evictIdle()
			p.ensureMinIdle()
		}
	}
}

// evictIdle removes idle connections that exceeded max idle time
func (p *Pool) evictIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for e := p.idle.Front(); e != nil; {
		pc := e.Value.(*pooledClient)
		next := e.Next()

		if now.Sub(pc.lastUsed) > p.config.MaxIdleTime {
			p.idle.Remove(e)
			pc.client.Close()
		}

		e = next
	}
}

// ensureMinIdle ensures minimum idle connections
func (p *Pool) ensureMinIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.idle.Len() < p.config.MinIdle && !p.closed {
		client := NewClient(p.host, p.port)
		if err := client.Connect(); err != nil {
			break // Stop trying if connection fails
		}

		p.idle.PushBack(&pooledClient{
			client:     client,
			lastUsed:   time.Now(),
			createTime: time.Now(),
		})
	}
}
