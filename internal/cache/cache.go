package cache

import "time"

// Cache is the interface that all cache implementations must satisfy.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, data []byte, ttl time.Duration)
	Clear()
	Stats() map[string]int
}
