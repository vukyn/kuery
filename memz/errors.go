package memz

import (
	"errors"
	"fmt"
)

// Sentinel errors mapped from the server's HTTP status code. Consumers test for
// them with errors.Is. Each is also wrapped by an *APIError (see classify), so
// errors.As(err, &apiErr) still exposes the status, envelope code, and message.
var (
	// ErrUnauthorized maps to HTTP 401: a missing, malformed, or unrecognized
	// API key on the Authorization header.
	ErrUnauthorized = errors.New("memz: unauthorized")
	// ErrForbidden maps to HTTP 403.
	ErrForbidden = errors.New("memz: forbidden")
	// ErrNotFound maps to HTTP 404.
	ErrNotFound = errors.New("memz: not found")
	// ErrTooManyRequests maps to HTTP 429: the per-second rate limit was hit.
	ErrTooManyRequests = errors.New("memz: too many requests")
)

// APIError carries the detail of a failed API call. StatusCode is the HTTP
// status; Code and Message come from the response envelope ({code, message})
// when present (some error paths, e.g. 401, return a non-enveloped body, in
// which case Code is 0 and Message is a best-effort description).
type APIError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("memz: api error (status %d, code %d): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("memz: api error (status %d, code %d)", e.StatusCode, e.Code)
}

// Unwrap returns the matching sentinel for the status code so errors.Is works
// against ErrUnauthorized/ErrForbidden/ErrNotFound/ErrTooManyRequests. Statuses
// without a dedicated sentinel unwrap to nil, so only errors.As(&APIError{})
// exposes their detail.
func (e *APIError) Unwrap() error {
	switch e.StatusCode {
	case statusUnauthorized:
		return ErrUnauthorized
	case statusForbidden:
		return ErrForbidden
	case statusNotFound:
		return ErrNotFound
	case statusTooManyRequests:
		return ErrTooManyRequests
	default:
		return nil
	}
}

// HTTP status codes the SDK maps, named locally to avoid importing net/http
// solely for constants (the package already imports net/http in client.go, but
// keeping these local keeps the mapping table self-contained and explicit).
const (
	statusUnauthorized    = 401
	statusForbidden       = 403
	statusNotFound        = 404
	statusTooManyRequests = 429
)
