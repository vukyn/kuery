package http

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	pkgErr "github.com/vukyn/kuery/http/errors"

	"github.com/gofiber/fiber/v3"
)

func respond(t *testing.T, err error) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Get("/boom", func(c fiber.Ctx) error { return Err(c, err) })

	res, testErr := app.Test(httptest.NewRequest("GET", "/boom", nil))
	if testErr != nil {
		t.Fatalf("request: %v", testErr)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	body := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	return res.StatusCode, body
}

func TestErrCarriesTheStableCode(t *testing.T) {
	status, body := respond(t, pkgErr.WithCode(pkgErr.NotFound("place not found"), "PLACE_NOT_FOUND"))

	if status != fiber.StatusNotFound {
		t.Fatalf("status = %d", status)
	}
	if body["error_code"] != "PLACE_NOT_FOUND" {
		t.Errorf("error_code = %v", body["error_code"])
	}
	if body["message"] != "place not found" {
		t.Errorf("message = %v, want it unchanged alongside the code", body["message"])
	}
}

// ⚠️ omitempty is what makes this addition safe for every service already on
// the envelope: an error with no code produces byte-identical JSON to before.
func TestAnUncodedErrorAddsNoField(t *testing.T) {
	_, body := respond(t, pkgErr.InvalidRequest("email is not valid"))

	if _, present := body["error_code"]; present {
		t.Errorf("error_code must be absent, not empty: %v", body)
	}
}

// ⚠️ A code on a 5xx would name the subsystem that failed — precisely the
// internal detail the generic 5xx body exists to withhold. The message is
// already replaced; the code must not reintroduce the leak.
func TestA5xxLeaksNeitherMessageNorCode(t *testing.T) {
	_, body := respond(t, pkgErr.WithCode(pkgErr.DatabaseError(`pq: relation "moments" does not exist`), "DATABASE_ERROR"))

	if body["message"] != "internal server error" {
		t.Errorf("message = %v, want the generic body", body["message"])
	}
	if _, present := body["error_code"]; present {
		t.Errorf("a 5xx must carry no error_code, got %v", body["error_code"])
	}
}
