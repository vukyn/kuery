package ratelimit

import "net/http"

// CountAuthFailures charges only a rejected credential (401) or a refused
// authorization (403). It is the default predicate and the whole point of this
// package: everything else a login endpoint can answer is either the server's
// fault or the client's malformed request, and charging for those turns an outage
// or a buggy client into a lockout.
//
// 429 is deliberately NOT counted, so an inner limiter tier's block never spends
// an outer tier's budget. See the package doc.
func CountAuthFailures(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// CountClientErrors charges any 4xx except 408 (request timeout — a transport
// problem, not an attempt) and 429 (another limiter already refused this request;
// counting it would let one blocked caller drain a shared budget).
//
// Use it for an endpoint where a malformed request IS abuse worth throttling. For
// authentication, prefer CountAuthFailures.
func CountClientErrors(status int) bool {
	if status < http.StatusBadRequest || status > 499 {
		return false
	}
	return status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}
