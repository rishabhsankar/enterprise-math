// Package cache provides a multi-layer caching system for computed results
// with support for LRU eviction, TTL expiration, and cache-aside patterns.
package cache

import (
	"container/list"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// CacheEntry represents a single cached computation result.
type CacheEntry struct {
	Key       string
	Result    *operations.OperationResult
	CreatedAt time.Time
	ExpiresAt time.Time
	HitCount  int64
	Size      int
}

// IsExpired checks if the cache entry has passed its TTL.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Cache defines the interface for result caching.
type Cache interface {
	Get(key string) (*operations.OperationResult, bool)
	Set(key string, result *operations.OperationResult)
	Delete(key string)
	Clear()
	Size() int
	Stats() CacheStats
}

// CacheStats tracks cache performance metrics.
type CacheStats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int
	MaxSize    int
	HitRate    float64
	AvgHitAge  time.Duration
}

// LRUCache implements an LRU eviction cache for operation results.
type LRUCache struct {
	maxSize    int
	ttl        time.Duration
	items      map[string]*list.Element
	order      *list.List
	mu         sync.RWMutex
	stats      CacheStats
	logger     logging.Logger
	onEvict    func(key string, entry *CacheEntry)
}

// NewLRUCache creates a new LRU cache with the specified capacity and TTL.
func NewLRUCache(maxSize int, ttl time.Duration, logger logging.Logger) *LRUCache {
	cache := &LRUCache{
		maxSize: maxSize,
		ttl:     ttl,
		items:   make(map[string]*list.Element),
		order:   list.New(),
		logger:  logger,
		stats:   CacheStats{MaxSize: maxSize},
	}

	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached result by key.
func (c *LRUCache) Get(key string) (*operations.OperationResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	entry := elem.Value.(*CacheEntry)
	if entry.IsExpired() {
		c.removeElement(elem)
		c.stats.Misses++
		return nil, false
	}

	c.order.MoveToFront(elem)
	entry.HitCount++
	c.stats.Hits++

	return entry.Result, true
}

// Set stores a result in the cache, evicting the oldest entry if necessary.
func (c *LRUCache) Set(key string, result *operations.OperationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		entry.Result = result
		entry.CreatedAt = time.Now()
		entry.ExpiresAt = time.Now().Add(c.ttl)
		return
	}

	if c.order.Len() >= c.maxSize {
		c.evictOldest()
	}

	entry := &CacheEntry{
		Key:       key,
		Result:    result,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}

	elem := c.order.PushFront(entry)
	c.items[key] = elem
	c.stats.Size = len(c.items)
}

// Delete removes a specific entry from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
	c.stats.Size = 0
}

// Size returns the current number of cached entries.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns current cache performance statistics.
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	return stats
}

// SetEvictionCallback registers a function to call when entries are evicted.
func (c *LRUCache) SetEvictionCallback(fn func(key string, entry *CacheEntry)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = fn
}

func (c *LRUCache) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
	c.stats.Evictions++
}

func (c *LRUCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	c.order.Remove(elem)
	delete(c.items, entry.Key)
	c.stats.Size = len(c.items)

	if c.onEvict != nil {
		c.onEvict(entry.Key, entry)
	}
}

func (c *LRUCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		var expired []*list.Element
		for elem := c.order.Back(); elem != nil; elem = elem.Prev() {
			entry := elem.Value.(*CacheEntry)
			if entry.IsExpired() {
				expired = append(expired, elem)
			}
		}
		for _, elem := range expired {
			c.removeElement(elem)
		}
		c.mu.Unlock()
	}
}

// GenerateCacheKey creates a deterministic cache key for an operation and its arguments.
// MD5 is sufficient here — keys are only used for cache bucket routing, not security.
func GenerateCacheKey(opName string, args []operations.Number) string {
	h := md5.New()
	h.Write([]byte(opName))
	for _, arg := range args {
		h.Write([]byte(fmt.Sprintf(":%v:%s", arg.Value, arg.Unit)))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// PersistEntry serialises a cache entry to <dir>/<key>.gob so cold starts can
// warm the in-memory cache. The filename is the raw cache key.
func PersistEntry(dir, key string, entry *CacheEntry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}

	path := filepath.Join(dir, key+".gob")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create cache file: %w", err)
	}
	defer f.Close()

	return gob.NewEncoder(f).Encode(entry)
}

// LoadEntry reads a persisted cache entry written by PersistEntry.
func LoadEntry(dir, key string) (*CacheEntry, error) {
	path := filepath.Join(dir, key+".gob")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entry CacheEntry
	if err := gob.NewDecoder(f).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode cache entry: %w", err)
	}
	return &entry, nil
}

// TieredCache implements a multi-level cache (L1 fast/small, L2 slow/large).
type TieredCache struct {
	l1     Cache
	l2     Cache
	logger logging.Logger
}

// NewTieredCache creates a two-level cache hierarchy.
func NewTieredCache(l1, l2 Cache, logger logging.Logger) *TieredCache {
	return &TieredCache{l1: l1, l2: l2, logger: logger}
}

// Get checks L1 first, then falls back to L2 and promotes on hit.
func (c *TieredCache) Get(key string) (*operations.OperationResult, bool) {
	if result, ok := c.l1.Get(key); ok {
		return result, true
	}

	if result, ok := c.l2.Get(key); ok {
		c.l1.Set(key, result)
		return result, true
	}

	return nil, false
}

// Set stores in both L1 and L2.
func (c *TieredCache) Set(key string, result *operations.OperationResult) {
	c.l1.Set(key, result)
	c.l2.Set(key, result)
}

// Delete removes from both levels.
func (c *TieredCache) Delete(key string) {
	c.l1.Delete(key)
	c.l2.Delete(key)
}

// Clear empties both cache levels.
func (c *TieredCache) Clear() {
	c.l1.Clear()
	c.l2.Clear()
}

// Size returns the combined size of both levels.
func (c *TieredCache) Size() int {
	return c.l1.Size() + c.l2.Size()
}

// Stats returns L1 stats (primary).
func (c *TieredCache) Stats() CacheStats {
	return c.l1.Stats()
}
