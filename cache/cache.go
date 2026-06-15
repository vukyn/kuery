package cache

import (
	"time"

	"github.com/maypok86/otter/v2"
)

// Cache is a generic in-memory cache with per-entry TTL, backed by otter.
//
// TTL contract: a ttl <= 0 stores a permanent entry that never expires;
// a ttl > 0 stores an entry that expires after that duration.
type Cache[K comparable, V any] struct {
	cache *otter.Cache[K, valueTTL[V]]
}

type valueTTL[V any] struct {
	value V
	ttl   time.Duration
}

// neverExpire is the sentinel duration used for permanent entries. otter
// stores expiry as nowNano + int64(duration), so returning math.MaxInt64
// would overflow int64 and make the entry expire immediately; a ~100-year
// duration is effectively permanent while staying well within int64 range.
const neverExpire = 100 * 365 * 24 * time.Hour

// permanentThreshold guards GetWithTTL against the never-expire sentinel:
// any remaining TTL above ~50 years is treated as permanent. This sits below
// neverExpire (~100 years) and far above any realistic finite TTL.
const permanentThreshold = 50 * 365 * 24 * time.Hour

// NewCache creates an in-memory cache whose entries expire per the TTL
// given at Set time.
//
// TTL contract: a ttl <= 0 stores a permanent entry that never expires;
// a ttl > 0 stores an entry that expires after that duration.
func NewCache[K comparable, V any]() *Cache[K, V] {
	cache := otter.Must(&otter.Options[K, valueTTL[V]]{
		ExpiryCalculator: otter.ExpiryAccessingFunc(func(e otter.Entry[K, valueTTL[V]]) time.Duration {
			if e.Value.ttl > 0 {
				return e.Value.ttl
			}
			return neverExpire
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

// Has reports whether the key is present. It does not mutate access
// statistics or the eviction policy.
func (c *Cache[K, V]) Has(key K) bool {
	_, ok := c.cache.GetEntryQuietly(key)
	return ok
}

// GetWithTTL returns the value, its remaining TTL, and whether the key was
// present. A permanent entry (stored with ttl <= 0) reports a remaining TTL
// of 0, which callers interpret as "no TTL / permanent".
//
// This enables the KeepTTL caller-side pattern: read the current ttl via
// GetWithTTL, then re-Set the value with that same ttl.
func (c *Cache[K, V]) GetWithTTL(key K) (V, time.Duration, bool) {
	entry, ok := c.cache.GetEntryQuietly(key)
	if !ok {
		var zero V
		return zero, 0, false
	}
	remaining := entry.ExpiresAfter()
	if remaining >= permanentThreshold {
		remaining = 0
	}
	return entry.Value.value, remaining, true
}

// SetNX sets the value only if the key is absent. It returns true if the
// value was stored, false if the key already existed. The TTL contract
// matches Set: ttl <= 0 stores a permanent entry.
func (c *Cache[K, V]) SetNX(key K, value V, ttl time.Duration) bool {
	_, set := c.cache.SetIfAbsent(key, valueTTL[V]{
		value: value,
		ttl:   ttl,
	})
	return set
}

// Range iterates over all live entries, invoking fn for each. Iteration
// stops early when fn returns false.
func (c *Cache[K, V]) Range(fn func(key K, value V) bool) {
	for key, value := range c.cache.All() {
		if !fn(key, value.value) {
			return
		}
	}
}

// Keys returns a snapshot slice of all live keys.
func (c *Cache[K, V]) Keys() []K {
	keys := make([]K, 0, c.cache.EstimatedSize())
	for key := range c.cache.Keys() {
		keys = append(keys, key)
	}
	return keys
}
