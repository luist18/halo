package cache

import (
	"sync"
	"time"
)

// TTLCache is a generic in-memory cache with per-entry TTL semantics.
// Entries expire after ttl unless they are accessed, which refreshes the TTL.
type TTLCache[K comparable, V any] struct {
	ttl             time.Duration
	cleanupInterval time.Duration
	onEvict         func(K, V)

	mu      sync.RWMutex
	items   map[K]*cacheEntry[V]
	stopCh  chan struct{}
	closeMu sync.Mutex
	closed  bool
}

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// Option configures a TTLCache instance.
type Option[K comparable, V any] func(*TTLCache[K, V])

// WithCleanupInterval adjusts how frequently expired entries are cleaned up.
// If interval <= 0, background cleanup is disabled.
func WithCleanupInterval[K comparable, V any](interval time.Duration) Option[K, V] {
	return func(c *TTLCache[K, V]) {
		if interval <= 0 {
			c.cleanupInterval = 0
			return
		}
		c.cleanupInterval = interval
	}
}

// WithOnEvict registers a callback invoked when an entry is removed.
func WithOnEvict[K comparable, V any](fn func(K, V)) Option[K, V] {
	return func(c *TTLCache[K, V]) {
		c.onEvict = fn
	}
}

// NewTTLCache builds a TTL cache with the provided ttl and options.
func NewTTLCache[K comparable, V any](ttl time.Duration, opts ...Option[K, V]) *TTLCache[K, V] {
	if ttl <= 0 {
		panic("cache ttl must be greater than zero")
	}

	cache := &TTLCache[K, V]{
		ttl:             ttl,
		cleanupInterval: ttl,
		items:           make(map[K]*cacheEntry[V]),
	}

	for _, opt := range opts {
		opt(cache)
	}

	if cache.cleanupInterval > 0 {
		cache.stopCh = make(chan struct{})
		go cache.cleanupLoop()
	}

	return cache
}

// Get returns the cached value if present and not expired.
// On hit, the TTL is refreshed.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	if c.isExpired(entry) {
		c.removeLocked(key, entry)
		var zero V
		return zero, false
	}

	entry.expiresAt = time.Now().Add(c.ttl)
	return entry.value, true
}

// Set stores the value and resets the TTL.
// Replacing an existing entry triggers the eviction callback.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setLocked(key, value)
}

// Delete removes an entry if it exists.
func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return
	}

	c.removeLocked(key, entry)
}

// Len returns the number of stored entries.
func (c *TTLCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Close stops background cleanup and clears the cache, invoking eviction callbacks.
func (c *TTLCache[K, V]) Close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	closeCh := c.stopCh
	c.closeMu.Unlock()

	if closeCh != nil {
		close(closeCh)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.items {
		c.disposeValue(key, entry.value)
		delete(c.items, key)
	}
}

// GetOrCompute retrieves an entry or computes and stores a new value.
// If another goroutine populates the value while compute executes, the new
// value is discarded and the existing one is returned.
func (c *TTLCache[K, V]) GetOrCompute(key K, compute func() (V, error)) (V, error) {
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	var zero V

	value, err := compute()
	if err != nil {
		return zero, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.items[key]; ok && !c.isExpired(entry) {
		entry.expiresAt = time.Now().Add(c.ttl)
		c.disposeValue(key, value)
		return entry.value, nil
	}

	c.items[key] = &cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}

	return value, nil
}

func (c *TTLCache[K, V]) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCh:
			return
		}
	}
}

func (c *TTLCache[K, V]) removeExpired() {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.items {
		if now.After(entry.expiresAt) {
			c.removeLocked(key, entry)
		}
	}
}

func (c *TTLCache[K, V]) setLocked(key K, value V) {
	if entry, ok := c.items[key]; ok {
		c.disposeValue(key, entry.value)
	}

	c.items[key] = &cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *TTLCache[K, V]) removeLocked(key K, entry *cacheEntry[V]) {
	delete(c.items, key)
	c.disposeValue(key, entry.value)
}

func (c *TTLCache[K, V]) isExpired(entry *cacheEntry[V]) bool {
	return time.Now().After(entry.expiresAt)
}

func (c *TTLCache[K, V]) disposeValue(key K, value V) {
	if c.onEvict != nil {
		c.onEvict(key, value)
	}
}
