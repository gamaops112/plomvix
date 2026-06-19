package planner

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// CacheKey uniquely identifies a cached plan.
type CacheKey struct {
	Fingerprint   string
	SchemaVersion uint64
}

func (k CacheKey) String() string {
	return fmt.Sprintf("%s@v%d", k.Fingerprint, k.SchemaVersion)
}

// PlanCache is a thread-safe, bounded, FIFO-evicting cache of PlanTemplates.
type PlanCache struct {
	mu       sync.RWMutex
	maxSize  int
	disabled bool
	items    map[string]*PlanTemplate
	order    []string

	hits   atomic.Uint64
	misses atomic.Uint64
}

// NewPlanCache creates a PlanCache. If maxSize <= 0, the cache is disabled.
func NewPlanCache(maxSize int) *PlanCache {
	if maxSize <= 0 {
		return &PlanCache{disabled: true}
	}
	return &PlanCache{
		maxSize: maxSize,
		items:   make(map[string]*PlanTemplate),
	}
}

// Lookup returns the cached template or nil on miss. RLock only.
func (c *PlanCache) Lookup(key CacheKey) *PlanTemplate {
	if c.disabled {
		c.misses.Add(1)
		return nil
	}
	c.mu.RLock()
	tmpl, ok := c.items[key.String()]
	c.mu.RUnlock()
	if ok {
		c.hits.Add(1)
		return tmpl
	}
	c.misses.Add(1)
	return nil
}

// Store inserts a template. If full, evicts the oldest (FIFO).
func (c *PlanCache) Store(key CacheKey, tmpl *PlanTemplate) {
	if c.disabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	k := key.String()
	if _, exists := c.items[k]; exists {
		return
	}
	if len(c.order) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
	c.items[k] = tmpl
	c.order = append(c.order, k)
}

// Stats returns current hit/miss counts and cache size.
func (c *PlanCache) Stats() (hits, misses uint64, size int) {
	if c.disabled {
		return c.hits.Load(), c.misses.Load(), 0
	}
	c.mu.RLock()
	sz := len(c.items)
	c.mu.RUnlock()
	return c.hits.Load(), c.misses.Load(), sz
}
