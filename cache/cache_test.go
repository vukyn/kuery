package cache

import (
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	c.Set("key", "value", time.Minute)
	got, ok := c.Get("key")
	if !ok || got != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", got, ok)
	}
}

func TestGetMissing(t *testing.T) {
	c := NewCache[string, int]()
	defer c.Close()

	got, ok := c.Get("absent")
	if ok || got != 0 {
		t.Fatalf("expected zero value and false, got (%d, %v)", got, ok)
	}
}

func TestDelete(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	c.Set("key", "value", time.Minute)
	c.Delete("key")
	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	c.Set("short", "value", 20*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("short"); ok {
		t.Fatal("expected entry to expire after its TTL")
	}
}

func TestStructValue(t *testing.T) {
	type session struct{ AppServiceID string }
	c := NewCache[string, session]()
	defer c.Close()

	c.Set("sid", session{AppServiceID: "app_1"}, time.Minute)
	got, ok := c.Get("sid")
	if !ok || got.AppServiceID != "app_1" {
		t.Fatalf("expected struct round-trip, got (%+v, %v)", got, ok)
	}
}

func TestPermanentEntry(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	c.Set("perm", "value", 0)
	time.Sleep(80 * time.Millisecond)
	got, ok := c.Get("perm")
	if !ok || got != "value" {
		t.Fatalf("expected permanent entry to persist, got (%q, %v)", got, ok)
	}

	c.Set("perm-neg", "value", -time.Second)
	if _, ok := c.Get("perm-neg"); !ok {
		t.Fatal("expected negative TTL to be treated as permanent")
	}
}

func TestHas(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	if c.Has("missing") {
		t.Fatal("expected Has to be false for absent key")
	}
	c.Set("present", "value", time.Minute)
	if !c.Has("present") {
		t.Fatal("expected Has to be true for present key")
	}
}

func TestSetNX(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	if !c.SetNX("key", "first", time.Minute) {
		t.Fatal("expected SetNX to return true on absent key")
	}
	if c.SetNX("key", "second", time.Minute) {
		t.Fatal("expected SetNX to return false on present key")
	}
	got, ok := c.Get("key")
	if !ok || got != "first" {
		t.Fatalf("expected SetNX not to overwrite, got (%q, %v)", got, ok)
	}
}

func TestRange(t *testing.T) {
	c := NewCache[string, int]()
	defer c.Close()

	c.Set("a", 1, time.Minute)
	c.Set("b", 2, time.Minute)
	c.Set("c", 3, time.Minute)

	visited := map[string]int{}
	c.Range(func(key string, value int) bool {
		visited[key] = value
		return true
	})
	if len(visited) != 3 || visited["a"] != 1 || visited["b"] != 2 || visited["c"] != 3 {
		t.Fatalf("expected Range to visit all entries, got %v", visited)
	}

	count := 0
	c.Range(func(key string, value int) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("expected Range to stop early after one entry, visited %d", count)
	}
}

func TestKeys(t *testing.T) {
	c := NewCache[string, int]()
	defer c.Close()

	c.Set("a", 1, time.Minute)
	c.Set("b", 2, time.Minute)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d (%v)", len(keys), keys)
	}
	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}
	if !found["a"] || !found["b"] {
		t.Fatalf("expected keys a and b, got %v", keys)
	}
}

func TestGetWithTTL(t *testing.T) {
	c := NewCache[string, string]()
	defer c.Close()

	// Missing key.
	if _, _, ok := c.GetWithTTL("missing"); ok {
		t.Fatal("expected ok=false for missing key")
	}

	// Permanent entry reports remaining TTL of 0.
	c.Set("perm", "value", 0)
	got, ttl, ok := c.GetWithTTL("perm")
	if !ok || got != "value" || ttl != 0 {
		t.Fatalf("expected permanent entry (value, 0, true), got (%q, %v, %v)", got, ttl, ok)
	}

	// TTL'd entry reports a positive remaining TTL near the set TTL.
	c.Set("ttld", "value", time.Minute)
	got, ttl, ok = c.GetWithTTL("ttld")
	if !ok || got != "value" {
		t.Fatalf("expected (value, _, true), got (%q, %v, %v)", got, ttl, ok)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("expected remaining TTL in (0, 1m], got %v", ttl)
	}
}
