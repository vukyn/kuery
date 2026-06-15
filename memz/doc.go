// Package memz is a typed, dependency-clean Go SDK for the memz service's
// API (the per-client in-memory cache + usage-tracking surface).
//
// It speaks the wire contract exactly as the memz server exposes it under
// {BaseURL}/api/v1/*: a cache surface (GET/POST/DELETE /caches, GET
// /caches/:key) and a read-only usage surface (GET /usages). Every server
// response is the kuery http/base envelope ({code, message, data}); the SDK
// unwraps data on success and maps non-2xx responses to the typed errors in
// errors.go.
//
// Auth: unlike the medioa SDK (which uses an X-API-Key header), memz reads the
// raw API key directly from the Authorization header — no "Bearer" prefix, no
// X-API-Key. The SDK therefore sets Authorization to the raw Config.APIKey on
// every call. The server SHA-256-hashes the presented value and derives the
// client id from the matched key, so the SDK never sends a client_id.
//
// The package imports only the standard library so kuery stays light and the
// SDK remains the single source of truth for the contract — no memz-server
// types leak in. Consumers construct a Client with New and call Get / Set /
// Del / List / GetUsage.
//
// Server-to-server callers should set Config.BaseURL to a 127.0.0.1:<port>
// address rather than a *.local hostname: Go's resolver tries mDNS first for
// *.local names, adding a ~5s stall per request.
package memz
