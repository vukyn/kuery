package medioa

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// CDNURL builds a direct, tokenless object URL on the PUBLIC R2/CDN origin:
// "{cdnBaseURL}/{rawKey}".
//
// This is the counterpart to PublicURL. PublicURL returns the token-gated read
// endpoint, which resolves an object by minting a short-lived presigned R2 URL
// and 302-redirecting to it — one extra hop, a server-side lookup per hit, and a
// URL that changes every time (so the edge can never cache it). CDNURL points
// straight at the object, so the response is cacheable and never expires.
//
// The tradeoffs are the flip side of that: the URL carries no token, so anyone
// holding it can read the object indefinitely even if its visibility is later
// changed, and the read bypasses the server entirely (no egress accounting).
// Use it for content that is meant to be public and immutable — cover art, HLS
// segments — and PublicURL for anything whose access you may need to revoke.
//
// Returns "" when either argument is empty, so a caller with an unconfigured CDN
// origin can fall back to PublicURL on a falsy result rather than emitting a
// malformed "/{rawKey}".
func CDNURL(cdnBaseURL, rawKey string) string {
	base := strings.TrimRight(strings.TrimSpace(cdnBaseURL), "/")
	key := strings.TrimLeft(strings.TrimSpace(rawKey), "/")
	if base == "" || key == "" {
		return ""
	}
	return base + "/" + key
}

// DeriveRawKey recovers an object's raw storage key from its token-gated public
// URL by issuing a GET that is NOT followed, then reading the key out of the 302
// Location the read endpoint redirects to.
//
// It exists to backfill a raw key that was never captured at upload time —
// either because the upload predates IncludeRawKey or because the key was
// discarded by the caller. Only public objects can be resolved this way: a
// private object answers 404 at the token-read endpoint, and an upload that did
// not opt into a raw key still has one server-side, so the probe is the only way
// to learn it after the fact.
//
// bucketPrefix is the segment of the redirect path that precedes the key (e.g.
// "rainy-prod/"); everything after it, query string stripped, is the key. Pass a
// client whose CheckRedirect returns http.ErrUseLastResponse — a client that
// follows redirects lands on the object itself and exposes no Location, so the
// probe would silently return "".
//
// A response that is not a redirect, or whose Location does not contain
// bucketPrefix, yields ("", nil): "could not determine" is an expected outcome
// here, not a failure. Only transport-level problems return an error. Callers
// backfilling in bulk should throttle — the token-read endpoint is rate-limited.
func DeriveRawKey(ctx context.Context, client *http.Client, publicURL, bucketPrefix string) (string, error) {
	if client == nil || publicURL == "" || bucketPrefix == "" {
		return "", nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, publicURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	location := response.Header.Get("Location")
	if location == "" {
		return "", nil
	}
	return RawKeyFromLocation(location, bucketPrefix), nil
}

// RawKeyFromLocation extracts the raw storage key from a presigned-URL redirect
// target. Split out from DeriveRawKey so the parsing is testable without a
// server, and reusable by a caller that already holds a Location header.
//
// Returns "" when the prefix is absent.
func RawKeyFromLocation(location, bucketPrefix string) string {
	// Prefer the parsed path so the query string (the presign signature) is
	// dropped; fall back to the raw value if it is not a well-formed URL, since
	// the prefix split alone is still enough to find the key.
	path := location
	if parsed, err := url.Parse(location); err == nil && parsed.Path != "" {
		path = parsed.Path
	}

	_, rawKey, found := strings.Cut(path, bucketPrefix)
	if !found {
		return ""
	}
	// Defensive: strip a leftover query if we fell back to the unparsed Location.
	if question := strings.IndexByte(rawKey, '?'); question >= 0 {
		rawKey = rawKey[:question]
	}
	return strings.TrimSpace(rawKey)
}

// NoRedirectClient returns a shallow copy of base whose CheckRedirect captures
// redirects instead of following them, as DeriveRawKey requires. base may be
// nil, in which case a client with the package default timeout is used.
//
// Copying rather than mutating matters: flipping CheckRedirect on a shared
// client would silently break every other caller's redirect handling.
func NoRedirectClient(base *http.Client) *http.Client {
	copied := &http.Client{Timeout: defaultTimeout}
	if base != nil {
		copied.Transport = base.Transport
		copied.Jar = base.Jar
		copied.Timeout = base.Timeout
	}
	copied.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return copied
}
