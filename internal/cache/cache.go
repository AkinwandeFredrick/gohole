package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	items   map[string]*entry
	maxSize int
	hits    int64
	misses  int64
}

func New(maxSize int) *Cache {
	c := &Cache{
		items:   make(map[string]*entry),
		maxSize: maxSize,
	}
	go c.evictLoop()
	return c
}

func (c *Cache) Get(key string) interface{} {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
	return e.value
}

func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *Cache) evictOldest() {
	var oldest string
	var oldestTime time.Time

	for k, e := range c.items {
		if oldest == "" || e.expiresAt.Before(oldestTime) {
			oldest = k
			oldestTime = e.expiresAt
		}
	}

	if oldest != "" {
		delete(c.items, oldest)
	}
}

func (c *Cache) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"size":     len(c.items),
		"max_size": c.maxSize,
		"hits":     c.hits,
		"misses":   c.misses,
		"hit_rate": hitRate,
	}
}

// Size returns the current number of items in the cache
func (c *Cache) Size() (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items), nil
}

// HitRate returns the cache hit rate as a percentage
func (c *Cache) HitRate() (float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0, nil
	}
	return float64(c.hits) / float64(total) * 100, nil
}

func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*entry)
}
