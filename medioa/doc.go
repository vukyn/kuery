// Package medioa is a typed, dependency-clean Go SDK for medioa2's public
// upload + read API (the 3rd-party "public storage" surface).
//
// It speaks the wire contract exactly as the medioa2 server exposes it under
// {BaseURL}/api/v1/public/storage/* (writes, authenticated with an X-API-Key
// header) and {BaseURL}/api/v1/public/objects/{token} (anonymous, token-gated
// reads). Every server response is the kuery http/base envelope
// ({code, message, data}); the SDK unwraps data on success and maps non-2xx
// responses to the typed errors in errors.go.
//
// The package imports only the standard library so kuery stays light and the
// SDK remains the single source of truth for the contract — no medioa2-server
// types leak in. Consumers (e.g. rainy) construct a Client with New and call
// Upload / UploadPrivate / UploadChunked, delete with Delete, or build a read
// URL with PublicURL.
//
// Reads come in two shapes. PublicURL is the token-gated endpoint: it resolves
// an object by minting a short-lived presigned URL and redirecting to it, which
// costs a hop per hit and produces a URL the edge cannot cache. CDNURL points
// directly at the object on the public CDN origin — cacheable and non-expiring,
// at the cost of being unrevocable and invisible to server-side egress
// accounting. Building a CDNURL needs the object's raw storage key, returned by
// the upload calls via UploadInput.IncludeRawKey; DeriveRawKey recovers it after
// the fact for objects uploaded before the key was captured.
//
// Visibility: Upload (and UploadChunked) store public objects whose URL
// resolves through the anonymous token-read endpoint. UploadPrivate forces
// private visibility — its returned URL is NOT resolvable via public token
// read (a private object responds 404 there by design); reading it back needs
// a human JWT session. UploadPrivate is single-shot only; chunked private
// upload (for files >100MB) is a future addition.
//
// Delete is owner-scoped: it removes an object only when the key owner is its
// creator, otherwise the server responds 404 (ErrNotFound).
//
// Server-to-server callers should set Config.BaseURL to a 127.0.0.1:<port>
// address rather than a *.local hostname: Go's resolver tries mDNS first for
// *.local names, adding a ~5s stall per request.
package medioa
