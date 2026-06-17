package presence

import (
	"sync"
	"time"
)

// Tracker is an in-process presence/heartbeat store: group -> member ->
// last-seen timestamp with TTL-based expiry. State is nested by group so a
// per-group count is O(members-in-group) rather than O(all-members).
//
// All methods are safe for concurrent use. State is in-memory only and does
// not survive a process restart.
type Tracker struct {
	mutex   sync.RWMutex
	members map[string]map[string]time.Time
	ttl     time.Duration
	now     func() time.Time

	ticker   *time.Ticker
	done     chan struct{}
	stopOnce sync.Once
}

// New creates a Tracker with the given member TTL and sweep interval, and
// starts a background goroutine that periodically prunes expired members and
// drops empty groups. Call Stop to release the goroutine on shutdown.
func New(ttl time.Duration, sweepInterval time.Duration) *Tracker {
	return newWithClock(ttl, sweepInterval, time.Now)
}

// newWithClock builds a Tracker with an injectable clock so behaviour can be
// driven by a fake clock in tests instead of wall-clock time.
func newWithClock(ttl time.Duration, sweepInterval time.Duration, now func() time.Time) *Tracker {
	tracker := &Tracker{
		members: make(map[string]map[string]time.Time),
		ttl:     ttl,
		now:     now,
		ticker:  time.NewTicker(sweepInterval),
		done:    make(chan struct{}),
	}
	go tracker.sweepLoop()
	return tracker
}

// Touch records a heartbeat, upserting the member's last-seen time to now.
func (t *Tracker) Touch(group, member string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	groupMembers, ok := t.members[group]
	if !ok {
		groupMembers = make(map[string]time.Time)
		t.members[group] = groupMembers
	}
	groupMembers[member] = t.now()
}

// Count returns the number of live members in the group. Liveness is filtered
// at read time so an entry that has expired since the last sweep is excluded.
func (t *Tracker) Count(group string) int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	cutoff := t.now().Add(-t.ttl)
	count := 0
	for _, lastSeen := range t.members[group] {
		if lastSeen.After(cutoff) {
			count++
		}
	}
	return count
}

// Members returns the live member ids in the group. The order is unspecified.
func (t *Tracker) Members(group string) []string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	cutoff := t.now().Add(-t.ttl)
	live := make([]string, 0, len(t.members[group]))
	for member, lastSeen := range t.members[group] {
		if lastSeen.After(cutoff) {
			live = append(live, member)
		}
	}
	return live
}

// Remove explicitly drops a member from a group, deleting the group if it
// becomes empty.
func (t *Tracker) Remove(group, member string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	groupMembers, ok := t.members[group]
	if !ok {
		return
	}
	delete(groupMembers, member)
	if len(groupMembers) == 0 {
		delete(t.members, group)
	}
}

// Stop halts the background sweep goroutine. It is idempotent and safe to call
// more than once.
func (t *Tracker) Stop() {
	t.stopOnce.Do(func() {
		t.ticker.Stop()
		close(t.done)
	})
}

// sweepLoop prunes expired members until Stop is called.
func (t *Tracker) sweepLoop() {
	for {
		select {
		case <-t.done:
			return
		case <-t.ticker.C:
			t.sweep()
		}
	}
}

// sweep removes every member whose last-seen time is older than the TTL and
// drops any group left empty.
func (t *Tracker) sweep() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	cutoff := t.now().Add(-t.ttl)
	for group, groupMembers := range t.members {
		for member, lastSeen := range groupMembers {
			if !lastSeen.After(cutoff) {
				delete(groupMembers, member)
			}
		}
		if len(groupMembers) == 0 {
			delete(t.members, group)
		}
	}
}
