package cache

import (
	"sync"
	"time"
)

type memoryEntry struct {
	data      []byte
	expiresAt time.Time
}

// MemoryCache is a simple in-memory cache with TTL support.
type MemoryCache struct {
	items map[string]memoryEntry
	mu    sync.RWMutex
}

func NewMemory() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]memoryEntry),
	}
}

func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.data, true
}

func (c *MemoryCache) Set(key string, data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = memoryEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]memoryEntry)
}

func (c *MemoryCache) Stats() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := len(c.items)
	expired := 0

	for _, item := range c.items {
		if time.Now().After(item.expiresAt) {
			expired++
		}
	}

	return map[string]int{
		"total":   total,
		"expired": expired,
		"active":  total - expired,
	}
}
