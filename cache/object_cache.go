package cache

import (
	"sync"
	"time"

	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// ObjectCache caches agent object information
type ObjectCache struct {
	objects    map[int32]*pack.ObjectPack // objHash -> ObjectPack
	byName     map[string]*pack.ObjectPack // objName -> ObjectPack
	byType     map[string][]*pack.ObjectPack // objType -> []ObjectPack
	lastUpdate time.Time
	updateInterval time.Duration
	mu         sync.RWMutex
}

var (
	objectCacheInstance *ObjectCache
	objectCacheOnce     sync.Once
)

// GetObjectCache returns the singleton ObjectCache instance
func GetObjectCache() *ObjectCache {
	objectCacheOnce.Do(func() {
		objectCacheInstance = &ObjectCache{
			objects:        make(map[int32]*pack.ObjectPack),
			byName:         make(map[string]*pack.ObjectPack),
			byType:         make(map[string][]*pack.ObjectPack),
			updateInterval: 5 * time.Second,
		}
	})
	return objectCacheInstance
}

// Get retrieves an object by hash
func (c *ObjectCache) Get(objHash int32) *pack.ObjectPack {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.objects[objHash]
}

// GetByName retrieves an object by name
func (c *ObjectCache) GetByName(objName string) *pack.ObjectPack {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byName[objName]
}

// GetByType retrieves objects by type
func (c *ObjectCache) GetByType(objType string) []*pack.ObjectPack {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byType[objType]
}

// GetAll retrieves all objects
func (c *ObjectCache) GetAll() []*pack.ObjectPack {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*pack.ObjectPack, 0, len(c.objects))
	for _, obj := range c.objects {
		result = append(result, obj)
	}
	return result
}

// GetAlive retrieves all alive objects
func (c *ObjectCache) GetAlive() []*pack.ObjectPack {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*pack.ObjectPack
	for _, obj := range c.objects {
		if obj.Alive {
			result = append(result, obj)
		}
	}
	return result
}

// Update updates the cache with objects from server
func (c *ObjectCache) Update(objects []*pack.ObjectPack) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing data
	c.objects = make(map[int32]*pack.ObjectPack)
	c.byName = make(map[string]*pack.ObjectPack)
	c.byType = make(map[string][]*pack.ObjectPack)

	// Populate with new data
	for _, obj := range objects {
		c.objects[obj.ObjHash] = obj
		c.byName[obj.ObjName] = obj

		if _, ok := c.byType[obj.ObjType]; !ok {
			c.byType[obj.ObjType] = make([]*pack.ObjectPack, 0)
		}
		c.byType[obj.ObjType] = append(c.byType[obj.ObjType], obj)
	}

	c.lastUpdate = time.Now()
}

// Refresh refreshes the cache from servers
func (c *ObjectCache) Refresh() error {
	objects := server.GetManager().GetAllObjects()
	c.Update(objects)
	return nil
}

// RefreshIfNeeded refreshes the cache if the interval has passed
func (c *ObjectCache) RefreshIfNeeded() {
	c.mu.RLock()
	needsUpdate := time.Since(c.lastUpdate) > c.updateInterval
	c.mu.RUnlock()

	if needsUpdate {
		c.Refresh()
	}
}

// SetUpdateInterval sets the auto-refresh interval
func (c *ObjectCache) SetUpdateInterval(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateInterval = interval
}

// Size returns the number of cached objects
func (c *ObjectCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.objects)
}

// GetTypes returns all unique object types
func (c *ObjectCache) GetTypes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	types := make([]string, 0, len(c.byType))
	for t := range c.byType {
		types = append(types, t)
	}
	return types
}

// Clear clears the cache
func (c *ObjectCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.objects = make(map[int32]*pack.ObjectPack)
	c.byName = make(map[string]*pack.ObjectPack)
	c.byType = make(map[string][]*pack.ObjectPack)
}

// GetObjName returns the object name for a hash, fetching if needed
func (c *ObjectCache) GetObjName(objHash int32) string {
	c.RefreshIfNeeded()

	if obj := c.Get(objHash); obj != nil {
		return obj.ObjName
	}
	return ""
}

// GetObjType returns the object type for a hash
func (c *ObjectCache) GetObjType(objHash int32) string {
	c.RefreshIfNeeded()

	if obj := c.Get(objHash); obj != nil {
		return obj.ObjType
	}
	return ""
}

// GetName is an alias for GetObjName for convenience
func (c *ObjectCache) GetName(objHash int32) string {
	return c.GetObjName(objHash)
}
