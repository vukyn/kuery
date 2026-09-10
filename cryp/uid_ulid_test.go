package cryp

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	ulid "github.com/oklog/ulid/v2"
)

// ULID is used across this platform as a credential — an OAuth authorization
// code, a session id, a JWT jti, an object-storage key — not only as a row id.
// ulid.Make() would satisfy every shape assertion below while remaining
// predictable, so the tests that matter are the two that compare against
// ulid.Make()'s actual behaviour rather than against a format.

func TestULIDIsParseableAndTimeOrdered(t *testing.T) {
	first := ULID()
	time.Sleep(2 * time.Millisecond)
	second := ULID()

	for _, encoded := range []string{first, second} {
		if _, err := ulid.ParseStrict(encoded); err != nil {
			t.Fatalf("ULID() = %q, not a valid ULID: %v", encoded, err)
		}
	}
	if !(first < second) {
		t.Errorf("ULIDs minted 2ms apart did not sort by time: %q then %q", first, second)
	}
}

// The timestamp prefix must stay a real timestamp: callers rely on ULIDs
// sorting by creation time, and swapping the whole value for random bytes would
// pass a uniqueness test while silently breaking that.
func TestULIDCarriesCurrentTimestamp(t *testing.T) {
	before := ulid.Timestamp(time.Now())
	parsed, err := ulid.ParseStrict(ULID())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	after := ulid.Timestamp(time.Now())

	if parsed.Time() < before || parsed.Time() > after {
		t.Errorf("ULID timestamp %d outside [%d, %d]", parsed.Time(), before, after)
	}
}

// The regression guard, and the only test here that fails if someone reverts to
// ulid.Make().
//
// ulid.Make() uses ulid.Monotonic, which GUARANTEES that ULIDs sharing a
// millisecond have strictly increasing entropy. crypto/rand has no such
// relationship: for k values in one millisecond the chance they land ascending
// by luck is 1/k!. So: mint a burst, take the largest same-millisecond group,
// and require it NOT to be strictly ascending.
//
// ⚠️ An earlier version of this test compared "near-neighbour" entropy pairs and
// t.Skip()ed when it observed none from ulid.Make() — which it always did,
// because Monotonic's increment is not small. A skipped test reads exactly like
// a passing one, so it would have guarded nothing. Hence the hard failure below
// when the burst cannot produce a usable group, and the explicit assertion that
// ulid.Make() really is ascending: if that premise ever stops holding, this test
// says so instead of quietly passing.
func TestULIDEntropyIsNotMonotonicLikeUlidMake(t *testing.T) {
	const burst = 2000
	const minGroup = 8

	largestSameMillisecondGroup := func(mint func() string) []ulid.ULID {
		byMillisecond := map[uint64][]ulid.ULID{}
		for i := 0; i < burst; i++ {
			parsed, err := ulid.ParseStrict(mint())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			byMillisecond[parsed.Time()] = append(byMillisecond[parsed.Time()], parsed)
		}
		var largest []ulid.ULID
		for _, group := range byMillisecond {
			if len(group) > len(largest) {
				largest = group
			}
		}
		return largest
	}

	ascending := func(group []ulid.ULID) bool {
		for i := 1; i < len(group); i++ {
			previous, current := group[i-1].Entropy(), group[i].Entropy()
			if bytes.Compare(current, previous) <= 0 {
				return false
			}
		}
		return true
	}

	// Premise: ulid.Make() must be ascending within a millisecond. If this fails,
	// the library changed and the assertion below no longer proves anything.
	makeGroup := largestSameMillisecondGroup(func() string { return ulid.Make().String() })
	if len(makeGroup) < minGroup {
		t.Fatalf("ulid.Make() burst produced a largest same-millisecond group of %d, need %d — cannot establish the premise", len(makeGroup), minGroup)
	}
	if !ascending(makeGroup) {
		t.Fatalf("premise broken: ulid.Make() entropy was not ascending within a millisecond (%d values), so this test can no longer detect a revert", len(makeGroup))
	}

	group := largestSameMillisecondGroup(ULID)
	if len(group) < minGroup {
		t.Fatalf("ULID() burst produced a largest same-millisecond group of %d, need %d", len(group), minGroup)
	}
	if ascending(group) {
		t.Errorf("ULID() entropy was strictly ascending across %d values in one millisecond — that is ulid.Monotonic behaviour, not crypto/rand", len(group))
	}
}

// A seeded math/rand ULID stream is reproducible: seed it twice and you get the
// same values. crypto/rand cannot be reproduced, which is the whole point.
func TestULIDIsNotReproducibleFromASeed(t *testing.T) {
	stream := func(seed int64) []string {
		source := ulid.Monotonic(rand.New(rand.NewSource(seed)), 0)
		fixed := ulid.Timestamp(time.Unix(0, 0))
		out := make([]string, 8)
		for i := range out {
			out[i] = ulid.MustNew(fixed, source).String()
		}
		return out
	}
	if a, b := stream(1), stream(1); !equal(a, b) {
		t.Fatal("premise broken: a seeded math/rand ULID stream should be reproducible")
	}

	seen := map[string]struct{}{}
	for i := 0; i < 256; i++ {
		value := ULID()
		if _, duplicate := seen[value]; duplicate {
			t.Fatalf("ULID() repeated a value after %d draws: %q", i, value)
		}
		seen[value] = struct{}{}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
