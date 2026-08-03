package medioa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCDNURL(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		rawKey string
		want   string
	}{
		{"joins", "https://media.test", "covers/a.jpg", "https://media.test/covers/a.jpg"},
		{"trims trailing slash on base", "https://media.test/", "covers/a.jpg", "https://media.test/covers/a.jpg"},
		{"trims leading slash on key", "https://media.test", "/covers/a.jpg", "https://media.test/covers/a.jpg"},
		{"empty base yields empty", "", "covers/a.jpg", ""},
		{"empty key yields empty", "https://media.test", "", ""},
		{"whitespace base yields empty", "   ", "covers/a.jpg", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CDNURL(tc.base, tc.rawKey); got != tc.want {
				t.Errorf("CDNURL(%q, %q) = %q, want %q", tc.base, tc.rawKey, got, tc.want)
			}
		})
	}
}

func TestRawKeyFromLocation(t *testing.T) {
	cases := []struct {
		name     string
		location string
		prefix   string
		want     string
	}{
		{
			"strips presign query",
			"https://r2.test/rainy-prod/covers/a.jpg?X-Amz-Signature=deadbeef&X-Amz-Expires=600",
			"rainy-prod/",
			"covers/a.jpg",
		},
		{
			"nested key",
			"https://r2.test/rainy-prod/hls/track/1/000.aac",
			"rainy-prod/",
			"hls/track/1/000.aac",
		},
		{
			"prefix absent yields empty",
			"https://r2.test/other-bucket/covers/a.jpg",
			"rainy-prod/",
			"",
		},
		{
			"unparseable location still splits on prefix",
			"://rainy-prod/covers/a.jpg?sig=1",
			"rainy-prod/",
			"covers/a.jpg",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RawKeyFromLocation(tc.location, tc.prefix); got != tc.want {
				t.Errorf("RawKeyFromLocation(%q) = %q, want %q", tc.location, got, tc.want)
			}
		})
	}
}

func TestDeriveRawKey_ReadsRedirectLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://r2.test/rainy-prod/covers/a.jpg?X-Amz-Signature=abc")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	got, err := DeriveRawKey(context.Background(), NoRedirectClient(nil), server.URL, "rainy-prod/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "covers/a.jpg" {
		t.Errorf("got %q, want %q", got, "covers/a.jpg")
	}
}

// A 200 (or any non-redirect) is "could not determine", not an error — the
// backfill caller counts it as a skip and moves on.
func TestDeriveRawKey_NonRedirectIsEmptyNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	got, err := DeriveRawKey(context.Background(), NoRedirectClient(nil), server.URL, "rainy-prod/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDeriveRawKey_NotFoundIsEmptyNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	got, err := DeriveRawKey(context.Background(), NoRedirectClient(nil), server.URL, "rainy-prod/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDeriveRawKey_GuardsEmptyArgs(t *testing.T) {
	if got, err := DeriveRawKey(context.Background(), nil, "https://x/y", "rainy-prod/"); got != "" || err != nil {
		t.Errorf("nil client: got (%q, %v), want empty", got, err)
	}
	if got, err := DeriveRawKey(context.Background(), NoRedirectClient(nil), "", "rainy-prod/"); got != "" || err != nil {
		t.Errorf("empty url: got (%q, %v), want empty", got, err)
	}
	if got, err := DeriveRawKey(context.Background(), NoRedirectClient(nil), "https://x/y", ""); got != "" || err != nil {
		t.Errorf("empty prefix: got (%q, %v), want empty", got, err)
	}
}

// The copy must not follow redirects, and the caller's client must be untouched.
func TestNoRedirectClient_DoesNotMutateBase(t *testing.T) {
	base := &http.Client{}
	copied := NoRedirectClient(base)

	if base.CheckRedirect != nil {
		t.Error("base client's CheckRedirect was mutated")
	}
	if copied.CheckRedirect == nil {
		t.Fatal("copy has no CheckRedirect")
	}
	if err := copied.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Errorf("CheckRedirect returned %v, want ErrUseLastResponse", err)
	}
}
