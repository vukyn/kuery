package medioa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newClientWithLimitsTTL mirrors newClient but sets the Limits cache TTL, which
// the shared helper leaves at its default.
func newClientWithLimitsTTL(t *testing.T, baseURL string, ttl time.Duration) *Client {
	t.Helper()
	client, err := New(Config{BaseURL: baseURL, APIKey: "mk_test", LimitsTTL: ttl})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

// A declared TotalSize must open the upload with /init BEFORE any part is sent —
// that ordering is the whole value of the field, since the server cannot see a
// declared size in a stage call until it has already received the 8 MiB part
// beside it.
func TestUploadChunkedWithTotalSizeInitsBeforeStaging(t *testing.T) {
	var mutex sync.Mutex
	calls := []string{}
	var declaredSize, stagedFileID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		calls = append(calls, r.URL.Path)
		mutex.Unlock()

		switch r.URL.Path {
		case pathUploadInit:
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("init: parse form: %v", err)
			}
			declaredSize = r.FormValue("total_size")
			okEnvelope(w, map[string]any{"file_id": "file-from-init"})
		case pathUploadStage:
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Errorf("stage: parse form: %v", err)
			}
			stagedFileID = r.FormValue("file_id")
			okEnvelope(w, uploadChunkResult{ChunkID: "etag-1", FileID: "file-from-init", PartNumber: 1})
		case pathUploadCommit:
			okEnvelope(w, UploadResult{FileID: "file-from-init", Token: "tok", URL: "u", FileSize: 9})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	payload := strings.NewReader("123456789")
	result, err := newClientWithLimitsTTL(t, srv.URL, 0).UploadChunked(context.Background(), UploadInput{
		File:      payload,
		FileName:  "song.mp3",
		Ext:       "mp3",
		TotalSize: 9,
	}, 1<<20)
	if err != nil {
		t.Fatalf("UploadChunked: %v", err)
	}

	if len(calls) == 0 || calls[0] != pathUploadInit {
		t.Fatalf("init must be the FIRST call, got %v", calls)
	}
	if declaredSize != "9" {
		t.Errorf("total_size = %q, want \"9\"", declaredSize)
	}
	// The id minted by /init has to be threaded into the first stage call; sending
	// an empty one there would make the server open a SECOND object and orphan the
	// first.
	if stagedFileID != "file-from-init" {
		t.Errorf("first stage sent file_id = %q, want the id from /init", stagedFileID)
	}
	if result.FileID != "file-from-init" {
		t.Errorf("result FileID = %q", result.FileID)
	}
}

// Zero TotalSize must leave the legacy flow byte-for-byte alone: no /init call at
// all, and an empty file_id on the first stage so the server mints it. A consumer
// on an older server depends on this.
func TestUploadChunkedWithoutTotalSizeSkipsInit(t *testing.T) {
	var mutex sync.Mutex
	initCalls := 0
	firstStageFileID := ""
	stageCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()

		switch r.URL.Path {
		case pathUploadInit:
			initCalls++
			w.WriteHeader(http.StatusNotFound)
		case pathUploadStage:
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Errorf("stage: parse form: %v", err)
			}
			if stageCount == 0 {
				firstStageFileID = r.FormValue("file_id")
			}
			stageCount++
			okEnvelope(w, uploadChunkResult{ChunkID: "etag", FileID: "minted-by-stage", PartNumber: 1})
		case pathUploadCommit:
			okEnvelope(w, UploadResult{FileID: "minted-by-stage", Token: "tok", URL: "u"})
		}
	}))
	defer srv.Close()

	_, err := newClientWithLimitsTTL(t, srv.URL, 0).UploadChunked(context.Background(), UploadInput{
		File:     strings.NewReader("abc"),
		FileName: "cover.jpg",
	}, 1<<20)
	if err != nil {
		t.Fatalf("UploadChunked: %v", err)
	}

	if initCalls != 0 {
		t.Errorf("/init was called %d times without a declared TotalSize", initCalls)
	}
	if firstStageFileID != "" {
		t.Errorf("legacy first stage sent file_id = %q, want empty so the server mints it", firstStageFileID)
	}
}

// A 404 from /init means the server predates this SDK. It must surface, not fall
// back: a silent fallback produces a working upload that has quietly lost the
// early rejection, hiding the deploy-order mistake instead of reporting it.
func TestUploadChunkedInitNotFoundIsNotSilentlyDowngraded(t *testing.T) {
	stageCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case pathUploadInit:
			w.WriteHeader(http.StatusNotFound)
		default:
			stageCalls++
			okEnvelope(w, map[string]any{"file_id": "x"})
		}
	}))
	defer srv.Close()

	_, err := newClientWithLimitsTTL(t, srv.URL, 0).UploadChunked(context.Background(), UploadInput{
		File:      strings.NewReader("abc"),
		TotalSize: 3,
	}, 1<<20)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound from /init, got %v", err)
	}
	if stageCalls != 0 {
		t.Errorf("nothing must be staged after a failed init, got %d calls", stageCalls)
	}
}

// An over-cap declaration is rejected at /init as a 413, before a single part is
// sent — the outcome the whole feature exists for.
func TestUploadChunkedInitTooLargeStagesNothing(t *testing.T) {
	stageCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathUploadInit {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			fmt.Fprint(w, `{"code":413,"message":"file size too large (max: 50MB)"}`)
			return
		}
		stageCalls++
	}))
	defer srv.Close()

	_, err := newClientWithLimitsTTL(t, srv.URL, 0).UploadChunked(context.Background(), UploadInput{
		File:      strings.NewReader("abc"),
		TotalSize: 200 << 20,
	}, 1<<20)

	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Message, "max: 50MB") {
		t.Errorf("the cap must survive into the error message, got %v", err)
	}
	if stageCalls != 0 {
		t.Errorf("nothing must be staged, got %d calls", stageCalls)
	}
}

func TestLimitsFetchesAndCaches(t *testing.T) {
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathLimits {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.Header.Get(apiKeyHeader) == "" {
			t.Error("the API key header must be attached to the limits GET")
		}
		fetches++
		okEnvelope(w, Limits{MaxFileSizeBytes: 50 << 20})
	}))
	defer srv.Close()

	client := newClientWithLimitsTTL(t, srv.URL, time.Hour)

	for attempt := 0; attempt < 3; attempt++ {
		limits, err := client.Limits(context.Background())
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if limits.MaxFileSizeBytes != 50<<20 {
			t.Fatalf("attempt %d: cap = %d", attempt, limits.MaxFileSizeBytes)
		}
	}
	if fetches != 1 {
		t.Errorf("expected 1 fetch across 3 calls, got %d", fetches)
	}
}

// The cache is a TTL, not a fetch-once, so an operator who raises the server's
// cap reaches consumers without restarting them. Aged via the injected clock
// rather than a sleep.
func TestLimitsRefetchesAfterTTL(t *testing.T) {
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		okEnvelope(w, Limits{MaxFileSizeBytes: int64(fetches) * 1024})
	}))
	defer srv.Close()

	client := newClientWithLimitsTTL(t, srv.URL, time.Minute)
	base := time.Unix(1_700_000_000, 0)
	client.now = func() time.Time { return base }

	first, err := client.Limits(context.Background())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	client.now = func() time.Time { return base.Add(time.Minute + time.Second) }
	second, err := client.Limits(context.Background())
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if fetches != 2 {
		t.Errorf("expected a refetch after the TTL, got %d fetches", fetches)
	}
	if first.MaxFileSizeBytes == second.MaxFileSizeBytes {
		t.Errorf("the refetched value must replace the cached one, both = %d", first.MaxFileSizeBytes)
	}
}

// A failure must not be cached: a blip would otherwise pin the consumer to a
// pre-flight it cannot perform for the rest of the TTL.
func TestLimitsDoesNotCacheFailures(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		okEnvelope(w, Limits{MaxFileSizeBytes: 1024})
	}))
	defer srv.Close()

	client := newClientWithLimitsTTL(t, srv.URL, time.Hour)

	if _, err := client.Limits(context.Background()); err == nil {
		t.Fatal("expected the first call to fail")
	}
	limits, err := client.Limits(context.Background())
	if err != nil {
		t.Fatalf("the second call must retry, got %v", err)
	}
	if limits.MaxFileSizeBytes != 1024 {
		t.Errorf("cap = %d, want 1024", limits.MaxFileSizeBytes)
	}
}

// A caller mutating the returned struct must not corrupt what the next caller
// reads out of the cache.
func TestLimitsReturnsACopy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		okEnvelope(w, Limits{MaxFileSizeBytes: 1024})
	}))
	defer srv.Close()

	client := newClientWithLimitsTTL(t, srv.URL, time.Hour)

	// Three calls, not two: the FIRST is the cold fetch, which returns a copy of a
	// local. Only the second onwards read the cache, so mutating the first would
	// never touch the aliasing this pins — the earlier two-call version of this
	// test passed even with the copy removed.
	if _, err := client.Limits(context.Background()); err != nil {
		t.Fatalf("cold fetch: %v", err)
	}

	cached, err := client.Limits(context.Background())
	if err != nil {
		t.Fatalf("first cached read: %v", err)
	}
	cached.MaxFileSizeBytes = 1

	again, err := client.Limits(context.Background())
	if err != nil {
		t.Fatalf("second cached read: %v", err)
	}
	if again.MaxFileSizeBytes != 1024 {
		t.Errorf("the cache was corrupted by the caller: %d", again.MaxFileSizeBytes)
	}
}

// Zero is the server's no-cap sentinel and must allow everything. Reading it as
// a ceiling would reject every file on a server with no configured cap.
func TestLimitsAllows(t *testing.T) {
	cases := []struct {
		name  string
		cap   int64
		size  int64
		allow bool
	}{
		{name: "under the cap", cap: 50 << 20, size: 10 << 20, allow: true},
		{name: "exactly at the cap", cap: 50 << 20, size: 50 << 20, allow: true},
		{name: "one byte over", cap: 50 << 20, size: (50 << 20) + 1, allow: false},
		{name: "no cap configured allows a huge file", cap: 0, size: 500 << 20, allow: true},
		{name: "a negative cap is still no cap", cap: -1, size: 500 << 20, allow: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Limits{MaxFileSizeBytes: tc.cap}).Allows(tc.size); got != tc.allow {
				t.Errorf("Allows(cap=%d, size=%d) = %v, want %v", tc.cap, tc.size, got, tc.allow)
			}
		})
	}
}
