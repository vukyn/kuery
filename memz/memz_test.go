package memz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// okEnvelope writes a 200 base.Response envelope with data.
func okEnvelope(w http.ResponseWriter, data any) {
	raw, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    http.StatusOK,
		"message": "OK",
		"data":    json.RawMessage(raw),
	})
}

func newClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(Config{BaseURL: base, APIKey: "rawkey123"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidation(t *testing.T) {
	if _, err := New(Config{APIKey: "k"}); err == nil {
		t.Fatal("expected error for empty BaseURL")
	}
	if _, err := New(Config{BaseURL: "http://x"}); err == nil {
		t.Fatal("expected error for empty APIKey")
	}
	c, err := New(Config{BaseURL: "http://x/", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != "http://x" {
		t.Fatalf("trailing slash not trimmed: %q", c.baseURL)
	}
}

func TestGetSuccess(t *testing.T) {
	want := GetResult{
		CacheEntry: CacheEntry{Key: "foo", Value: "bar", TTL: 42},
		Exist:      true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != pathCaches+"/foo" {
			t.Errorf("path = %q, want %q", r.URL.Path, pathCaches+"/foo")
		}
		// memz reads the raw key from Authorization (no Bearer, no X-API-Key).
		if got := r.Header.Get("Authorization"); got != "rawkey123" {
			t.Errorf("Authorization = %q, want rawkey123", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Errorf("X-API-Key should be unset, got %q", got)
		}
		okEnvelope(w, want)
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.Get(context.Background(), "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if *got != want {
		t.Fatalf("result = %+v, want %+v", *got, want)
	}
}

func TestGetMissingKey(t *testing.T) {
	if _, err := newClient(t, "http://127.0.0.1:9").Get(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestSetSuccess(t *testing.T) {
	want := SetResult{
		CacheEntry: CacheEntry{Key: "k", Value: "v", TTL: 10},
		Ok:         true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != pathCaches {
			t.Errorf("path = %q, want %q", r.URL.Path, pathCaches)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if got := r.Header.Get("Authorization"); got != "rawkey123" {
			t.Errorf("Authorization = %q, want rawkey123", got)
		}

		body, _ := io.ReadAll(r.Body)
		var sent map[string]any
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// The SDK must NOT send client_id (server derives it from the key).
		if _, ok := sent["client_id"]; ok {
			t.Errorf("body unexpectedly carried client_id: %v", sent)
		}
		if sent["key"] != "k" || sent["value"] != "v" {
			t.Errorf("body key/value = %v", sent)
		}
		if sent["nx"] != true || sent["keep_ttl"] != true {
			t.Errorf("body nx/keep_ttl = %v", sent)
		}
		if sent["ttl"].(float64) != 10 {
			t.Errorf("body ttl = %v, want 10", sent["ttl"])
		}
		okEnvelope(w, want)
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.Set(context.Background(), SetParams{
		Key:     "k",
		Value:   "v",
		NX:      true,
		TTL:     10,
		KeepTTL: true,
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if *got != want {
		t.Fatalf("result = %+v, want %+v", *got, want)
	}
}

func TestSetMissingKey(t *testing.T) {
	if _, err := newClient(t, "http://127.0.0.1:9").Set(context.Background(), SetParams{Value: "v"}); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestDelSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != pathCaches+"/gone" {
			t.Errorf("path = %q, want %q", r.URL.Path, pathCaches+"/gone")
		}
		if got := r.Header.Get("Authorization"); got != "rawkey123" {
			t.Errorf("Authorization = %q, want rawkey123", got)
		}
		okEnvelope(w, DelResult{Ok: true})
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.Del(context.Background(), "gone")
	if err != nil {
		t.Fatalf("Del: %v", err)
	}
	if !got.Ok {
		t.Fatalf("Del Ok = false, want true")
	}
}

func TestListSuccess(t *testing.T) {
	want := ListResult{Items: []ListItem{
		{Key: "a", TTL: -1},
		{Key: "b", TTL: 30},
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != pathCaches {
			t.Errorf("path = %q, want %q", r.URL.Path, pathCaches)
		}
		if got := r.Header.Get("Authorization"); got != "rawkey123" {
			t.Errorf("Authorization = %q, want rawkey123", got)
		}
		okEnvelope(w, want)
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != 2 || got.Items[0] != want.Items[0] || got.Items[1] != want.Items[1] {
		t.Fatalf("List items = %+v, want %+v", got.Items, want.Items)
	}
}

func TestGetUsageSuccess(t *testing.T) {
	want := Usage{
		ID:          "u1",
		ClientID:    "client-a",
		Requests:    7,
		RequestCost: 0.007,
		HourlyCost:  0.005,
		LastUpdated: "2026-06-15T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != pathUsages {
			t.Errorf("path = %q, want %q", r.URL.Path, pathUsages)
		}
		if got := r.Header.Get("Authorization"); got != "rawkey123" {
			t.Errorf("Authorization = %q, want rawkey123", got)
		}
		okEnvelope(w, want)
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if *got != want {
		t.Fatalf("usage = %+v, want %+v", *got, want)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
		// enveloped controls whether the error body is the base.Response
		// envelope (true) or a bare {"error":...} body like the 401 funnel.
		enveloped bool
	}{
		{http.StatusUnauthorized, ErrUnauthorized, false},
		{http.StatusForbidden, ErrForbidden, true},
		{http.StatusNotFound, ErrNotFound, true},
		{http.StatusTooManyRequests, ErrTooManyRequests, false},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.enveloped {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"code":    tc.status,
						"message": "boom",
					})
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"error": "nope"})
				}
			}))
			defer srv.Close()

			c := newClient(t, srv.URL)
			_, err := c.Get(context.Background(), "k")
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tc.sentinel)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("errors.As(*APIError) = false for %v", err)
			}
			if apiErr.StatusCode != tc.status {
				t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if tc.enveloped {
				if apiErr.Code != tc.status || apiErr.Message != "boom" {
					t.Fatalf("envelope detail not captured: %+v", apiErr)
				}
			}
		})
	}
}

func TestGeneralAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 500, "message": "kaboom"})
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	_, err := c.GetUsage(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// 500 has no sentinel — must NOT match any of them.
	for _, s := range []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrTooManyRequests} {
		if errors.Is(err, s) {
			t.Fatalf("500 unexpectedly matched sentinel %v", s)
		}
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As(*APIError) = false")
	}
	if apiErr.StatusCode != 500 || apiErr.Code != 500 || apiErr.Message != "kaboom" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}
