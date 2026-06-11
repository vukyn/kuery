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
// Upload / UploadChunked, or build a read URL with PublicURL.
//
// Server-to-server callers should set Config.BaseURL to a 127.0.0.1:<port>
// address rather than a *.local hostname: Go's resolver tries mDNS first for
// *.local names, adding a ~5s stall per request.
package medioa
