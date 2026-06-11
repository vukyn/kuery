package medioa

import (
	"errors"
	"fmt"
)

// Sentinel errors mapped from the server's HTTP status code. Consumers test for
// them with errors.Is. Each is also wrapped by an *APIError (see classify), so
// errors.As(err, &apiErr) still exposes the status, envelope code, and message.
var (
	// ErrUnauthorized maps to HTTP 401: a missing, malformed, revoked, or
	// expired X-API-Key, or a key whose owner lost bucket membership.
	ErrUnauthorized = errors.New("medioa: unauthorized")
	// ErrForbidden maps to HTTP 403.
	ErrForbidden = errors.New("medioa: forbidden")
	// ErrNotFound maps to HTTP 404.
	ErrNotFound = errors.New("medioa: not found")
	// ErrTooLarge maps to HTTP 413: the upload exceeded the server's body /
	// size limit.
	ErrTooLarge = errors.New("medioa: request entity too large")
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
		return fmt.Sprintf("medioa: api error (status %d, code %d): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("medioa: api error (status %d, code %d)", e.StatusCode, e.Code)
}

// Unwrap returns the matching sentinel for the status code so errors.Is works
// against ErrUnauthorized/ErrForbidden/ErrNotFound/ErrTooLarge. Statuses
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
	case statusRequestEntityTooLarge:
		return ErrTooLarge
	default:
		return nil
	}
}

// HTTP status codes the SDK maps, named locally to avoid importing net/http
// solely for constants (the package already imports net/http in client.go, but
// keeping these local keeps the mapping table self-contained and explicit).
const (
	statusUnauthorized          = 401
	statusForbidden             = 403
	statusNotFound              = 404
	statusRequestEntityTooLarge = 413
)
