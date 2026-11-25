// Package ttlcache provides bounded, process-local read-through caches.
package ttlcache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value V
	until time.Time
}
type Cache[K comparable, V any] struct {
	mu         sync.Mutex
	entries    map[K]entry[V]
	generation uint64
}

func (c *Cache[K, V]) Load(key K, ttl time.Duration, fetch func() (V, error)) (V, error) {
	c.mu.Lock()
	e := c.entries[key]
	gen := c.generation
	c.mu.Unlock()
	now := time.Now()
	if now.Before(e.until) {
		return e.value, nil
	}
	v, err := fetch()
	if err != nil {
		return v, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen == c.generation {
		if c.entries == nil {
			c.entries = make(map[K]entry[V])
		}
		if len(c.entries) >= 1024 {
			for old := range c.entries {
				delete(c.entries, old)
				break
			}
		}
		c.entries[key] = entry[V]{v, now.Add(ttl)}
	}
	return v, nil
}
func (c *Cache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	delete(c.entries, key)
}

func (c *Cache[K, V]) InvalidateMatching(matches func(K) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	for key := range c.entries {
		if matches(key) {
			delete(c.entries, key)
		}
	}
}
