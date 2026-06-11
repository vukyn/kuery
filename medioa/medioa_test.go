package medioa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	c, err := New(Config{BaseURL: base, APIKey: "mk_test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidation(t *testing.T) {
	if _, err := New(Config{APIKey: "mk_x"}); err == nil {
		t.Fatal("expected error for empty BaseURL")
	}
	if _, err := New(Config{BaseURL: "http://x"}); err == nil {
		t.Fatal("expected error for empty APIKey")
	}
	c, err := New(Config{BaseURL: "http://x/", APIKey: "mk_x"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != "http://x" {
		t.Fatalf("trailing slash not trimmed: %q", c.baseURL)
	}
}

func TestPublicURL(t *testing.T) {
	got := PublicURL("http://127.0.0.1:8082", "tok123")
	want := "http://127.0.0.1:8082/api/v1/public/objects/tok123"
	if got != want {
		t.Fatalf("PublicURL = %q, want %q", got, want)
	}
	// Trailing slash on base must not double up.
	got = PublicURL("http://127.0.0.1:8082/", "tok123")
	if got != want {
		t.Fatalf("PublicURL with trailing slash = %q, want %q", got, want)
	}
}

func TestUploadSuccess(t *testing.T) {
	want := UploadResult{
		FileID:   "f1",
		Token:    "t1",
		URL:      "http://127.0.0.1:8082/api/v1/public/objects/t1",
		FileName: "hello.txt",
		FileSize: 5,
		Ext:      "txt",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathUpload {
			t.Errorf("path = %q, want %q", r.URL.Path, pathUpload)
		}
		if got := r.Header.Get(apiKeyHeader); got != "mk_test" {
			t.Errorf("X-API-Key = %q, want mk_test", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
		} else {
			defer f.Close()
			body, _ := io.ReadAll(f)
			if string(body) != "hello" {
				t.Errorf("file body = %q, want hello", body)
			}
			if hdr.Filename != "hello.txt" {
				t.Errorf("filename = %q", hdr.Filename)
			}
		}
		if got := r.FormValue("path"); got != "docs/sub" {
			t.Errorf("path field = %q, want docs/sub", got)
		}
		if got := r.FormValue("ext"); got != "txt" {
			t.Errorf("ext field = %q, want txt", got)
		}
		okEnvelope(w, want)
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	got, err := c.Upload(context.Background(), UploadInput{
		File:     strings.NewReader("hello"),
		FileName: "hello.txt",
		Ext:      "txt",
		Path:     "docs/sub",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if *got != want {
		t.Fatalf("result = %+v, want %+v", *got, want)
	}
}

func TestUploadContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		_, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		if ct := hdr.Header.Get("Content-Type"); ct != "image/png" {
			t.Errorf("part Content-Type = %q, want image/png", ct)
		}
		okEnvelope(w, UploadResult{FileID: "f"})
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	if _, err := c.Upload(context.Background(), UploadInput{
		File:        strings.NewReader("x"),
		FileName:    "a.png",
		ContentType: "image/png",
	}); err != nil {
		t.Fatalf("Upload: %v", err)
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
		{http.StatusRequestEntityTooLarge, ErrTooLarge, false},
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
			_, err := c.Upload(context.Background(), UploadInput{
				File: strings.NewReader("x"), FileName: "f",
			})
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
	_, err := c.Upload(context.Background(), UploadInput{File: strings.NewReader("x"), FileName: "f"})
	if err == nil {
		t.Fatal("expected error")
	}
	// 500 has no sentinel — must NOT match any of them.
	for _, s := range []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrTooLarge} {
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

func TestUploadChunkedFlow(t *testing.T) {
	// 20 bytes of content, 8-byte chunks => stages of 8, 8, 4, then commit.
	content := "abcdefghijklmnopqrst" // 20 bytes
	const chunkSize = 8

	var (
		stageCalls   int
		commitCalls  int
		seenParts    [][]byte
		seenFileIDs  []string
		committedID  string
		committedPth string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		switch r.URL.Path {
		case pathUploadStage:
			stageCalls++
			f, _, err := r.FormFile("chunk")
			if err != nil {
				t.Fatalf("FormFile chunk: %v", err)
			}
			defer f.Close()
			body, _ := io.ReadAll(f)
			seenParts = append(seenParts, body)
			seenFileIDs = append(seenFileIDs, r.FormValue("file_id"))
			// Server mints the id on the first chunk and echoes it after.
			okEnvelope(w, uploadChunkResult{
				ChunkID:    fmt.Sprintf("chunk-%d", stageCalls),
				FileID:     "FILE-XYZ",
				PartNumber: int32(stageCalls),
			})
		case pathUploadCommit:
			commitCalls++
			committedID = r.FormValue("file_id")
			committedPth = r.FormValue("path")
			okEnvelope(w, UploadResult{
				FileID:   committedID,
				Token:    "tok",
				URL:      PublicURL("http://srv", "tok"),
				FileName: "blob",
				FileSize: int64(len(content)),
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	res, err := c.UploadChunked(context.Background(), UploadInput{
		File:     strings.NewReader(content),
		FileName: "blob",
		Path:     "media/clips",
	}, chunkSize)
	if err != nil {
		t.Fatalf("UploadChunked: %v", err)
	}

	if stageCalls != 3 {
		t.Fatalf("stageCalls = %d, want 3", stageCalls)
	}
	if commitCalls != 1 {
		t.Fatalf("commitCalls = %d, want 1", commitCalls)
	}

	// file_id threading: first stage empty, subsequent stages carry the id.
	if seenFileIDs[0] != "" {
		t.Fatalf("first stage file_id = %q, want empty", seenFileIDs[0])
	}
	for i := 1; i < len(seenFileIDs); i++ {
		if seenFileIDs[i] != "FILE-XYZ" {
			t.Fatalf("stage %d file_id = %q, want FILE-XYZ", i, seenFileIDs[i])
		}
	}
	if committedID != "FILE-XYZ" {
		t.Fatalf("commit file_id = %q, want FILE-XYZ", committedID)
	}
	if committedPth != "media/clips" {
		t.Fatalf("commit path = %q, want media/clips", committedPth)
	}

	// Reassembled chunks must equal the original content, in order.
	var reassembled bytes.Buffer
	for _, p := range seenParts {
		reassembled.Write(p)
	}
	if reassembled.String() != content {
		t.Fatalf("reassembled = %q, want %q", reassembled.String(), content)
	}
	if len(seenParts[0]) != 8 || len(seenParts[1]) != 8 || len(seenParts[2]) != 4 {
		t.Fatalf("chunk sizes = %d/%d/%d, want 8/8/4", len(seenParts[0]), len(seenParts[1]), len(seenParts[2]))
	}

	if res.Token != "tok" || res.FileID != "FILE-XYZ" {
		t.Fatalf("commit result = %+v", res)
	}
}

func TestUploadChunkedExactBoundary(t *testing.T) {
	// Content length is an exact multiple of chunkSize (no short final chunk).
	content := "abcdefghijklmnop" // 16 bytes
	const chunkSize = 8

	var stageCalls, commitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		switch r.URL.Path {
		case pathUploadStage:
			stageCalls++
			okEnvelope(w, uploadChunkResult{FileID: "ID", PartNumber: int32(stageCalls)})
		case pathUploadCommit:
			commitCalls++
			okEnvelope(w, UploadResult{FileID: "ID"})
		}
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	if _, err := c.UploadChunked(context.Background(), UploadInput{
		File: strings.NewReader(content), FileName: "x",
	}, chunkSize); err != nil {
		t.Fatalf("UploadChunked: %v", err)
	}
	if stageCalls != 2 {
		t.Fatalf("stageCalls = %d, want 2", stageCalls)
	}
	if commitCalls != 1 {
		t.Fatalf("commitCalls = %d, want 1", commitCalls)
	}
}

func TestUploadChunkedDefaultChunkSize(t *testing.T) {
	// Small content with chunkSize <= 0 should still upload in a single stage.
	var stageCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		switch r.URL.Path {
		case pathUploadStage:
			stageCalls++
			okEnvelope(w, uploadChunkResult{FileID: "ID", PartNumber: 1})
		case pathUploadCommit:
			okEnvelope(w, UploadResult{FileID: "ID"})
		}
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	if _, err := c.UploadChunked(context.Background(), UploadInput{
		File: strings.NewReader("tiny"), FileName: "x",
	}, 0); err != nil {
		t.Fatalf("UploadChunked: %v", err)
	}
	if stageCalls != 1 {
		t.Fatalf("stageCalls = %d, want 1", stageCalls)
	}
}

func TestUploadNilFile(t *testing.T) {
	c := newClient(t, "http://127.0.0.1:9")
	if _, err := c.Upload(context.Background(), UploadInput{}); err == nil {
		t.Fatal("expected error for nil File")
	}
	if _, err := c.UploadChunked(context.Background(), UploadInput{}, 8); err == nil {
		t.Fatal("expected error for nil File")
	}
}
