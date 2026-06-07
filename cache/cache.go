package cache

import (
	"time"

	"github.com/maypok86/otter/v2"
)

// Cache is a generic in-memory cache with per-entry TTL, backed by otter.
// Entries without an explicit TTL fall back to defaultTTL.
type Cache[K comparable, V any] struct {
	cache *otter.Cache[K, valueTTL[V]]
}

type valueTTL[V any] struct {
	value V
	ttl   time.Duration
}

const defaultTTL = 5 * time.Minute

// NewCache creates an in-memory cache whose entries expire per the TTL
// given at Set time (falling back to defaultTTL when non-positive).
func NewCache[K comparable, V any]() *Cache[K, V] {
	cache := otter.Must(&otter.Options[K, valueTTL[V]]{
		ExpiryCalculator: otter.ExpiryAccessingFunc(func(e otter.Entry[K, valueTTL[V]]) time.Duration {
			if e.Value.ttl > 0 {
				return e.Value.ttl
			}
			return defaultTTL
		}),
	})
	return &Cache[K, V]{
		cache: cache,
	}
}

func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.cache.Set(key, valueTTL[V]{
		value: value,
		ttl:   ttl,
	})
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	value, ok := c.cache.GetIfPresent(key)
	if !ok {
		var zero V
		return zero, false
	}
	return value.value, true
}

func (c *Cache[K, V]) Delete(key K) {
	c.cache.Invalidate(key)
}

func (c *Cache[K, V]) Close() {
	c.cache.CleanUp()
}
