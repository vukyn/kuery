package ratelimit

import (
	"sync"
	"time"
)

// entry is one key's fixed window.
type entry struct {
	count     int
	expiresAt time.Time
	// logged records that the first block of THIS window has already been
	// reported, so a blocked flood produces one log line per window instead of
	// one per request.
	logged bool
}

// reservation is the outcome of accounting one request against a key.
type reservation struct {
	// allowed is false when the key has already spent its budget.
	allowed bool
	// firstBlock is true only for the first blocked request of this window — the
	// one request the caller should log.
	firstBlock bool
	// retryAfter is how long until the window resets.
	retryAfter time.Duration
}

// store holds the per-key windows. Every field is guarded by mutex; the whole
// reserve/refund accounting happens under it so a burst of concurrent requests
// can never hand out more than Max slots.
type store struct {
	mutex   sync.Mutex
	entries map[string]*entry

	max     int
	window  time.Duration
	maxKeys int
	now     func() time.Time

	// nextSweepAt amortises the expiry sweep: scanning the whole map on every
	// request would make the middleware O(keys) per request.
	nextSweepAt time.Time
}

func newStore(max int, window time.Duration, maxKeys int, now func() time.Time) *store {
	return &store{
		entries:     make(map[string]*entry),
		max:         max,
		window:      window,
		maxKeys:     maxKeys,
		now:         now,
		nextSweepAt: now().Add(window),
	}
}

// reserve charges one attempt to key BEFORE the handler runs, and reports whether
// the request may proceed. refund gives the slot back when the response turns out
// not to count.
func (s *store) reserve(key string) reservation {
	now := s.now()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sweepLocked(now)

	current, found := s.entries[key]
	if !found {
		current = s.admitLocked(key, now)
	} else if !now.Before(current.expiresAt) {
		// Window expired: start a fresh one instead of carrying the old count.
		current.count = 0
		current.expiresAt = now.Add(s.window)
		current.logged = false
	}

	if current.count >= s.max {
		firstBlock := !current.logged
		current.logged = true
		// The counter stays pinned at max. Incrementing while already blocked
		// carries no information and would overflow under a long enough flood.
		return reservation{
			allowed:    false,
			firstBlock: firstBlock,
			retryAfter: current.expiresAt.Sub(now),
		}
	}

	current.count++
	return reservation{
		allowed:    true,
		retryAfter: current.expiresAt.Sub(now),
	}
}

// refund returns one slot to key. It is a no-op when the key's window has already
// rolled over or the entry was evicted — there is nothing to give back, and
// decrementing a fresh window would hand out extra budget.
//
// An entry refunded back to zero is DELETED rather than left at zero. Reserving
// creates the entry before the response status is known, so an uncounted status
// (5xx, a validation 400, an inner tier's 429) would otherwise leave a key behind
// that cost the caller no budget at all. Since the key is attacker-influenced, that
// residue is a free way to fill the bounded map: once it is full, admitting a new
// key evicts the entry nearest to expiry, which can be a real caller's — so the
// limiter starts forgetting the buckets it exists to remember. Deleting an empty
// entry keeps the map's size proportional to callers who actually spent budget.
func (s *store) refund(key string) {
	now := s.now()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	current, found := s.entries[key]
	if !found || !now.Before(current.expiresAt) {
		return
	}
	if current.count > 0 {
		current.count--
	}
	// `logged` is kept: an entry that has already reported a block in this window
	// still carries state worth remembering, so only untouched entries are dropped.
	if current.count == 0 && !current.logged {
		delete(s.entries, key)
	}
}

// admitLocked inserts a fresh window for key, making room first if the tracked
// set is at its bound.
func (s *store) admitLocked(key string, now time.Time) *entry {
	if len(s.entries) >= s.maxKeys {
		// Forced sweep: cheap, and usually enough since most entries are stale.
		s.purgeExpiredLocked(now)
	}
	if len(s.entries) >= s.maxKeys {
		s.evictNearestExpiryLocked()
	}

	fresh := &entry{expiresAt: now.Add(s.window)}
	s.entries[key] = fresh
	return fresh
}

// sweepLocked drops expired entries at most once per window.
func (s *store) sweepLocked(now time.Time) {
	if now.Before(s.nextSweepAt) {
		return
	}
	s.purgeExpiredLocked(now)
	s.nextSweepAt = now.Add(s.window)
}

func (s *store) purgeExpiredLocked(now time.Time) {
	for key, current := range s.entries {
		if !now.Before(current.expiresAt) {
			delete(s.entries, key)
		}
	}
}

// evictNearestExpiryLocked drops the entry that would have expired soonest — the
// one with the least remaining value. O(keys), but only reached when the map is
// full even after a sweep.
//
// Evicting means the new key starts with a full budget, i.e. the limiter fails
// OPEN when saturated. That is deliberate: refusing new keys instead would let an
// attacker who filled the map lock out every real user.
func (s *store) evictNearestExpiryLocked() {
	var evictKey string
	var evictAt time.Time
	// picked, not evictKey != "", because the empty string is a valid key.
	picked := false
	for key, current := range s.entries {
		if !picked || current.expiresAt.Before(evictAt) {
			evictKey = key
			evictAt = current.expiresAt
			picked = true
		}
	}
	if picked {
		delete(s.entries, evictKey)
	}
}
