package http

import (
	"testing"

	pkgErr "github.com/vukyn/kuery/http/errors"

	"github.com/gofiber/fiber/v2"
)

type details struct {
	Reason string `json:"reason"`
}

func TestErrWritesDetailsIntoData(t *testing.T) {
	cases := map[string]error{
		"code then details": pkgErr.WithCode(pkgErr.WithDetails(pkgErr.Conflict("taken"), details{Reason: "dup"}), "NAME_TAKEN"),
		"details then code": pkgErr.WithDetails(pkgErr.WithCode(pkgErr.Conflict("taken"), "NAME_TAKEN"), details{Reason: "dup"}),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			status, body := respond(t, err)
			if status != fiber.StatusConflict || body["error_code"] != "NAME_TAKEN" || body["message"] != "taken" {
				t.Fatalf("status=%d body=%v", status, body)
			}
			data, _ := body["data"].(map[string]any)
			if data["reason"] != "dup" {
				t.Errorf("data = %v", body["data"])
			}
		})
	}
}

func TestAPlainErrorAddsNoDataKey(t *testing.T) {
	_, body := respond(t, pkgErr.Conflict("taken"))
	if _, present := body["data"]; present {
		t.Errorf("data must be absent: %v", body)
	}
}

func TestA5xxNeverCarriesDetails(t *testing.T) {
	_, body := respond(t, pkgErr.WithDetails(pkgErr.DatabaseError("pq: boom"), details{Reason: "secret"}))
	if _, present := body["data"]; present {
		t.Errorf("a 5xx must not leak details: %v", body)
	}
}
