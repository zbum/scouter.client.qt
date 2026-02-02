// Package cache provides caching for Scouter client
package cache

import (
	"sync"
	"time"

	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// TextEntry represents a cached text entry
type TextEntry struct {
	Text      string
	Timestamp time.Time
	Negative  bool // true if we looked up this hash and got no result
}

// TextCache caches hash-to-text mappings
type TextCache struct {
	cache map[string]map[int32]TextEntry // textType -> hash -> entry
	ttl   time.Duration
	mu    sync.RWMutex
}

var (
	textCacheInstance *TextCache
	textCacheOnce     sync.Once
)

// GetTextCache returns the singleton TextCache instance
func GetTextCache() *TextCache {
	textCacheOnce.Do(func() {
		textCacheInstance = &TextCache{
			cache: make(map[string]map[int32]TextEntry),
			ttl:   30 * time.Minute,
		}
	})
	return textCacheInstance
}

// Get retrieves text from cache
func (c *TextCache) Get(textType string, hash int32) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	typeCache, ok := c.cache[textType]
	if !ok {
		return "", false
	}

	entry, ok := typeCache[hash]
	if !ok {
		return "", false
	}

	// Negative cache entries expire faster (5 min) to allow re-lookup
	ttl := c.ttl
	if entry.Negative {
		ttl = 5 * time.Minute
	}
	if time.Since(entry.Timestamp) > ttl {
		return "", false
	}

	return entry.Text, true
}

// Put stores text in cache
func (c *TextCache) Put(textType string, hash int32, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.cache[textType]; !ok {
		c.cache[textType] = make(map[int32]TextEntry)
	}

	c.cache[textType][hash] = TextEntry{
		Text:      text,
		Timestamp: time.Now(),
		Negative:  text == "",
	}
}

// GetOrFetch retrieves text from cache or fetches from server
func (c *TextCache) GetOrFetch(textType string, hash int32) string {
	if hash == 0 {
		return ""
	}

	// Check cache first (including negative cache)
	if text, ok := c.Get(textType, hash); ok {
		return text
	}

	// Fetch from server
	text := c.fetchFromServer(textType, hash)

	// Cache both positive and negative results to avoid repeated lookups.
	// Negative entries use a shorter TTL (5 min) via the Negative flag.
	c.Put(textType, hash, text)

	return text
}

// fetchFromServer fetches text from a connected server.
// Uses GET_TEXT first (MapPack response), falls back to GET_TEXT_PACK (streaming TextPack)
// if the text is not found. Some text types (like method) are stored in permanent storage
// and may only be retrievable via GET_TEXT_PACK.
func (c *TextCache) fetchFromServer(textType string, hash int32) string {
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return ""
	}

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		date := time.Now().Format("20060102")

		// Try GET_TEXT first (works well for service, etc.)
		text := c.fetchViaGetText(session, textType, hash, date)
		if text != "" {
			return text
		}

		// Fall back to GET_TEXT_PACK (works for method, error stored in permanent DB)
		text = c.fetchViaGetTextPack(session, textType, hash, date)
		if text != "" {
			return text
		}
	}

	return ""
}

// fetchViaGetText uses CMD_GET_TEXT which returns a MapPack with Hexa32-encoded keys
func (c *TextCache) fetchViaGetText(session interface{ Request(string, *pack.MapPack) (*pack.MapPack, error) }, textType string, hash int32, date string) string {
	param := pack.NewMapPack()
	param.PutText("date", date)
	param.PutText(protocol.ParamTextType, textType)
	hashList := &io.ListValue{}
	hashList.Add(io.NewDecimalValue(hash))
	param.Put(protocol.ParamHashValue, hashList)

	resp, err := session.Request(protocol.CMD_GET_TEXT, param)
	if err != nil {
		return ""
	}
	if resp != nil {
		key := protocol.Hexa32ToString32(int64(hash))
		return resp.GetText(key)
	}
	return ""
}

// fetchViaGetTextPack uses CMD_GET_TEXT_PACK which streams TextPack objects directly
func (c *TextCache) fetchViaGetTextPack(session interface {
	RequestStream(string, *pack.MapPack, func(pack.Pack) bool) error
}, textType string, hash int32, date string) string {
	param := pack.NewMapPack()
	param.PutText("date", date)
	param.PutText(protocol.ParamTextType, textType)
	hashList := &io.ListValue{}
	hashList.Add(io.NewDecimalValue(hash))
	param.Put(protocol.ParamHashValue, hashList)

	var result string
	_ = session.RequestStream(protocol.CMD_GET_TEXT_PACK, param, func(p pack.Pack) bool {
		if tp, ok := p.(*pack.TextPack); ok {
			if tp.Hash == hash && tp.Text != "" {
				result = tp.Text
			}
		}
		return true
	})
	// Ignore EOF errors - GET_TEXT_PACK may close stream without NoNEXT flag
	return result
}

// BatchGet retrieves multiple texts, fetching missing ones
func (c *TextCache) BatchGet(textType string, hashes []int32) map[int32]string {
	result := make(map[int32]string)
	var missing []int32

	// Check cache for each hash
	for _, hash := range hashes {
		if text, ok := c.Get(textType, hash); ok {
			result[hash] = text
		} else {
			missing = append(missing, hash)
		}
	}

	// Fetch missing from server
	if len(missing) > 0 {
		fetched := c.batchFetchFromServer(textType, missing)
		for hash, text := range fetched {
			c.Put(textType, hash, text)
			result[hash] = text
		}
	}

	return result
}

// batchFetchFromServer fetches multiple texts from server using a single request
func (c *TextCache) batchFetchFromServer(textType string, hashes []int32) map[int32]string {
	result := make(map[int32]string)

	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return result
	}

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutText(protocol.ParamTextType, textType)
		hashList := &io.ListValue{}
		for _, hash := range hashes {
			if _, exists := result[hash]; !exists {
				hashList.Add(io.NewDecimalValue(hash))
			}
		}
		if hashList.Size() == 0 {
			break
		}
		param.Put(protocol.ParamHashValue, hashList)

		resp, err := session.Request(protocol.CMD_GET_TEXT, param)
		if err == nil && resp != nil {
			for _, hash := range hashes {
				key := protocol.Hexa32ToString32(int64(hash))
				text := resp.GetText(key)
				if text != "" {
					result[hash] = text
				}
			}
		}
		break
	}

	return result
}

// Clear clears all cached entries
func (c *TextCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]map[int32]TextEntry)
}

// ClearType clears cached entries for a specific type
func (c *TextCache) ClearType(textType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, textType)
}

// Size returns the total number of cached entries
func (c *TextCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, typeCache := range c.cache {
		total += len(typeCache)
	}
	return total
}

// Prune removes expired entries
func (c *TextCache) Prune() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for textType, typeCache := range c.cache {
		for hash, entry := range typeCache {
			if now.Sub(entry.Timestamp) > c.ttl {
				delete(typeCache, hash)
			}
		}
		if len(typeCache) == 0 {
			delete(c.cache, textType)
		}
	}
}

// --- Convenience methods for common text types ---

// GetService retrieves service name by hash
func (c *TextCache) GetService(hash int32) string {
	return c.GetOrFetch(pack.TextTypeService, hash)
}

// GetMethod retrieves method name by hash
func (c *TextCache) GetMethod(hash int32) string {
	return c.GetOrFetch(pack.TextTypeMethod, hash)
}

// GetSQL retrieves SQL text by hash
func (c *TextCache) GetSQL(hash int32) string {
	return c.GetOrFetch(pack.TextTypeSQL, hash)
}

// GetError retrieves error message by hash
func (c *TextCache) GetError(hash int32) string {
	return c.GetOrFetch(pack.TextTypeError, hash)
}

// GetAPICall retrieves API call URL by hash
func (c *TextCache) GetAPICall(hash int32) string {
	return c.GetOrFetch(pack.TextTypeAPICall, hash)
}

// GetUserAgent retrieves user agent by hash
func (c *TextCache) GetUserAgent(hash int32) string {
	return c.GetOrFetch(pack.TextTypeUserAgent, hash)
}

// GetReferer retrieves referer by hash
func (c *TextCache) GetReferer(hash int32) string {
	return c.GetOrFetch(pack.TextTypeReferer, hash)
}

// GetGroup retrieves group name by hash
func (c *TextCache) GetGroup(hash int32) string {
	return c.GetOrFetch(pack.TextTypeGroup, hash)
}

// GetLogin retrieves login name by hash
func (c *TextCache) GetLogin(hash int32) string {
	return c.GetOrFetch(pack.TextTypeLogin, hash)
}

// GetDesc retrieves description by hash
func (c *TextCache) GetDesc(hash int32) string {
	return c.GetOrFetch(pack.TextTypeDesc, hash)
}

// GetMessage retrieves hash message by hash
func (c *TextCache) GetMessage(hash int32) string {
	return c.GetOrFetch(pack.TextTypeHashMsg, hash)
}

