package presence

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock for deterministic time control. It
// avoids any wall-clock time.Now call in tests.
type fakeClock struct {
	mutex sync.Mutex
	t     time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Unix(0, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.t = c.t.Add(d)
}

// newTestTracker builds a Tracker with a fake clock and a sweep interval long
// enough that the background goroutine never fires during a test, so liveness
// is exercised through read-time filtering and explicit sweep() calls.
func newTestTracker(clock *fakeClock, ttl time.Duration) *Tracker {
	return newWithClock(ttl, time.Hour, clock.Now)
}

func TestTouchThenCount(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("Count = %d, want 1", got)
	}
}

func TestCountTwoMembers(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	tracker.Touch("station-1", "session-b")
	if got := tracker.Count("station-1"); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
}

func TestMemberExpiresAfterTTL(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("Count before expiry = %d, want 1", got)
	}

	// Advance past the TTL; read-time filtering must drop the member.
	clock.Advance(ttl + time.Second)
	if got := tracker.Count("station-1"); got != 0 {
		t.Fatalf("Count after expiry = %d, want 0", got)
	}
}

func TestTouchKeepsMemberAlive(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	clock.Advance(20 * time.Second)
	// Heartbeat refreshes last-seen before the TTL elapses.
	tracker.Touch("station-1", "session-a")
	clock.Advance(20 * time.Second)
	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("Count = %d, want 1 (heartbeat should keep member alive)", got)
	}
}

func TestRemove(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	tracker.Touch("station-1", "session-b")
	tracker.Remove("station-1", "session-a")

	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("Count after Remove = %d, want 1", got)
	}

	tracker.Remove("station-1", "session-b")
	if got := tracker.Count("station-1"); got != 0 {
		t.Fatalf("Count after removing all = %d, want 0", got)
	}
	// Removing the last member must not leave a dangling empty group.
	tracker.mutex.RLock()
	_, exists := tracker.members["station-1"]
	tracker.mutex.RUnlock()
	if exists {
		t.Fatal("empty group should be dropped after last Remove")
	}
}

func TestRemoveMissing(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	// Must not panic on unknown group/member.
	tracker.Remove("missing", "nobody")
	tracker.Touch("station-1", "session-a")
	tracker.Remove("station-1", "nobody")
	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("Count = %d, want 1", got)
	}
}

func TestGroupsIsolated(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	tracker.Touch("station-2", "session-a")
	tracker.Touch("station-2", "session-b")

	if got := tracker.Count("station-1"); got != 1 {
		t.Fatalf("station-1 Count = %d, want 1", got)
	}
	if got := tracker.Count("station-2"); got != 2 {
		t.Fatalf("station-2 Count = %d, want 2", got)
	}
	if got := tracker.Count("station-unknown"); got != 0 {
		t.Fatalf("unknown group Count = %d, want 0", got)
	}
}

func TestMembers(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	tracker.Touch("station-1", "session-b")

	members := tracker.Members("station-1")
	if len(members) != 2 {
		t.Fatalf("Members len = %d, want 2", len(members))
	}
	seen := map[string]bool{}
	for _, member := range members {
		seen[member] = true
	}
	if !seen["session-a"] || !seen["session-b"] {
		t.Fatalf("Members = %v, want session-a and session-b", members)
	}

	clock.Advance(ttl + time.Second)
	if got := tracker.Members("station-1"); len(got) != 0 {
		t.Fatalf("Members after expiry = %v, want empty", got)
	}
}

func TestSweepDropsExpired(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	clock.Advance(ttl + time.Second)
	tracker.sweep()

	tracker.mutex.RLock()
	_, exists := tracker.members["station-1"]
	tracker.mutex.RUnlock()
	if exists {
		t.Fatal("sweep should have dropped the expired member and its empty group")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	tracker.Stop()
	// Second Stop must not panic on a re-closed channel.
	tracker.Stop()
}

func TestStopHaltsSweep(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	// Short sweep interval so the goroutine would normally fire quickly.
	tracker := newWithClock(ttl, time.Millisecond, clock.Now)

	tracker.Touch("station-1", "session-a")
	tracker.Stop()

	// After Stop, advance the clock past the TTL. With the sweep halted, the
	// raw entry must remain in the map (read-time Count still filters it).
	clock.Advance(ttl + time.Second)
	time.Sleep(10 * time.Millisecond)

	tracker.mutex.RLock()
	_, exists := tracker.members["station-1"]
	tracker.mutex.RUnlock()
	if !exists {
		t.Fatal("sweep should be halted after Stop; entry must remain unswept")
	}
}

func TestConcurrentTouch(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, time.Hour)
	defer tracker.Stop()

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(groupID int) {
			defer wg.Done()
			group := "station-" + string(rune('A'+groupID%5))
			for i := range perGoroutine {
				tracker.Touch(group, "session-"+string(rune('a'+i%26)))
				tracker.Count(group)
				tracker.Members(group)
			}
		}(g)
	}
	wg.Wait()

	// Sanity: at least one group accumulated members without a panic/data race.
	total := 0
	for _, group := range []string{"station-A", "station-B", "station-C", "station-D", "station-E"} {
		total += tracker.Count(group)
	}
	if total == 0 {
		t.Fatal("expected some live members after concurrent Touch")
	}
}
