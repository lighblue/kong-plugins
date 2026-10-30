package outbound

import (
	"sync"
	"time"
)

type memoryCacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

type MemoryCache struct {
	mu        sync.RWMutex
	ttl       time.Duration
	entries   map[string]memoryCacheEntry
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewMemoryCache starts periodic cleanup when cleanupInterval is positive.
// Call Close when the cache is no longer needed.
func NewMemoryCache(ttl, cleanupInterval time.Duration) *MemoryCache {
	cache := &MemoryCache{
		ttl:     ttl,
		entries: make(map[string]memoryCacheEntry),
	}
	if cleanupInterval > 0 {
		cache.stop = make(chan struct{})
		cache.done = make(chan struct{})
		go cache.cleanupPeriodically(cleanupInterval)
	}
	return cache
}

func (c *MemoryCache) Get(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || !time.Now().Before(entry.expiresAt) {
		return nil, nil
	}
	return entry.value, nil
}

func (c *MemoryCache) Set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ttl <= 0 {
		return nil
	}
	c.entries[key] = memoryCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	return nil
}

func (c *MemoryCache) cleanupPeriodically(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(c.done)

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stop:
			return
		}
	}
}

func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// Close stops periodic cleanup and waits for the cleanup goroutine to exit.
func (c *MemoryCache) Close() {
	if c.stop == nil {
		return
	}
	c.closeOnce.Do(func() { close(c.stop) })
	<-c.done
}
