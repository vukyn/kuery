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

func InvalidRequest(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusBadRequest,
	}
}

func DatabaseError(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusInternalServerError,
	}
}

func NotFound(message string) error {
	return &errorImpl{
		message: message,
		status:  http.StatusNotFound,
	}
}

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

func (e *errorImpl) Error() string {
	return e.message
}

func (e *errorImpl) Status() int {
	return e.status
}
