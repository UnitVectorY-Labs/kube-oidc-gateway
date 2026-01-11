package gateway

import (
	"sync"
	"time"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Body      []byte
	ExpiresAt time.Time
}

// Cache provides in-memory caching with TTL
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached entry if it exists and is not expired
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Body, true
}

// Set stores a value in the cache with TTL
func (c *Cache) Set(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Body:      body,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}
