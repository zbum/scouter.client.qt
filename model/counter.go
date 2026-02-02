// Package model provides data models for Scouter client
package model

import (
	"sync"
	"time"
)

// CounterKey uniquely identifies a counter
type CounterKey struct {
	ObjHash int32
	Counter string
}

// CounterData holds counter value with timestamp
type CounterData struct {
	TimeStamp time.Time
	Value     float64
	ObjHash   int32
	ObjName   string
	Counter   string
}

// CounterSeries holds time-series data for a counter
type CounterSeries struct {
	Key       CounterKey
	ObjName   string
	Data      []CounterData
	MaxSize   int
	mu        sync.RWMutex
}

// NewCounterSeries creates a new counter series
func NewCounterSeries(objHash int32, counter string, maxSize int) *CounterSeries {
	return &CounterSeries{
		Key: CounterKey{
			ObjHash: objHash,
			Counter: counter,
		},
		MaxSize: maxSize,
		Data:    make([]CounterData, 0, maxSize),
	}
}

// Add adds a new data point
func (s *CounterSeries) Add(data CounterData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data = append(s.Data, data)
	if len(s.Data) > s.MaxSize {
		s.Data = s.Data[1:]
	}
	if s.ObjName == "" && data.ObjName != "" {
		s.ObjName = data.ObjName
	}
}

// GetLast returns the last n data points
func (s *CounterSeries) GetLast(n int) []CounterData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n >= len(s.Data) {
		result := make([]CounterData, len(s.Data))
		copy(result, s.Data)
		return result
	}

	result := make([]CounterData, n)
	copy(result, s.Data[len(s.Data)-n:])
	return result
}

// GetAfter returns data points after the given time
func (s *CounterSeries) GetAfter(after time.Time) []CounterData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CounterData
	for _, d := range s.Data {
		if d.TimeStamp.After(after) {
			result = append(result, d)
		}
	}
	return result
}

// Latest returns the latest data point
func (s *CounterSeries) Latest() *CounterData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Data) == 0 {
		return nil
	}
	latest := s.Data[len(s.Data)-1]
	return &latest
}

// Size returns the number of data points
func (s *CounterSeries) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Data)
}

// Clear removes all data points
func (s *CounterSeries) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data = s.Data[:0]
}

// CounterStore stores multiple counter series
type CounterStore struct {
	series  map[CounterKey]*CounterSeries
	maxSize int
	mu      sync.RWMutex
}

// NewCounterStore creates a new counter store
func NewCounterStore(maxSize int) *CounterStore {
	return &CounterStore{
		series:  make(map[CounterKey]*CounterSeries),
		maxSize: maxSize,
	}
}

// GetOrCreate gets or creates a counter series
func (s *CounterStore) GetOrCreate(objHash int32, counter string) *CounterSeries {
	key := CounterKey{ObjHash: objHash, Counter: counter}

	s.mu.RLock()
	series, ok := s.series[key]
	s.mu.RUnlock()

	if ok {
		return series
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check after acquiring write lock
	if series, ok := s.series[key]; ok {
		return series
	}

	series = NewCounterSeries(objHash, counter, s.maxSize)
	s.series[key] = series
	return series
}

// Get retrieves a counter series
func (s *CounterStore) Get(objHash int32, counter string) *CounterSeries {
	key := CounterKey{ObjHash: objHash, Counter: counter}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.series[key]
}

// Add adds a counter data point
func (s *CounterStore) Add(data CounterData) {
	series := s.GetOrCreate(data.ObjHash, data.Counter)
	series.Add(data)
}

// GetAllForCounter retrieves all series for a counter name
func (s *CounterStore) GetAllForCounter(counter string) []*CounterSeries {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*CounterSeries
	for key, series := range s.series {
		if key.Counter == counter {
			result = append(result, series)
		}
	}
	return result
}

// Clear removes all series
func (s *CounterStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.series = make(map[CounterKey]*CounterSeries)
}
