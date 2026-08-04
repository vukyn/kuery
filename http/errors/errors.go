package errors

import (
	"net/http"

	"github.com/vukyn/kuery/http/base"
)

type Error interface {
	Error() string
	Status() int
}

type errorImpl struct {
	message string
	status  int
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

func (e *errorImpl) Error() string {
	return e.message
}

func (e *errorImpl) Status() int {
	return e.status
}
