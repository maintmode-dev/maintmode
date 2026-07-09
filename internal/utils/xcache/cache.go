// Package xcache provides a small process-local TTL cache whose writes are
// guarded by per-key invalidation generations, closing the read-then-cache vs
// invalidate race that a plain delete-on-write cache cannot: an invalidation
// can only remove what is already cached — it cannot stop an in-flight reader
// that loaded source state BEFORE the write from caching that stale state
// AFTER the invalidation ran.
//
// The intended flow for a cache-filling read:
//
//	if v, ok := c.Get(key); ok { return v }
//	snap := c.Generation(key)
//	v := loadAndBuild(key)   // slow: DB read, decrypt, client construction…
//	c.Set(key, v, snap)      // silently dropped if Invalidate ran since snap
//
// and every write to the underlying source calls c.Invalidate(key) after
// commit. On the instance that served the write, invalidation is then
// authoritative — a concurrent stale build cannot resurrect pre-write state;
// the TTL only bounds staleness on replicas that did not serve the write.
package xcache

import (
	"sync"
	"time"
)

// Cache is a mutex-guarded TTL map with generation-checked writes. It is built
// for small, bounded key sets (its generation map is never cleaned — see
// Invalidate); it deliberately has no eviction policy beyond TTL.
type Cache[K comparable, V any] struct {
	mu  sync.Mutex
	ttl time.Duration
	// now is injectable for deterministic TTL tests; defaults to time.Now.
	now     func() time.Time
	entries map[K]entry[V]
	// gen tracks the invalidation generation per key; a Set is dropped if the
	// generation moved between the caller's snapshot and the Set.
	gen map[K]uint64
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

func New[K comparable, V any](ttl time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[K]entry[V]),
		gen:     make(map[K]uint64),
	}
}

// Get returns the cached value for key if present and unexpired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V

	e, ok := c.entries[key]
	if !ok {
		return zero, false
	}
	if c.now().After(e.expiresAt) {
		delete(c.entries, key)
		return zero, false
	}
	return e.value, true
}

// Generation returns the current invalidation generation for key. A
// cache-filling read snapshots this BEFORE loading the source and passes it
// back to Set.
func (c *Cache[K, V]) Generation(key K) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen[key]
}

// Set caches the value for key with the configured TTL, but only if no
// Invalidate bumped the key's generation since the caller's snapshot. This
// prevents a reader that loaded pre-write source state from repopulating the
// cache after the write's invalidation already ran.
func (c *Cache[K, V]) Set(key K, value V, snapshot uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen[key] != snapshot {
		return // invalidated mid-build: the built value is already stale
	}
	c.entries[key] = entry[V]{value: value, expiresAt: c.now().Add(c.ttl)}
}

// Invalidate drops the cached value for key and bumps its generation, so a
// concurrent in-flight read cannot repopulate it with pre-write state. Call it
// synchronously after every committed write to the underlying source.
//
// The generation entry itself is NEVER removed — deliberately. Deleting it
// would reset the counter to the zero value, making "never invalidated" and
// "invalidated then cleaned" indistinguishable to a reader holding a snapshot
// of 0 — the ABA that reintroduces exactly the race this cache exists to
// close. The cost is one uint64 per key ever invalidated, which is why the
// cache is documented as being for bounded key sets.
func (c *Cache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	c.gen[key]++
}
