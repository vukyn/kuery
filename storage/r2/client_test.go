package r2

import (
	"context"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	client, err := New(Config{
		Endpoint:        "https://account.r2.cloudflarestorage.com",
		AccessKeyID:     "test-access-key-id",
		SecretAccessKey: "test-secret-access-key",
		Bucket:          "kioku",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// The signature is computed locally by the SDK, so this needs no network.
func TestPresignPutSignsTheObject(t *testing.T) {
	client := testClient(t)
	url, err := client.PresignPut(context.Background(), "media/u1/m1/320.jpg", "image/jpeg", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	for _, want := range []string{"kioku", "media/u1/m1/320.jpg", "X-Amz-Signature", "X-Amz-Expires"} {
		if !strings.Contains(url, want) {
			t.Fatalf("presigned url is missing %q: %s", want, url)
		}
	}
}

// The content type is signed, not merely suggested: an upload whose header
// disagrees must be rejected by R2 rather than silently stored.
func TestPresignPutSignsTheContentType(t *testing.T) {
	client := testClient(t)
	url, err := client.PresignPut(context.Background(), "media/u1/m1/320.jpg", "image/jpeg", time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	if !strings.Contains(url, "content-type") && !strings.Contains(url, "Content-Type") {
		t.Fatalf("content-type is not part of the signature: %s", url)
	}
}

func TestPresignPutRejectsEmptyArguments(t *testing.T) {
	client := testClient(t)
	if _, err := client.PresignPut(context.Background(), "", "image/jpeg", time.Minute); err == nil {
		t.Fatal("expected an error for an empty key")
	}
	if _, err := client.PresignPut(context.Background(), "k", "", time.Minute); err == nil {
		t.Fatal("expected an error for an empty content type")
	}
}

func TestStatRejectsEmptyKey(t *testing.T) {
	client := testClient(t)
	if _, err := client.Stat(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty key")
	}
}
