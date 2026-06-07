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
