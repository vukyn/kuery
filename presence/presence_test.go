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

func TestHasAnyAndCounts(t *testing.T) {
	clock := newFakeClock()
	tracker := newTestTracker(clock, 30*time.Second)
	defer tracker.Stop()

	if tracker.HasAny() {
		t.Fatal("HasAny should be false on an empty tracker")
	}
	if got := tracker.Counts(); len(got) != 0 {
		t.Fatalf("Counts should be empty on an empty tracker, got %v", got)
	}

	tracker.Touch("station-1", "session-a")
	tracker.Touch("station-1", "session-b")
	tracker.Touch("station-2", "session-c")

	if !tracker.HasAny() {
		t.Fatal("HasAny should be true with live members present")
	}

	counts := tracker.Counts()
	if len(counts) != 2 {
		t.Fatalf("Counts should carry exactly 2 groups, got %v", counts)
	}
	if counts["station-1"] != 2 {
		t.Fatalf("station-1 Counts = %d, want 2", counts["station-1"])
	}
	if counts["station-2"] != 1 {
		t.Fatalf("station-2 Counts = %d, want 1", counts["station-2"])
	}
	// Counts must agree with the existing per-group Count read path.
	if counts["station-1"] != tracker.Count("station-1") {
		t.Fatalf("Counts[station-1]=%d disagrees with Count=%d", counts["station-1"], tracker.Count("station-1"))
	}
}

func TestHasAnyFalseAfterTTLExpiry(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	tracker.Touch("station-1", "session-a")
	if !tracker.HasAny() {
		t.Fatal("expected live immediately after Touch")
	}

	// Advance past the TTL; the read-time filter must drop the member from every
	// accessor without a sweep.
	clock.Advance(ttl + time.Second)
	if tracker.HasAny() {
		t.Fatal("HasAny should be false once every member has expired")
	}
	if got := tracker.Counts(); len(got) != 0 {
		t.Fatalf("Counts should be empty once every member has expired, got %v", got)
	}
}

func TestCountsOmitsEmptyGroups(t *testing.T) {
	clock := newFakeClock()
	ttl := 30 * time.Second
	tracker := newTestTracker(clock, ttl)
	defer tracker.Stop()

	// "stale" is seen at t0; "fresh" is seen 20s later.
	tracker.Touch("stale", "session-a")
	clock.Advance(20 * time.Second)
	tracker.Touch("fresh", "session-b")

	// Advance so "stale" (seen at t0) is 31s old → expired, while "fresh" (seen at
	// t0+20s) is only 11s old → still live.
	clock.Advance(11 * time.Second)

	counts := tracker.Counts()
	if _, ok := counts["stale"]; ok {
		t.Fatalf("expired group must be omitted from Counts, got %v", counts)
	}
	if counts["fresh"] != 1 {
		t.Fatalf("fresh group Counts = %d, want 1", counts["fresh"])
	}
	if !tracker.HasAny() {
		t.Fatal("HasAny should stay true while the fresh member is live")
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
