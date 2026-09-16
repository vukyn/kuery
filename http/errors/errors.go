package errors

import (
	"net/http"

	"github.com/vukyn/kuery/http/base"
)

type Error interface {
	Error() string
	Status() int
}

// Coded is an OPTIONAL companion to Error: an error that also carries a
// stable, machine-readable name for the failure ("PLACE_NOT_FOUND").
//
// ⚠️ It is a separate interface on purpose rather than a third method on
// Error. Error is exported and services may implement it themselves; adding a
// method would break every one of them at compile time for a field most of
// them do not set yet. Response writers type-assert for Coded and carry the
// code when it is there.
type Coded interface {
	Code() string
}

type errorImpl struct {
	message string
	status  int
	code    string
}

// WithCode attaches a stable error code to an error built by any constructor
// in this package, leaving its status and message untouched:
//
//	func NewPlaceNotFoundError() error {
//		return errors.WithCode(errors.NotFound("place not found"), "PLACE_NOT_FOUND")
//	}
//
// The code is what a client branches on. The message is prose for a developer
// and stays free to be reworded — which it is not, once a client has started
// matching on it.
//
// An error that is not one of this package's returns unchanged, so a caller
// can wrap unconditionally.
func WithCode(err error, code string) error {
	impl, ok := err.(*errorImpl)
	if !ok || code == "" {
		return err
	}
	return &errorImpl{message: impl.message, status: impl.status, code: code}
}

// 400
func InvalidRequest(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusBadRequest,
	}
}

// 500
func DatabaseError(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusInternalServerError,
	}
}

// 404
func NotFound(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusNotFound,
	}
}

// 500
func InternalServerError(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusInternalServerError,
	}
}

func Forward(res base.Response) error {
	return &errorImpl{
		message: res.Message,
		status:  res.Code,
	}
}

// 401
func Unauthorized(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusUnauthorized,
	}
}

// 403
func Forbidden(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusForbidden,
	}
}

// 429 — the caller is being rate-limited. Distinct from 400/401 so a client can
// tell "slow down" from "wrong input" and from "not signed in".
func TooManyRequests(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusTooManyRequests,
	}
}

// 503 — a dependency the endpoint needs is not configured or not reachable, and
// the caller should retry later. Distinct from 500 so a deploy that is simply
// missing an optional integration's secrets answers "not available here" rather
// than claiming the server broke.
func ServiceUnavailable(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusServiceUnavailable,
	}
}

func (e *errorImpl) Error() string {
	return e.message
}

func (e *errorImpl) Status() int {
	return e.status
}

// Code returns the stable error name, or "" when none was attached.
func (e *errorImpl) Code() string {
	return e.code
}
