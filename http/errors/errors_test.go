package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestWithCodeKeepsStatusAndMessage(t *testing.T) {
	coded := WithCode(NotFound("place not found"), "PLACE_NOT_FOUND")

	asError, ok := coded.(Error)
	if !ok {
		t.Fatal("WithCode must return something that still satisfies Error")
	}
	if asError.Status() != http.StatusNotFound {
		t.Errorf("status = %d, want 404", asError.Status())
	}
	if asError.Error() != "place not found" {
		t.Errorf("message = %q, want it unchanged", asError.Error())
	}

	withCode, ok := coded.(Coded)
	if !ok {
		t.Fatal("WithCode must return something that satisfies Coded")
	}
	if withCode.Code() != "PLACE_NOT_FOUND" {
		t.Errorf("code = %q", withCode.Code())
	}
}

// An error built without WithCode must still satisfy Coded and answer "" —
// that is what lets a response writer ask unconditionally, and what keeps the
// envelope byte-identical for every service that has not adopted codes.
func TestUncodedErrorReportsAnEmptyCode(t *testing.T) {
	plain := InvalidRequest("email is not valid")

	withCode, ok := plain.(Coded)
	if !ok {
		t.Fatal("every error from this package must satisfy Coded")
	}
	if withCode.Code() != "" {
		t.Errorf("code = %q, want empty", withCode.Code())
	}
}

// ⚠️ Coded is a SEPARATE interface from Error on purpose. If it were a third
// method on Error, every service implementing Error itself would stop
// compiling. This is the test that fails if someone merges them.
func TestErrorInterfaceStaysTwoMethods(t *testing.T) {
	var custom Error = stubError{}
	if custom.Status() != http.StatusTeapot {
		t.Fatal("a type with only Error() and Status() must satisfy Error")
	}
}

type stubError struct{}

func (stubError) Error() string { return "brewing" }
func (stubError) Status() int   { return http.StatusTeapot }

func TestWithCodeLeavesAForeignErrorAlone(t *testing.T) {
	foreign := errors.New("not ours")
	if WithCode(foreign, "SOMETHING") != foreign {
		t.Fatal("WithCode must return a non-package error untouched")
	}
}

func TestWithCodeIgnoresAnEmptyCode(t *testing.T) {
	original := NotFound("gone")
	if WithCode(original, "") != original {
		t.Fatal("an empty code must not allocate a new error")
	}
}
