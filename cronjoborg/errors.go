package cronjoborg

import (
	"errors"
	"fmt"
)

// Sentinel errors mapped from the API's HTTP status code. Consumers test for
// them with errors.Is; every one of them is reachable through an *APIError, so
// errors.As(err, &apiErr) still exposes the status and the response body.
var (
	// ErrBadRequest maps to HTTP 400: invalid request or invalid input data.
	ErrBadRequest = errors.New("cronjoborg: bad request")
	// ErrUnauthorized maps to HTTP 401: the API key itself is invalid.
	ErrUnauthorized = errors.New("cronjoborg: unauthorized")
	// ErrForbidden maps to HTTP 403: the key is valid but cannot be used from
	// this origin (its IP allowlist rejected the caller). Minting a new key does
	// not help — allowlist the origin, or clear the restriction.
	ErrForbidden = errors.New("cronjoborg: api key not allowed from this origin")
	// ErrNotFound maps to HTTP 404.
	ErrNotFound = errors.New("cronjoborg: not found")
	// ErrConflict maps to HTTP 409: the resource already exists (folder titles
	// must be unique within an account).
	ErrConflict = errors.New("cronjoborg: conflict")
	// ErrTooManyRequests maps to HTTP 429, which the API returns for THREE
	// different conditions: API-key quota exhausted, resource quota exhausted,
	// and rate limit exceeded. Only the response body distinguishes them, so
	// back-off logic that branches on this sentinel alone can spin against a
	// daily quota that will not refill until tomorrow. Read APIError.Message
	// and APIError.RetryAfter before retrying.
	ErrTooManyRequests = errors.New("cronjoborg: quota or rate limit exceeded")
	// ErrServer maps to HTTP 5xx.
	ErrServer = errors.New("cronjoborg: server error")
)

// APIError carries the detail of a failed API call: the HTTP status, the raw
// response body (truncated), and the Retry-After hint when the API sent one.
// The API does not document an error payload shape, so Message is the body as
// received rather than a parsed field.
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Message is the response body, trimmed and truncated to maxErrorBodyBytes.
	Message string
	// RetryAfter is the Retry-After response header verbatim, empty when absent.
	// It is a hint for a rate limit; it says nothing about a quota.
	RetryAfter string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("cronjoborg: api error (status %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("cronjoborg: api error (status %d)", e.StatusCode)
}

// Unwrap returns the matching sentinel so errors.Is works against the status.
// Statuses without a dedicated sentinel unwrap to nil, so only
// errors.As(&APIError{}) exposes their detail.
func (e *APIError) Unwrap() error {
	switch {
	case e.StatusCode == statusBadRequest:
		return ErrBadRequest
	case e.StatusCode == statusUnauthorized:
		return ErrUnauthorized
	case e.StatusCode == statusForbidden:
		return ErrForbidden
	case e.StatusCode == statusNotFound:
		return ErrNotFound
	case e.StatusCode == statusConflict:
		return ErrConflict
	case e.StatusCode == statusTooManyRequests:
		return ErrTooManyRequests
	case e.StatusCode >= 500:
		return ErrServer
	default:
		return nil
	}
}

// HTTP status codes the SDK maps, named locally to keep the mapping table
// self-contained and explicit.
const (
	statusBadRequest      = 400
	statusUnauthorized    = 401
	statusForbidden       = 403
	statusNotFound        = 404
	statusConflict        = 409
	statusTooManyRequests = 429
)
