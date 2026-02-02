package model

import (
	"sync"
	"time"

	"scouter.client.qt/net"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// CounterCallback is called when new counter data arrives
type CounterCallback func(objHash int32, counter string, value float64, timestamp time.Time)

// Subscription represents a counter subscription
type Subscription struct {
	ID       int
	Counter  string
	ObjHash  int32 // 0 means all objects
	Callback CounterCallback
}

// CounterEngine manages real-time counter subscriptions
type CounterEngine struct {
	store         *CounterStore
	subscriptions map[int]*Subscription
	nextSubID     int

	stopCh chan struct{}
	wg     sync.WaitGroup

	mu sync.RWMutex
}

var (
	engineInstance *CounterEngine
	engineOnce     sync.Once
)

// GetCounterEngine returns the singleton CounterEngine instance
func GetCounterEngine() *CounterEngine {
	engineOnce.Do(func() {
		engineInstance = &CounterEngine{
			store:         NewCounterStore(300), // 5 minutes at 1-second intervals
			subscriptions: make(map[int]*Subscription),
			nextSubID:     1,
			stopCh:        make(chan struct{}),
		}
	})
	return engineInstance
}

// Start starts the counter engine
func (e *CounterEngine) Start() {
	e.wg.Add(1)
	go e.run()
}

// Stop stops the counter engine
func (e *CounterEngine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

// Subscribe subscribes to counter updates
// objHash=0 means subscribe to all objects
func (e *CounterEngine) Subscribe(counter string, objHash int32, callback CounterCallback) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	sub := &Subscription{
		ID:       e.nextSubID,
		Counter:  counter,
		ObjHash:  objHash,
		Callback: callback,
	}
	e.subscriptions[e.nextSubID] = sub
	e.nextSubID++

	return sub.ID
}

// Unsubscribe removes a subscription
func (e *CounterEngine) Unsubscribe(id int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.subscriptions, id)
}

// GetStore returns the counter store
func (e *CounterEngine) GetStore() *CounterStore {
	return e.store
}

// GetLatest returns the latest value for a counter
func (e *CounterEngine) GetLatest(objHash int32, counter string) *CounterData {
	series := e.store.Get(objHash, counter)
	if series == nil {
		return nil
	}
	return series.Latest()
}

// GetHistory returns historical data for a counter
func (e *CounterEngine) GetHistory(objHash int32, counter string, duration time.Duration) []CounterData {
	series := e.store.Get(objHash, counter)
	if series == nil {
		return nil
	}
	return series.GetAfter(time.Now().Add(-duration))
}

// run is the main loop for fetching counter data
func (e *CounterEngine) run() {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.fetchCounters()
		}
	}
}

// fetchCounters fetches counter data from all connected servers
func (e *CounterEngine) fetchCounters() {
	// Get unique counters to fetch
	counters := e.getSubscribedCounters()
	if len(counters) == 0 {
		return
	}

	// Get connected servers
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	// Fetch from each server
	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		for counter := range counters {
			e.fetchCounter(session, counter)
		}
	}
}

// getSubscribedCounters returns unique counter names from subscriptions
func (e *CounterEngine) getSubscribedCounters() map[string]bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	counters := make(map[string]bool)
	for _, sub := range e.subscriptions {
		counters[sub.Counter] = true
	}
	return counters
}

// fetchCounter fetches a specific counter from a session
func (e *CounterEngine) fetchCounter(session interface{}, counter string) {
	s, ok := session.(*net.Session)
	if !ok {
		return
	}

	param := pack.NewMapPack()
	param.PutText(protocol.ParamCounter, counter)

	s.RequestStream(protocol.CMD_COUNTER_REAL_TIME_ALL, param, func(p pack.Pack) bool {
		if pc, ok := p.(*pack.PerfCounterPack); ok {
			e.processCounterPack(pc, counter)
		}
		return true
	})
}

// processCounterPack processes received counter pack
func (e *CounterEngine) processCounterPack(pc *pack.PerfCounterPack, counter string) {
	timestamp := time.UnixMilli(pc.TimeStamp)
	val := pc.Data.Get(counter)
	if val == nil {
		return
	}

	var value float64
	switch v := val.(type) {
	case *io.FloatValue:
		value = float64(v.Value)
	case *io.DoubleValue:
		value = v.Value
	case *io.DecimalValue:
		value = float64(v.Value)
	case *io.DecimalLongValue:
		value = float64(v.Value)
	default:
		return
	}

	// Store the data
	data := CounterData{
		TimeStamp: timestamp,
		Value:     value,
		ObjHash:   pc.ObjHash,
		ObjName:   pc.ObjName,
		Counter:   counter,
	}
	e.store.Add(data)

	// Notify subscribers
	e.notifySubscribers(pc.ObjHash, counter, value, timestamp)
}

// notifySubscribers notifies relevant subscribers
func (e *CounterEngine) notifySubscribers(objHash int32, counter string, value float64, timestamp time.Time) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sub := range e.subscriptions {
		if sub.Counter == counter && (sub.ObjHash == 0 || sub.ObjHash == objHash) {
			if sub.Callback != nil {
				sub.Callback(objHash, counter, value, timestamp)
			}
		}
	}
}
